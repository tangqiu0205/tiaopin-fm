package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

// Favoriting a horizontal card must not trigger its enclosing play button,
// and the saved station remains reachable from the library at either width.
func TestStationCardFavoriteNavigation(t *testing.T) {
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.radio.stations = sampleStations()[:1]
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			if err := tt.Click("收藏"); err != nil {
				t.Fatal(err)
			}
			if !a.lib.has("soma") {
				t.Fatal("the heart did not save the station")
			}
			if a.pl.Current() != nil {
				t.Fatal("the heart also played its enclosing card")
			}
			if err := tt.Click("我的收藏"); err != nil {
				t.Fatal(err)
			}
			if a.page != pageFavorites {
				t.Fatal("the library did not open")
			}
			if err := tt.Click("收藏"); err != nil {
				t.Fatal(err)
			}
			if len(a.lib.Favorites) != 0 || a.pl.Current() != nil {
				t.Fatal("removing the favorite played it or left it in the library")
			}
			tt.Frame()
		})
	}
}

// A restored paused station starts connecting with the intent to play on the first click.
func TestRestoredStationPlaysOnFirstClick(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	track := stationTrack(sampleStations()[0])
	track.Stream = "http://127.0.0.1:1/stream"
	a.pl.Restore(player.Session{Stations: []player.Track{track}, At: 0})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("播放/暂停"); err != nil {
		t.Fatal(err)
	}
	// A restored player used to connect while still marked paused, so the
	// listener had to press play a second time after the connection opened.
	if !a.pl.Playing() || !a.pl.Loading() {
		t.Fatalf("first click: playing=%v loading=%v", a.pl.Playing(), a.pl.Loading())
	}
	if err := tt.Click("播放/暂停"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Playing() || !a.pl.Loading() {
		t.Fatalf("second click: playing=%v loading=%v", a.pl.Playing(), a.pl.Loading())
	}
}

// The full-width player keeps next, previous and play/pause usable in both
// the two-column and three-column layouts.
func TestPlayerBarControlsAtBothWidths(t *testing.T) {
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			stations := sampleStations()[:2]
			a.radio.stations = stations
			a.pl.Restore(player.Session{Stations: []player.Track{
				stationTrack(stations[0]), stationTrack(stations[1]),
			}, At: 0})
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			if err := tt.Click("下一个电台"); err != nil {
				t.Fatal(err)
			}
			if a.pl.Current().Name != stations[1].Name {
				t.Fatal("next did not change the station")
			}
			if err := tt.Click("上一个电台"); err != nil {
				t.Fatal(err)
			}
			if a.pl.Current().Name != stations[0].Name {
				t.Fatal("previous did not restore the station")
			}
			before := a.pl.Playing()
			if err := tt.Click("播放/暂停"); err != nil {
				t.Fatal(err)
			}
			if a.pl.Playing() == before {
				t.Fatal("play/pause did not toggle playback")
			}
		})
	}
}
