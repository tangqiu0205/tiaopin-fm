package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// library is what the listener kept: the stations they favorited, and the
// ones they played lately, kept in library.json in the user data
// directory.
type library struct {
	Favorites  []radio.Station  `json:"favorites"`
	Recent     []radio.Station  `json:"recent"`
	Recognized []recognizedSong `json:"recognized,omitempty"`

	path string // where it was loaded from, and is saved to
}

// recentKeep is how many stations the recent list keeps: a station played
// again goes to its head rather than taking a second place.
const recentKeep = 40

const recognitionKeep = 200

type recognizedSong struct {
	Song    player.Song `json:"song"`
	Station string      `json:"station"`
	At      time.Time   `json:"at"`
}

func recognitionKey(song player.Song) string {
	if song.ID != "" {
		return "id:" + song.ID
	}
	return "name:" + strings.ToLower(strings.TrimSpace(song.Title)) + "\x00" + strings.ToLower(strings.TrimSpace(song.Artist))
}

func (l *library) addRecognition(song player.Song, station string, at time.Time) error {
	key := recognitionKey(song)
	kept := []recognizedSong{{Song: song, Station: station, At: at}}
	for _, item := range l.Recognized {
		if recognitionKey(item.Song) != key && len(kept) < recognitionKeep {
			kept = append(kept, item)
		}
	}
	l.Recognized = kept
	return l.save()
}

func (l *library) removeRecognition(key string) error {
	for i, item := range l.Recognized {
		if recognitionKey(item.Song) == key {
			l.Recognized = append(l.Recognized[:i], l.Recognized[i+1:]...)
			return l.save()
		}
	}
	return nil
}

func libraryPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "library.json"), nil
}

// loadLibrary reads what the listener kept, or an empty library.
func loadLibrary() *library {
	l := &library{}
	if path, err := libraryPath(); err == nil {
		if b, err := os.ReadFile(path); err == nil {
			json.Unmarshal(b, l)
		}
		l.path = path
	}
	return l
}

// save keeps the library where it was loaded from; a library never loaded,
// as in tests, is not saved.
func (l *library) save() error {
	if l == nil || l.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(l.path, b, 0o600)
}

// has reports whether the station is one of the favorites.
func (l *library) has(id string) bool {
	if l == nil || id == "" {
		return false
	}
	for _, st := range l.Favorites {
		if st.ID == id {
			return true
		}
	}
	return false
}

// toggle adds the station to the favorites, or takes it out, and reports
// whether it is one now.
func (l *library) toggle(st radio.Station) bool {
	if l == nil || st.ID == "" {
		return false
	}
	for i, f := range l.Favorites {
		if f.ID == st.ID {
			l.Favorites = append(l.Favorites[:i], l.Favorites[i+1:]...)
			l.save()
			return false
		}
	}
	l.Favorites = append([]radio.Station{st}, l.Favorites...)
	l.save()
	return true
}

// played notes that the station plays: it goes to the head of the recent
// list, out of wherever it was, and the list keeps its last recentKeep.
func (l *library) played(st radio.Station) {
	if l == nil || st.ID == "" {
		return
	}
	kept := []radio.Station{st}
	for _, r := range l.Recent {
		if r.ID != st.ID && len(kept) < recentKeep {
			kept = append(kept, r)
		}
	}
	l.Recent = kept
	l.save()
}
