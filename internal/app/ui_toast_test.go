package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A notification must not block navigation and disappears after its timeout.
func TestToastDoesNotBlockNavigation(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	message := "已复制歌曲信息"
	a.toast = message
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if _, ok := tt.Find(message); !ok {
		t.Fatal("notification did not appear")
	}
	if err := tt.Click("识曲历史"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageRecognition {
		t.Fatal("notification blocked navigation")
	}
	tt.Advance(5 * time.Second)
	tt.Frame()
	if _, ok := tt.Find(message); ok {
		t.Fatal("notification did not expire")
	}
}

// With NM_FRAMES set, saves a notification in light and dark themes.
func TestToastFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.toast = "已复制歌曲信息"
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(dark)
		tt.Frame()
		name := "toast-light.png"
		if dark {
			name = "toast-dark.png"
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
