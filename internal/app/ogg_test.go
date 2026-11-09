package app

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// Clicking an Ogg/Vorbis/Opus card must start that station, rather than leave
// it disabled merely because its directory metadata does not say MP3/AAC.
func TestOggStationCardPlays(t *testing.T) {
	for _, codec := range []string{"OGG", "VORBIS", "OPUS"} {
		t.Run(codec, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.radio.stations = []radio.Station{
				{ID: codec, Name: "Ogg Test FM", Codec: codec, Stream: "http://127.0.0.1:9/ogg", Curated: true},
			}
			tt := ui.NewTester(a.view, 1200, 800)
			tt.Frame()
			if err := tt.Click("Ogg Test FM"); err != nil {
				t.Fatal(err)
			}
			if cur := a.pl.Current(); cur == nil || cur.ID != stationID(codec) {
				t.Fatal("Ogg station did not enter the player queue")
			}
		})
	}
}
