package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

// Selecting a timer then cancelling it leaves playback alone, even if its old callback arrives.
func TestSleepTimerControls(t *testing.T) {
	old := sleepAfter
	defer func() { sleepAfter = old }()
	var pending []func()
	cancelled := 0
	sleepAfter = func(_ *app, d time.Duration, fn func()) func() {
		if d != 15*time.Minute {
			t.Fatalf("duration = %v", d)
		}
		pending = append(pending, fn)
		return func() { cancelled++ }
	}
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			track := stationTrack(sampleStations()[0])
			track.Stream = "http://127.0.0.1:1/stream"
			a.pl.Restore(player.Session{Stations: []player.Track{track}, At: 0})
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			if err := tt.Click("播放/暂停"); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click("定时停止"); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click("15 分钟"); err != nil {
				t.Fatal(err)
			}
			if got := a.sleepDeadline.Sub(tt.Now()); got != 15*time.Minute {
				t.Fatalf("deadline = %v", got)
			}
			callback := pending[len(pending)-1]
			if err := tt.Click("定时停止"); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click("关闭定时"); err != nil {
				t.Fatal(err)
			}
			callback()
			if !a.sleepDeadline.IsZero() || !a.pl.Playing() {
				t.Fatal("cancelled timer stopped playback or remained active")
			}
		})
	}
	if cancelled != 2 {
		t.Fatalf("cancelled %d timers, want 2", cancelled)
	}
}

// A timer pauses a station still connecting and cannot be fired twice.
func TestSleepTimerExpiresWhileConnecting(t *testing.T) {
	old := sleepAfter
	defer func() { sleepAfter = old }()
	var expire func()
	sleepAfter = func(_ *app, _ time.Duration, fn func()) func() { expire = fn; return func() {} }
	a := newTestApp()
	track := player.Track{ID: 1, Name: "Station", Stream: "http://127.0.0.1:1/stream"}
	a.pl.Restore(player.Session{Stations: []player.Track{track}, At: 0})
	a.pl.TogglePlay()
	if !a.pl.Playing() || !a.pl.Loading() {
		t.Fatal("station did not start connecting")
	}
	a.startSleepTimer(time.Unix(100, 0), 15)
	expire()
	if a.pl.Playing() || !a.sleepDeadline.IsZero() {
		t.Fatal("timer did not pause and clear itself")
	}
	expire()
	if a.pl.Playing() {
		t.Fatal("stale timer resumed playback")
	}
}

// A timer ending while playback is already paused must not start it.
func TestSleepTimerDoesNotResumePausedStation(t *testing.T) {
	old := sleepAfter
	defer func() { sleepAfter = old }()
	var expire func()
	sleepAfter = func(_ *app, _ time.Duration, fn func()) func() { expire = fn; return func() {} }
	a := newTestApp()
	a.pl.Restore(player.Session{Stations: []player.Track{{ID: 1, Name: "Station"}}, At: 0})
	a.startSleepTimer(time.Unix(100, 0), 15)
	expire()
	if a.pl.Playing() || a.pl.Loading() {
		t.Fatal("timer resumed the paused station")
	}
}

// Invalid custom minutes leave an existing timer in place; valid minutes replace it.
func TestCustomSleepTimer(t *testing.T) {
	old := sleepAfter
	defer func() { sleepAfter = old }()
	sleepAfter = func(_ *app, _ time.Duration, _ func()) func() { return func() {} }
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("定时停止"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("30 分钟"); err != nil {
		t.Fatal(err)
	}
	original := a.sleepDeadline
	if err := tt.Click("定时停止"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("自定义分钟数"); err != nil {
		t.Fatal(err)
	}
	tt.Type("0")
	if err := tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	if a.sleepError == "" || a.sleepDeadline != original {
		t.Fatal("invalid input replaced the existing timer")
	}
	if err := tt.Click("自定义分钟数"); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("45")
	if err := tt.Click("确定"); err != nil {
		t.Fatal(err)
	}
	if a.sleepError != "" || a.sleepDeadline.Sub(tt.Now()) != 45*time.Minute || a.sleepOpen {
		t.Fatal("valid custom timer was not applied")
	}
}

// With NM_FRAMES set, saves the active timer popover in both window widths.
func TestSleepTimerFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, width := range []int{880, 1200} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.sleepOpen = true
		a.sleepDeadline = time.Now().Add(30 * time.Minute)
		tt := ui.NewTester(a.view, width, 800)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("sleep-%d.png", width)))
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
