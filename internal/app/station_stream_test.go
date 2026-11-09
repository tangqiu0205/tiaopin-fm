package app

import (
	"testing"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// A refreshed stream address must repair all copies of the station without
// marking the rest of an expired category's metadata as fresh.
func TestStationRefreshedStreamRepairsCachedCopies(t *testing.T) {
	a := newTestApp()
	now := time.Unix(1000000, 0)
	stations := []radio.Station{{ID: "a", Stream: "old"}, {ID: "b", Stream: "other"}}
	a.radio.cache.put("music", radio.Page{Stations: stations}, now)
	a.radio.cache.put("all", radio.Page{Stations: stations}, now)
	expires := a.radio.cache.entries["music"].Expires
	snapshot := a.radio.cache.snapshot()
	a.radio.stations = append([]radio.Station(nil), stations...)
	a.lib.Favorites = append([]radio.Station(nil), stations...)
	a.lib.Recent = append([]radio.Station(nil), stations...)
	a.stationStreamRefreshed(player.Track{StationID: "a", Stream: "new"})
	for _, list := range [][]radio.Station{
		a.radio.stations, a.lib.Favorites, a.lib.Recent,
		a.radio.cache.entries["music"].Stations, a.radio.cache.entries["all"].Stations,
	} {
		if list[0].Stream != "new" || list[1].Stream != "other" {
			t.Fatalf("refreshed URL did not propagate correctly: %v", list)
		}
	}
	if !a.radio.cache.entries["music"].Expires.Equal(expires) {
		t.Fatal("refreshing a URL renewed unrelated category data")
	}
	if snapshot.Entries["music"].Stations[0].Stream != "old" {
		t.Fatal("a background disk snapshot shared mutable cache storage")
	}
}

// Older playback sessions lacked the directory UUID; library entries can
// recover it, while built-in station IDs must not trigger directory lookups.
func TestRestoreStationIDsForOlderSessions(t *testing.T) {
	a := newTestApp()
	a.lib.Recent = []radio.Station{{ID: "uuid-a"}, {ID: "curated:b", Curated: true}}
	tracks := []player.Track{{ID: stationID("uuid-a")}, {ID: stationID("curated:b")}}
	a.restoreStationIDs(tracks)
	if tracks[0].StationID != "uuid-a" || tracks[1].StationID != "" {
		t.Fatalf("restored identities: %+v", tracks)
	}
}
