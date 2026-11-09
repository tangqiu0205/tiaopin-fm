package app

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

func TestSidebarRandomPlayButton(t *testing.T) {
	for _, width := range []int{880, 1200} {
		for _, page := range []pageRef{pageRadio, pageFavorites, pageRecent} {
			t.Run(fmt.Sprintf("%d/%d", width, page), func(t *testing.T) {
				a := newTestApp()
				a.page = page
				station := radio.Station{ID: "random", Name: "Random FM", Stream: "http://127.0.0.1:9/live", Curated: true}
				a.radio.stations = []radio.Station{station}
				a.lib.Favorites, a.lib.Recent = []radio.Station{station}, []radio.Station{station}
				tt := ui.NewTester(a.view, width, 580)
				tt.Frame()
				button, ok := tt.Find("随机播放")
				if !ok || button.X+button.W > sidebarWidth || button.Y+button.H > 580-playerBarHeight {
					t.Fatalf("random action outside sidebar or overlapping player: %+v", button)
				}
				if math.Abs(float64(button.X+button.W/2-sidebarWidth/2)) > 0.5 {
					t.Fatalf("random action is not horizontally centered: %+v", button)
				}
				last, ok := tt.Find("识曲历史")
				if !ok || last.Y+last.H > button.Y {
					t.Fatalf("random action overlaps navigation: button=%+v last=%+v", button, last)
				}
				if tt.HasText("随便听听") {
					t.Fatal("the duplicate header action remains")
				}
				if err := tt.Click("随机播放"); err != nil {
					t.Fatal(err)
				}
				if a.page != page || a.pl.Current() == nil || a.pl.Current().Name != station.Name {
					t.Fatal("random action navigated instead of starting playback")
				}
			})
		}
	}
}

func TestSidebarRandomPlayDisabledWithoutPlayableStations(t *testing.T) {
	a := newTestApp()
	a.radio.stations = []radio.Station{{ID: "invalid", Stream: "file:///private/audio.mp3"}}
	tt := ui.NewTester(a.view, 880, 580)
	tt.Frame()
	access, ok := tt.Access("随机播放")
	if !ok || !access.Disabled {
		t.Fatalf("random action should be disabled: %+v", access)
	}
}
