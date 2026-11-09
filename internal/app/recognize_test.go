package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

// Clicking recognition while a restored station is paused must not start sampling.
func TestRecognizePausedStation(t *testing.T) {
	old := recognitionAfter
	defer func() { recognitionAfter = old }()
	var expire func()
	recognitionAfter = func(_ *app, _ time.Duration, fn func()) func() { expire = fn; return func() {} }
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(sampleStations()[0])}, At: 0})
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("识别歌曲（发送音频指纹至 Shazam）"); err != nil {
		t.Fatal(err)
	}
	if a.recognizing || a.recognitionText != "请先播放电台再识曲" || expire == nil {
		t.Fatalf("recognition state: active=%v, message=%q, expiry scheduled=%v", a.recognizing, a.recognitionText, expire != nil)
	}
	expire()
	if a.recognizedTrack != nil {
		t.Fatal("the paused message did not expire")
	}
}

// Copying a recognized song writes its artist and title without changing stations.
func TestCopyRecognizedSong(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(sampleStations()[0])}, At: 0})
	a.recognizedTrack = a.pl.Current()
	a.recognitionSong = player.Song{Title: "Song", Artist: "Artist"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("复制歌曲信息"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "Artist - Song" {
		t.Fatalf("clipboard = %q", got)
	}
	if a.pl.Current() == nil || a.recognizedTrack != nil {
		t.Fatal("copying changed the station or left the result open")
	}
}

// A result expires after the scheduled delay, while an older timer cannot close a newer result.
func TestRecognitionResultExpires(t *testing.T) {
	old := recognitionAfter
	defer func() { recognitionAfter = old }()
	var pending []func()
	cancelled := 0
	recognitionAfter = func(_ *app, delay time.Duration, fn func()) func() {
		if delay != recognitionResultDuration {
			t.Fatalf("delay = %v", delay)
		}
		pending = append(pending, fn)
		return func() { cancelled++ }
	}
	a := newTestApp()
	track := &player.Track{Name: "Station"}
	a.recognizedTrack = track
	a.recognitionSong = player.Song{Title: "First"}
	a.expireRecognitionLater()
	a.dismissRecognition()
	a.recognizedTrack = track
	a.recognitionSong = player.Song{Title: "Second"}
	a.expireRecognitionLater()
	pending[0]()
	if a.recognizedTrack == nil || a.recognitionSong.Title != "Second" {
		t.Fatal("an old timer dismissed the new result")
	}
	pending[1]()
	if a.recognizedTrack != nil || cancelled != 2 {
		t.Fatalf("result still open or timers not cancelled: track=%v cancelled=%d", a.recognizedTrack, cancelled)
	}
}

// Completed recognition schedules dismissal for both a matched song and no match.
func TestCompletedRecognitionExpires(t *testing.T) {
	old := recognitionAfter
	defer func() { recognitionAfter = old }()
	var expire func()
	recognitionAfter = func(_ *app, _ time.Duration, fn func()) func() {
		expire = fn
		return func() {}
	}
	for _, err := range []error{nil, player.ErrNoSong} {
		a := newTestApp()
		a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(sampleStations()[0])}, At: 0})
		track := a.pl.Current()
		a.recognizedTrack, a.recognizing = track, true
		expire = nil
		a.finishRecognition(track, track.Name, a.recognitionGen, player.Song{Title: "Song"}, err)
		if a.recognizing || a.recognizedTrack == nil || expire == nil {
			t.Fatalf("result not shown or expiration not scheduled for %v", err)
		}
		expire()
		if a.recognizedTrack != nil {
			t.Fatalf("result did not expire for %v", err)
		}
	}
}

// Closing a result dismisses it without changing playback.
func TestDismissRecognizedSong(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(sampleStations()[0])}, At: 0})
	track := a.pl.Current()
	a.recognizedTrack = track
	a.recognitionSong = player.Song{Title: "Song"}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("关闭识曲结果"); err != nil {
		t.Fatal(err)
	}
	if a.recognizedTrack != nil || a.pl.Current() != track {
		t.Fatal("dismiss did not close the card or changed playback")
	}
}

// With NM_FRAMES set, saves the recognition card in light and dark themes.
func TestRecognitionFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.pl.Restore(player.Session{Stations: []player.Track{stationTrack(sampleStations()[0])}, At: 0})
		a.recognizedTrack = a.pl.Current()
		a.recognitionSong = player.Song{Title: "笼 (电影《消失的她》片尾主题曲)", Artist: "Diamond Zhang", Cover: "test-cover"}
		data, err := os.ReadFile("../../resources/icon.png")
		if err != nil {
			t.Fatal(err)
		}
		bmp, _, err := decodeArt(data)
		if err != nil {
			t.Fatal(err)
		}
		a.art.art["test-cover"] = &artwork{bmp: bmp, used: a.art.now()}
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(dark)
		tt.Frame()
		name := "recognition-light.png"
		if dark {
			name = "recognition-dark.png"
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
