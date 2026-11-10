package app

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// sampleStations include MP3 and AAC stations from different regions.
func sampleStations() []radio.Station {
	return []radio.Station{
		{ID: "soma", Name: "SomaFM Groove Salad", Stream: "http://127.0.0.1:9/a", Country: "United States",
			CountryCode: "US", Language: "english", Tags: "ambient,chillout", Codec: "MP3", Bitrate: 256},
		{ID: "classic", Name: "Classic FM", Stream: "http://127.0.0.1:9/c", Country: "United Kingdom",
			CountryCode: "GB", Language: "english", Tags: "classical", Codec: "MP3", Bitrate: 128},
		{ID: "rthk", Name: "RTHK Radio 3", Stream: "http://127.0.0.1:9/b", Country: "Hong Kong",
			CountryCode: "HK", Language: "english", Tags: "news,talk", Codec: "AAC", Bitrate: 96},
		{ID: "cnr", Name: "中国之声", Stream: "http://127.0.0.1:9/d", Country: "China",
			CountryCode: "CN", Language: "chinese", Tags: "news", Codec: "MP3", Bitrate: 64},
	}
}

// A click on a station plays it, and the list it came from with it: the
// next station is the next of the list.
func TestPlayStation(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = sampleStations()
	a.playStation(0)
	cur := a.pl.Current()
	if cur == nil || cur.Name != "SomaFM Groove Salad" {
		t.Fatalf("playing %v", cur)
	}
	if q := a.pl.Queue(); len(q) != len(a.radio.stations) {
		t.Errorf("queue of %d, want the %d stations", len(q), len(a.radio.stations))
	}
	a.pl.Next()
	if cur := a.pl.Current(); cur == nil || cur.Name != "Classic FM" {
		t.Errorf("next played %v", cur)
	}
}

// A station plays live: the bar says so, and has no progress to drag.
func TestStationBarIsLive(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = sampleStations()
	a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(a.radio.stations[0])}, At: 0})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if !tt.HasText("LIVE") {
		t.Fatalf("bar: %q", tt.Texts())
	}
	if err := tt.Click("播放进度"); err == nil {
		t.Error("a station showed a progress bar")
	}
}

// The favorite action stays in place as playback changes state.
func TestStationFavoritePositionAcrossPlaybackStates(t *testing.T) {
	a := newTestApp()
	a.radio.stations = sampleStations()[:1]
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	before, ok := tt.Find("收藏")
	if !ok {
		t.Fatal("favorite button not found")
	}
	a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(a.radio.stations[0])}, At: 0})
	tt.Frame()
	paused, _ := tt.Find("收藏")
	a.pl.TogglePlay()
	tt.Frame()
	playing, _ := tt.Find("收藏")
	if before != paused || paused != playing {
		t.Fatalf("favorite moved: idle=%+v paused=%+v playing=%+v", before, paused, playing)
	}
}

// Usable network addresses sort first without changing directory order.
func TestSortStationsPlayableFirst(t *testing.T) {
	list := []radio.Station{
		{ID: "a", Codec: "AAC", Stream: "https://example.com/a"}, {ID: "b", Codec: "MP3", Bitrate: 128, Stream: "https://example.com/b"},
		{ID: "c", Stream: "file:///local.mp3"}, {ID: "d", Codec: "AAC+", Stream: "https://example.com/d"}, {ID: "e", Codec: "OGG", Stream: "https://example.com/e"},
	}
	sortStations(list)
	var got []string
	for _, st := range list {
		got = append(got, st.ID)
	}
	if strings.Join(got, "") != "abdec" {
		t.Errorf("stations %v", got)
	}
}

// A station whose url is an HLS playlist plays, whatever codec the directory
// leaves it with: many Chinese stations are listed as UNKNOWN and point at a
// .m3u8, which the player schedules and demuxes into audio.
func TestStationPlayableHLS(t *testing.T) {
	for _, tc := range []struct {
		codec, stream string
		want          bool
	}{
		{"UNKNOWN", "https://piccpndali.v.myalicdn.com/audio/cctv13_2.m3u8", true},
		{"", "http://ls.qingting.fm/live/3412131.m3u8?bitrate=64", true},
		{"HLS", "http://example.com/live.m3u8", true},
		{"AAC+", "http://example.com/live.m3u8", true},
		{"MP3", "http://example.com/live.mp3", true},
		{"UNKNOWN", "http://example.com/live", true},
	} {
		if got := stationPlayable(radio.Station{Codec: tc.codec, Stream: tc.stream}); got != tc.want {
			t.Errorf("stationPlayable(%q, %q) = %v, want %v", tc.codec, tc.stream, got, tc.want)
		}
	}
}

// Directory metadata is not a codec allowlist; mpv probes the real stream.
func TestStationPlayableDoesNotRejectCodec(t *testing.T) {
	for _, codec := range []string{"AAC+", "FLAC", "OPUS", "UNKNOWN"} {
		if !stationPlayable(radio.Station{Codec: codec, Stream: "https://example.com/live"}) {
			t.Errorf("codec %q was disabled", codec)
		}
	}
}

// Remote directory entries cannot open a local file through their cards.
func TestLocalStationCardIsDisabled(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = []radio.Station{{ID: "local", Name: "Local", Stream: "file:///private/audio.mp3", Curated: true}}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	_ = tt.Click("Local")
	if a.pl.Current() != nil {
		t.Fatal("a directory entry opened a local file")
	}
}

// What is typed in the field searching the stations is what the page looks
// for, a space among it: the key that plays does not take it.
func TestStationSearchTypesSpace(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Play([]player.Track{stationTrack(sampleStations()[0])}, 0)
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("搜索电台"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	tt.Type("jazz fm")
	if a.radio.query != "jazz fm" {
		t.Errorf("typed %q", a.radio.query)
	}
	if !a.pl.Playing() {
		t.Error("a space paused what played")
	}
}

// With NM_FRAMES set, saves the stations page, idle and playing one, in
// the light and the dark.
func TestStationFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	stations := sampleStations()
	for _, dark := range []bool{false, true} {
		for _, playing := range []bool{false, true} {
			a := newTestApp()
			a.settings = defaultSettings()
			a.radio.stations = stations
			if playing {
				a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(stations[0])}, At: 0})
			}
			tt := ui.NewTester(a.view, 1200, 900)
			tt.SetDark(dark)
			tt.Advance(time.Second)
			tt.Frame()
			name := "stations.png"
			switch {
			case dark && playing:
				name = "stations-playing-dark.png"
			case dark:
				name = "stations-dark.png"
			case playing:
				name = "stations-playing.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			png.Encode(f, tt.Image())
			f.Close()
		}
	}
}

// With NM_FRAMES set, saves the page of the favorites, a station kept, and
// the page of the recent, empty.
func TestLibraryFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, c := range []struct {
		name string
		page pageRef
	}{{"favorites.png", pageFavorites}, {"recent.png", pageRecent}} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.lib = &library{}
		a.lib.toggle(sampleStations()[0])
		a.page = c.page
		tt := ui.NewTester(a.view, 1200, 900)
		tt.Advance(time.Second)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, c.name))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, tt.Image())
		f.Close()
	}
}

// With NM_FRAMES set, saves the two-column stations page at the window's
// minimum width, with names truncated rather than cards clipped.
func TestNarrowFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = sampleStations()
	tt := ui.NewTester(a.view, 880, 700)
	tt.Advance(time.Second)
	tt.Frame()
	f, err := os.Create(filepath.Join(dir, "stations-narrow.png"))
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, tt.Image())
	f.Close()
}

// Idle, the bar says nothing of live: nothing plays.
func TestIdleBarHasNoLive(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if tt.HasText("LIVE") {
		t.Errorf("idle bar says live: %q", tt.Texts())
	}
}

// A station that has no logo shows its initial in the badge: the first
// letter or digit of its name, which a Chinese name shows as its first
// character, and the punctuation before it is skipped.
func TestStationInitial(t *testing.T) {
	cases := []struct{ name, want string }{
		{"怀集音乐之声", "怀"},
		{"香港电台第一台 RTHK", "香"},
		{"bbc radio 4", "B"},
		{"  1.FM - Chill", "1"},
		{"· ·", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := stationInitial(c.name); got != c.want {
			t.Errorf("stationInitial(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// What a card says of a station's stream: the directory writes "UNKNOWN"
// where it has no codec, and the format is only settled by connecting, so
// the placeholder is left out rather than shown as a format.
func TestStationMetaCodec(t *testing.T) {
	cases := []struct {
		codec   string
		bitrate int
		want    string
	}{
		{"MP3", 128, "MP3 128 kbps"},
		{"MP3", 0, "MP3"},
		{"UNKNOWN", 0, ""},
		{"unknown", 128, "128 kbps"},
		{"", 0, ""},
	}
	for _, c := range cases {
		got := stationMeta(radio.Station{Codec: c.codec, Bitrate: c.bitrate})
		if got != c.want {
			t.Errorf("stationMeta(%q, %d) = %q, want %q", c.codec, c.bitrate, got, c.want)
		}
	}
}

// A station keeps the color of its badge over the frames and the sessions:
// the hue comes from its name, not from the order it shows in.
func TestStationHueHolds(t *testing.T) {
	name := "Classic FM"
	hue := stationHue(name)
	if stationHue(name) != hue {
		t.Error("a station changed its hue between calls")
	}
	if stationHue("Classic FM ") == hue {
		t.Error("two names share a hue by accident of the space")
	}
}

// The stations kept come before the ones kept before them, and keeping one
// again lets it go.
func TestFavoriteKeepsStation(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.lib = &library{}
	st := sampleStations()
	if !a.lib.toggle(st[0]) {
		t.Fatal("the first station was not kept")
	}
	a.lib.toggle(st[1])
	if got := len(a.lib.Favorites); got != 2 {
		t.Fatalf("%d stations kept, want 2", got)
	}
	if a.lib.Favorites[0].ID != st[1].ID {
		t.Errorf("favorites start with %q, want the last kept", a.lib.Favorites[0].ID)
	}
	if a.lib.toggle(st[1]) {
		t.Error("a station kept again was kept")
	}
	if a.lib.has(st[1].ID) || len(a.lib.Favorites) != 1 {
		t.Errorf("letting go left %v", a.lib.Favorites)
	}
}

// The page of the favorites lists what was kept, and the one of the recent
// what played.
func TestPagesListLibrary(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.lib = &library{}
	st := sampleStations()
	a.lib.toggle(st[2])
	if got := a.pageStations(); len(got) != 0 {
		t.Errorf("the stations page listed %d stations before asking", len(got))
	}
	a.page = pageFavorites
	if got := a.pageStations(); len(got) != 1 || got[0].ID != st[2].ID {
		t.Errorf("favorites listed %v", got)
	}
	a.page = pageRecent
	if got := a.pageStations(); len(got) != 0 {
		t.Errorf("recent listed %v", got)
	}
}

// Playing a station notes it once, at the head of the recent, whatever page
// it was played from.
func TestPlayedKeepsRecentOnce(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.lib = &library{}
	a.radio.stations = sampleStations()
	a.playStation(0)
	a.playStation(1)
	a.playStation(0)
	if got := len(a.lib.Recent); got != 2 {
		t.Fatalf("%d stations recent, want 2", got)
	}
	if a.lib.Recent[0].ID != "soma" {
		t.Errorf("recent starts with %q, want the one played last", a.lib.Recent[0].ID)
	}
}

// The dice plays one of the page's stations that the app decodes, and not
// the one already playing.
func TestPlayRandomPlaysPlayable(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = []radio.Station{
		{ID: "aac", Name: "HE-AAC FM", Codec: "AAC+", Stream: "http://127.0.0.1:9/a", Curated: true},
		{ID: "one", Name: "One", Codec: "MP3", Stream: "http://127.0.0.1:9/b", Curated: true},
		{ID: "two", Name: "Two", Codec: "MP3", Stream: "http://127.0.0.1:9/c", Curated: true},
	}
	seen := map[string]bool{}
	for range 30 {
		a.playRandom()
		cur := a.pl.Current()
		if cur == nil {
			t.Fatal("the dice played nothing")
		}
		seen[cur.Name] = true
	}
	if len(seen) < 2 {
		t.Errorf("the dice played %v, always the same", seen)
	}
}

// A page with nothing the app plays leaves the dice alone rather than
// playing a station that would only fail.
func TestPlayRandomNothingToPlay(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.radio.stations = []radio.Station{{ID: "aac", Name: "HE-AAC FM", Codec: "AAC+", Curated: true}}
	a.playRandom()
	if cur := a.pl.Current(); cur != nil {
		t.Errorf("the dice played %v", cur)
	}
}
