package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// Cached lists must expire at the TTL boundary and must not share mutable
// slices with either the network result or the currently displayed list.
func TestStationCacheExpiryAndOwnership(t *testing.T) {
	var cache stationListCache
	now := time.Unix(1000, 0)
	stations := []radio.Station{{ID: "a", Name: "Original"}}
	cache.put("music", radio.Page{Stations: stations}, now)
	stations[0].Name = "Changed input"
	got, ok, _ := cache.lookup("music", now.Add(stationCacheTTL-time.Nanosecond))
	if !ok || got.Stations[0].Name != "Original" {
		t.Fatalf("cached list before expiry: %v, %v", got, ok)
	}
	got.Stations[0].Name = "Changed view"
	if got, _, _ := cache.lookup("music", now); got.Stations[0].Name != "Original" {
		t.Fatal("the view changed the cached list")
	}
	if _, ok, _ := cache.lookup("music", now.Add(stationCacheTTL)); ok {
		t.Fatal("an expired list was reused")
	}
	cache.put("empty", radio.Page{Stations: nil}, now)
	if got, ok, _ := cache.lookup("empty", now); !ok || len(got.Stations) != 0 {
		t.Fatal("a successful empty result was not cached")
	}
}

// Searching many different names must evict the least recently used list,
// while a recently revisited category remains cached.
func TestStationCacheEvictsLeastRecentlyUsed(t *testing.T) {
	var cache stationListCache
	now := time.Unix(1000, 0)
	for i := range stationCacheLimit {
		cache.put(fmt.Sprint(i), radio.Page{Stations: nil}, now)
	}
	cache.lookup("0", now)
	cache.put("new", radio.Page{Stations: nil}, now)
	if len(cache.entries) != stationCacheLimit {
		t.Fatalf("cache grew to %d entries", len(cache.entries))
	}
	if _, ok, _ := cache.lookup("0", now); !ok {
		t.Fatal("the recently used entry was evicted")
	}
	if _, ok, _ := cache.lookup("1", now); ok {
		t.Fatal("the least recently used entry was retained")
	}
}

// Obsolete responses must not update the visible list or cache; failed
// responses must not cache or show partial results.
func TestStationCacheReceivesOnlySuccessfulResults(t *testing.T) {
	a := newTestApp()
	now := time.Unix(1000, 0)
	a.radio.asked = "news"
	a.radio.loading = true
	a.radio.stations = []radio.Station{{ID: "news"}}
	a.receiveStations("music", a.radio.generation, 0, radio.Page{Stations: []radio.Station{{ID: "music"}}}, nil, now)
	if a.radio.stations[0].ID != "news" || !a.radio.loading {
		t.Fatal("a late result replaced the active request")
	}
	if _, ok, _ := a.radio.cache.lookup("music", now); ok {
		t.Fatal("an obsolete response populated the cache")
	}
	for _, stations := range [][]radio.Station{nil, {{ID: "offline"}}} {
		a.receiveStations("news", a.radio.generation, 0, radio.Page{Stations: stations}, errors.New("directory unavailable"), now)
		if _, ok, _ := a.radio.cache.lookup("news", now); ok {
			t.Fatal("an error poisoned the directory cache")
		}
		if len(a.radio.stations) != 1 || a.radio.stations[0].ID != "news" || a.radio.err == "" {
			t.Fatal("a failed response replaced the visible list or hid the error")
		}
	}
}

// Switching back to a cached category must show its own list immediately,
// with no loading state or new network request at either supported layout.
func TestStationCachedCategorySwitch(t *testing.T) {
	old := findStations
	findStations = func(context.Context, radio.Filter, int, int) (radio.Page, error) {
		return radio.Page{}, errors.New("unexpected cache miss")
	}
	defer func() { findStations = old }()
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			for tag, id := range []string{"all", "music", "news"} {
				a.radio.tag = tag
				a.radio.cache.put(a.stationKey(), radio.Page{Stations: []radio.Station{{ID: id, Name: id, Codec: "MP3"}}}, time.Now())
			}
			a.radio.tag = 0
			a.loadStations()
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			for _, choice := range []struct{ label, id string }{
				{"音乐", "music"}, {"新闻", "news"}, {"音乐", "music"}, {"全部内容", "all"},
			} {
				if err := tt.Click(choice.label); err != nil {
					t.Fatal(err)
				}
				if a.radio.loading || len(a.radio.stations) != 1 || a.radio.stations[0].ID != choice.id {
					t.Fatalf("cached category %q requested the network or displayed another list", choice.label)
				}
			}
		})
	}
}

// An expired cache entry must trigger a refresh even when the same filter
// was already displayed; the old asked-key guard must not bypass expiry.
func TestStationCacheExpiryRefreshesCurrentFilter(t *testing.T) {
	requests := make(chan radio.Filter, 1)
	old := findStations
	findStations = func(_ context.Context, f radio.Filter, _, _ int) (radio.Page, error) {
		requests <- f
		return radio.Page{}, nil
	}
	defer func() { findStations = old }()
	a := newTestApp()
	a.radio.tag = 1
	a.radio.asked = a.stationKey()
	a.radio.cache.put(a.stationKey(), radio.Page{Stations: sampleStations()}, time.Unix(0, 0))
	a.loadStations()
	if !a.radio.loading || !a.radio.stale || len(a.radio.stations) != len(sampleStations()) {
		t.Fatal("an expired list was not shown while refreshing in the background")
	}
	select {
	case f := <-requests:
		if f.Tag != "music" {
			t.Fatalf("refreshed the wrong filter: %+v", f)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no request for the expired list")
	}
}

// Every active facet must contribute to the cache key, so music from one
// region, language or name search cannot leak into another result list.
func TestStationCacheKeysIncludeEveryFacet(t *testing.T) {
	a := newTestApp()
	seen := make(map[string]bool)
	for _, state := range []radioState{
		{}, {tag: 1}, {country: "美国"}, {language: "英语"}, {name: "jazz fm"},
	} {
		a.radio = state
		key := a.stationKey()
		if seen[key] {
			t.Fatalf("different filters share a cache key: %+v", state)
		}
		seen[key] = true
	}
}
