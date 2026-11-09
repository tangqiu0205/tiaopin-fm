package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// The stations page, with nothing playing, asks for no frames of its own:
// idle, it is still, and the app sleeps.
func TestStationsIdleFrames(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = []radio.Station{
		{ID: "a", Name: "Classic Vinyl HD", Stream: "http://127.0.0.1:9/a", Country: "United States",
			CountryCode: "US", Codec: "MP3", Bitrate: 320},
		{ID: "b", Name: "BBC World Service", Stream: "http://127.0.0.1:9/b", Country: "United Kingdom",
			CountryCode: "GB", Codec: "MP3", Bitrate: 56},
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	asked := 0
	for range 60 {
		tt.Advance(time.Second / 60)
		if tt.FramePending() {
			asked++
			tt.Frame()
		}
	}
	if asked > 3 {
		t.Errorf("the stations page idle: %d frames a second", asked)
	}
}

// A running sleep countdown updates on a timer rather than requesting continuous frames.
func TestSleepTimerFrameBudget(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1200, 800)
	a.sleepDeadline = tt.Now().Add(15 * time.Minute)
	tt.Frame()
	tt.Advance(time.Second)
	tt.Frame()
	asked := 0
	for range 60 {
		tt.Advance(time.Second / 60)
		if tt.FramePending() {
			asked++
			tt.Frame()
		}
	}
	if asked > 3 {
		t.Errorf("the sleep countdown requested %d continuous frames in a second", asked)
	}
}
