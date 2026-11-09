package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// Cached lists must survive a disk round trip without renewing their TTL;
// both fresh and expired entries remain usable after restarting the app.
func TestStationCachePersistsFreshAndStaleLists(t *testing.T) {
	var cache stationListCache
	now := time.Unix(1000000, 0)
	cache.put("old", radio.Page{Stations: []radio.Station{{ID: "old", Stream: "http://old"}}}, now.Add(-stationCacheTTL))
	cache.put("fresh", radio.Page{Stations: []radio.Station{{ID: "fresh", Stream: "http://fresh"}}, NextOffset: 137, HasMore: true}, now)
	disk := &stationCacheDisk{path: filepath.Join(t.TempDir(), "stations-cache.json")}
	if err := disk.write(cache.snapshot()); err != nil {
		t.Fatal(err)
	}
	restored, err := readStationCache(disk.path)
	if err != nil {
		t.Fatal(err)
	}
	if list, fresh, found := restored.lookup("fresh", now); !found || !fresh || list.Stations[0].ID != "fresh" || list.NextOffset != 137 || !list.HasMore {
		t.Fatal("a fresh list was lost across restart")
	}
	if list, fresh, found := restored.lookup("old", now); !found || fresh || list.Stations[0].ID != "old" {
		t.Fatal("an expired list was discarded or given a new TTL")
	}
	if !restored.entries["fresh"].Expires.Equal(cache.entries["fresh"].Expires) {
		t.Fatal("reading disk renewed freshness")
	}
}

// A worker finishing late must not overwrite a newer snapshot; replacing
// an existing file must also leave a readable complete cache.
func TestStationCacheDiskRejectsOlderWrites(t *testing.T) {
	var cache stationListCache
	now := time.Unix(1000000, 0)
	cache.put("music", radio.Page{Stations: []radio.Station{{ID: "a", Stream: "old"}}}, now)
	old := cache.snapshot()
	cache.put("music", radio.Page{Stations: []radio.Station{{ID: "a", Stream: "new"}}}, now)
	latest := cache.snapshot()
	disk := &stationCacheDisk{path: filepath.Join(t.TempDir(), "cache.json")}
	for _, snapshot := range []stationCacheSnapshot{old, latest, old} {
		if err := disk.write(snapshot); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := readStationCache(disk.path)
	if err != nil {
		t.Fatal(err)
	}
	if list, _, found := restored.lookup("music", now); !found || list.Stations[0].Stream != "new" {
		t.Fatal("a delayed write replaced the refreshed URL")
	}
	parts, _ := filepath.Glob(filepath.Join(filepath.Dir(disk.path), "*.part"))
	if len(parts) != 0 {
		t.Fatal("a completed cache write left temporary files")
	}
}

// Missing cache files are an ordinary cold start, while malformed files
// must report failure instead of silently claiming valid cached data.
func TestStationCacheDiskReportsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if _, err := readStationCache(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readStationCache(path); err == nil {
		t.Fatal("a truncated cache was accepted")
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readStationCache(path); err == nil {
		t.Fatal("an unknown cache format was accepted")
	}
}

// An expired visible list must remain on screen when background updating
// fails, and a subsequent success must replace it and clear the stale state.
func TestStationStaleListSurvivesRefreshFailure(t *testing.T) {
	a := newTestApp()
	now := time.Unix(1000000, 0)
	a.radio.asked = "music"
	a.radio.loading, a.radio.stale = true, true
	a.radio.stations = []radio.Station{{ID: "old"}}
	a.radio.cache.put("music", radio.Page{Stations: a.radio.stations}, now.Add(-stationCacheTTL))
	a.receiveStations("music", a.radio.generation, 0, radio.Page{Stations: []radio.Station{{ID: "offline-fallback"}}}, errors.New("offline"), now)
	if a.radio.loading || !a.radio.stale || a.radio.err == "" || a.radio.stations[0].ID != "old" {
		t.Fatal("refresh failure discarded the cached list or concealed the error")
	}
	if _, fresh, _ := a.radio.cache.lookup("music", now); fresh {
		t.Fatal("failure made stale data fresh")
	}
	a.receiveStations("music", a.radio.generation, 0, radio.Page{Stations: []radio.Station{{ID: "new"}}}, nil, now)
	if a.radio.stale || a.radio.err != "" || a.radio.stations[0].ID != "new" {
		t.Fatal("refresh success did not replace stale data")
	}
}
