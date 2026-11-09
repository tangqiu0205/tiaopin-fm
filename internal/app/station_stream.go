package app

import (
	"io.github.tangqiu0205/tiaopin-fm/internal/player"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// stationStreamRefreshed runs after the player accepts the result for its
// current load. Repair every cached copy, without renewing list freshness.
func (a *app) stationStreamRefreshed(track player.Track) {
	changed := false
	for key, entry := range a.radio.cache.entries {
		if patchStationStreams(entry.Stations, track) {
			a.radio.cache.entries[key] = entry
			changed = true
		}
	}
	if changed {
		a.radio.cache.clock++
		a.persistStationCache()
	}
	patchStationStreams(a.radio.stations, track)
	if a.lib != nil {
		favorites := patchStationStreams(a.lib.Favorites, track)
		recent := patchStationStreams(a.lib.Recent, track)
		if favorites || recent {
			if err := a.lib.save(); err != nil {
				a.toast = "无法保存更新后的电台地址：" + err.Error()
			}
		}
	}
}

func patchStationStreams(stations []radio.Station, track player.Track) bool {
	changed := false
	for i := range stations {
		if stations[i].ID == track.StationID && stations[i].Stream != track.Stream {
			stations[i].Stream = track.Stream
			changed = true
		}
	}
	return changed
}

// Older sessions predate the persisted UUID. Recover it from the library
// when possible, so the current station can also refresh an expired URL.
func (a *app) restoreStationIDs(stations []player.Track) {
	if a.lib == nil {
		return
	}
	for i := range stations {
		if stations[i].StationID != "" {
			continue
		}
		for _, list := range [][]radio.Station{a.lib.Recent, a.lib.Favorites} {
			for _, st := range list {
				if !st.Curated && stationID(st.ID) == stations[i].ID {
					stations[i].StationID = st.ID
					break
				}
			}
			if stations[i].StationID != "" {
				break
			}
		}
	}
}
