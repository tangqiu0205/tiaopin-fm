package app

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

// Recognizing the same song updates its history entry and persists the latest station.
func TestRecognitionHistoryPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	l := &library{path: path}
	song := player.Song{ID: "123", Title: "Song", Artist: "Artist", Cover: "https://example.com/cover"}
	if err := l.addRecognition(song, "First", time.Unix(10, 0)); err != nil {
		t.Fatal(err)
	}
	if err := l.addRecognition(song, "Second", time.Unix(20, 0)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var restored library
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Recognized) != 1 || restored.Recognized[0].Station != "Second" || restored.Recognized[0].Song != song || restored.Recognized[0].At.Unix() != 20 {
		t.Fatalf("restored history = %+v", restored.Recognized)
	}
	if err := l.removeRecognition(recognitionKey(song)); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Unmarshal into a fresh value: omitted empty history must remain empty.
	var afterDelete library
	if err := json.Unmarshal(data, &afterDelete); err != nil || len(afterDelete.Recognized) != 0 {
		t.Fatalf("history after deletion = %+v, error %v", afterDelete.Recognized, err)
	}
}

// The history keeps only the most recently identified songs, even without Shazam IDs.
func TestRecognitionHistoryLimit(t *testing.T) {
	l := &library{}
	for i := 0; i < recognitionKeep+1; i++ {
		if err := l.addRecognition(player.Song{Title: fmt.Sprintf("Song %d", i), Artist: "Artist"}, "Station", time.Unix(int64(i), 0)); err != nil {
			t.Fatal(err)
		}
	}
	if len(l.Recognized) != recognitionKeep || l.Recognized[0].Song.Title != "Song 200" || l.Recognized[recognitionKeep-1].Song.Title != "Song 1" {
		t.Fatalf("unexpected history bounds: %d entries", len(l.Recognized))
	}
}

// Navigation, copying and deletion of a history entry do not change playback.
func TestRecognitionHistoryActions(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	if err := a.lib.addRecognition(player.Song{ID: "1", Title: "Song", Artist: "Artist"}, "Station", time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	if err := tt.Click("识曲历史"); err != nil {
		t.Fatal(err)
	}
	if a.page != pageRecognition {
		t.Fatal("history did not open")
	}
	if err := tt.Click("复制歌曲：Song"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "Artist - Song" {
		t.Fatalf("clipboard = %q", got)
	}
	if err := tt.Click("删除识曲记录：Song"); err != nil {
		t.Fatal(err)
	}
	if len(a.lib.Recognized) != 0 || a.pl.Current() != nil {
		t.Fatal("delete left the song or changed playback")
	}
}

// With NM_FRAMES set, saves the recognition history in light and dark themes.
func TestRecognitionHistoryFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip()
	}
	for _, dark := range []bool{false, true} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.page = pageRecognition
		if err := a.lib.addRecognition(player.Song{ID: "1", Title: "笼 (电影《消失的她》片尾主题曲)", Artist: "Diamond Zhang"}, "音乐电台", time.Unix(100, 0)); err != nil {
			t.Fatal(err)
		}
		tt := ui.NewTester(a.view, 1200, 800)
		tt.SetDark(dark)
		tt.Frame()
		name := "recognition-history-light.png"
		if dark {
			name = "recognition-history-dark.png"
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
