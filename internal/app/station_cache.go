package app

import (
	"slices"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

const (
	stationCacheTTL   = 24 * time.Hour
	stationCacheLimit = 32
)

type stationCacheEntry struct {
	radio.Page
	Expires time.Time `json:"expires"`
	Used    uint64    `json:"used"`
}

// stationListCache is window-thread-only and bounded so arbitrary search
// terms cannot grow it indefinitely. Its keys include every directory facet.
type stationListCache struct {
	entries map[string]stationCacheEntry
	clock   uint64
}

// Expired entries remain available while their replacements load.
func (c *stationListCache) lookup(key string, now time.Time) (radio.Page, bool, bool) {
	entry, ok := c.entries[key]
	if !ok {
		return radio.Page{}, false, false
	}
	c.clock++
	entry.Used = c.clock
	c.entries[key] = entry
	entry.Stations = slices.Clone(entry.Stations)
	return entry.Page, now.Before(entry.Expires), true
}

func (c *stationListCache) put(key string, page radio.Page, now time.Time) {
	if c.entries == nil {
		c.entries = make(map[string]stationCacheEntry)
	}
	if _, exists := c.entries[key]; !exists && len(c.entries) >= stationCacheLimit {
		oldest := ""
		var used uint64 = ^uint64(0)
		for key, entry := range c.entries {
			if entry.Used < used {
				oldest, used = key, entry.Used
			}
		}
		delete(c.entries, oldest)
	}
	c.clock++
	page.Stations = slices.Clone(page.Stations)
	c.entries[key] = stationCacheEntry{
		Page: page, Expires: now.Add(stationCacheTTL), Used: c.clock,
	}
}

// Only the active generation may update the list or its first-page cache.
// A -> B -> A must not accept an older A response.
func (a *app) receiveStations(key string, generation uint64, offset int, page radio.Page, err error, now time.Time) {
	if a.quitting || a.radio.asked != key || a.radio.generation != generation {
		return
	}
	a.radio.loading = false
	if a.radio.cancel != nil {
		a.radio.cancel()
		a.radio.cancel = nil
	}
	if err != nil {
		if offset > 0 {
			a.radio.moreErr = "加载更多失败，请重试"
		} else if a.radio.stale {
			a.radio.err = "列表更新失败，正在显示本地缓存"
		} else {
			a.radio.err = "电台目录暂时连不上"
		}
		return
	}
	sortStations(page.Stations)
	if offset == 0 {
		a.radio.cache.put(key, page, now)
		a.persistStationCache()
		a.radio.stations = nil
	}
	a.radio.stations = appendUniqueStations(a.radio.stations, page.Stations)
	a.radio.nextOffset, a.radio.hasMore = page.NextOffset, page.HasMore
	a.radio.err, a.radio.moreErr, a.radio.stale = "", "", false
}
