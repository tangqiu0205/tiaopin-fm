package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// Codec metadata no longer restricts what mpv may probe.
func TestStationPlayableAACProfiles(t *testing.T) {
	for _, tc := range []struct {
		codec string
		want  bool
	}{
		{"AAC", true}, {"aac-lc", true}, {" MP3 ", true},
		{"AAC+", true}, {"HE-AAC", true}, {"HE-AACv2", true},
		{"HLS", true}, {"AAC HLS", true}, {"MPEG-4 HE-AAC", true}, {"OGG", true}, {"VORBIS", true}, {"OPUS", true}, {"Ogg Vorbis", true}, {"Ogg Opus", true}, {"Ogg FLAC", true},
	} {
		if got := stationPlayable(radio.Station{Codec: tc.codec, Stream: "https://example.com/live"}); got != tc.want {
			t.Errorf("stationPlayable(%q) = %v, want %v", tc.codec, got, tc.want)
		}
	}
}

// Both AAC-LC and HE-AAC can enter the queue.
func TestAACStationCardPlays(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = []radio.Station{
		{ID: "aac-lc", Name: "AAC LC FM", Codec: "AAC", Stream: "http://127.0.0.1:9/a", Curated: true},
		{ID: "he-aac", Name: "HE AAC FM", Codec: "AAC+", Stream: "http://127.0.0.1:9/b", Curated: true},
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("AAC LC FM"); err != nil {
		t.Fatal(err)
	}
	if cur := a.pl.Current(); cur == nil || cur.ID != stationID("aac-lc") {
		t.Fatal("the AAC-LC station did not enter the player queue")
	}
	if err := tt.Click("HE AAC FM"); err != nil {
		t.Fatal(err)
	}
	if a.pl.Current().ID != stationID("he-aac") {
		t.Fatal("HE-AAC station was not selected")
	}
}
