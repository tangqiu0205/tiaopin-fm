package app

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/egoist/mygo"
)

// stationCacheVersion is the shape of the file, and of the lists in it: a
// version bump drops the cached lists, as when stations an earlier version
// left out of them became playable.
const stationCacheVersion = 3

type stationCacheSnapshot struct {
	Version int                          `json:"version"`
	Clock   uint64                       `json:"clock"`
	Entries map[string]stationCacheEntry `json:"entries"`
}

// Workers serialize writes and reject older snapshots, so a delayed write
// cannot overwrite a newer directory result or stream address.
type stationCacheDisk struct {
	path    string
	mu      sync.Mutex
	written uint64
}

func (c *stationListCache) snapshot() stationCacheSnapshot {
	s := stationCacheSnapshot{Version: stationCacheVersion, Clock: c.clock, Entries: make(map[string]stationCacheEntry, len(c.entries))}
	for key, entry := range c.entries {
		entry.Stations = slices.Clone(entry.Stations)
		s.Entries[key] = entry
	}
	return s
}

func readStationCache(path string) (stationListCache, error) {
	var cache stationListCache
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return cache, nil
	}
	if err != nil {
		return cache, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 8<<20))
	if err != nil {
		return cache, err
	}
	var s stationCacheSnapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return cache, err
	}
	if s.Version != stationCacheVersion || len(s.Entries) > stationCacheLimit {
		return cache, errors.New("unsupported or oversized station cache")
	}
	for _, entry := range s.Entries {
		if entry.Expires.IsZero() || len(entry.Stations) > stationLimit || entry.NextOffset < 0 || (entry.HasMore && entry.NextOffset == 0) {
			return cache, errors.New("invalid station cache entry")
		}
		if entry.Used > s.Clock {
			s.Clock = entry.Used
		}
	}
	cache.entries, cache.clock = s.Entries, s.Clock
	return cache, nil
}

func (d *stationCacheDisk) write(s stationCacheSnapshot) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s.Clock <= d.written {
		return nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(d.path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(d.path), "stations-cache-*.part")
	if err != nil {
		return err
	}
	part := f.Name()
	defer os.Remove(part)
	_, writeErr := f.Write(b)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(part, d.path); err != nil {
		return err
	}
	d.written = s.Clock
	return nil
}

// Loading happens before the first directory request, but not on the UI
// thread. Missing cache files are normal; unreadable files are reported.
func (a *app) loadStationCache() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		a.toast = "无法打开电台缓存：" + err.Error()
		a.loadStations()
		return
	}
	disk := &stationCacheDisk{path: filepath.Join(dir, "stations-cache.json")}
	a.radio.loading, a.radio.cacheLoading = true, true
	go func() {
		cache, err := readStationCache(disk.path)
		a.post(func() {
			if err != nil {
				log.Printf("read station cache: %v", err)
				a.toast = "本地电台缓存无法读取，将重新获取"
			} else {
				a.radio.cache = cache
			}
			a.stationDisk = disk
			a.radio.loading, a.radio.cacheLoading = false, false
			// Read the current controls, not the ones active before disk I/O.
			a.loadStations()
		})
	}()
}

func (a *app) persistStationCache() {
	if a.stationDisk == nil {
		return
	}
	disk, snapshot := a.stationDisk, a.radio.cache.snapshot()
	go func() {
		if err := disk.write(snapshot); err != nil {
			log.Printf("write station cache: %v", err)
			a.post(func() { a.toast = "无法保存电台列表缓存：" + err.Error() })
		}
	}()
}

// Flush the latest snapshot before exit; older queued workers then become
// no-ops. Waiting here prevents a normal quit from losing a just-loaded list.
func (a *app) flushStationCache() {
	if a.stationDisk != nil {
		if err := a.stationDisk.write(a.radio.cache.snapshot()); err != nil {
			log.Printf("flush station cache: %v", err)
		}
	}
}
