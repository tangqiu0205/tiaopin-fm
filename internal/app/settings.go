package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
)

// settings are the listener's choices, kept in settings.json in the user
// data directory.
type settings struct {
	// Notch shows the quick player around the notch: the listener turns
	// it on.
	Notch bool `json:"notch"`
	// NotchActivity shows the cover and the music's levels beside the
	// notch while music plays, which hides the menu bar there.
	NotchActivity bool `json:"notchActivity"`
	// NotchVirtual keeps a notch of the app's own showing at the top of a
	// display without one; else it shows only while the pointer asks.
	NotchVirtual bool `json:"notchVirtual"`
	// NotchGreeted says the notch showed itself, once, as the first song
	// played (greet).
	NotchGreeted bool `json:"notchGreeted"`
	// Appearance is light, dark, or system to follow the system.
	Appearance string `json:"appearance"`
	// Volume is where the volume was left, from 0 to 1.
	Volume float64 `json:"volume"`
	// AppIcon is where the app's icon shows on macOS; elsewhere
	// it is always in the notification area.
	AppIcon string `json:"appIcon"`
	// RadioCountry and RadioLanguage keep stable API values, not UI labels.
	RadioCountry  string `json:"radioCountry,omitempty"`
	RadioLanguage string `json:"radioLanguage,omitempty"`

	path string // where they were loaded from, and are saved to
}

func defaultSettings() settings {
	return settings{NotchActivity: true, Appearance: "system", Volume: 0.8, AppIcon: "dock-and-menubar"}
}

func settingsPath() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// oldNames are what the app was called before it took its bundle's name,
// and kept its data under, the latest first.
var oldNames = []string{"MyGo FM Dev", "MyGo FM", "MyGo 云音乐", "网易云音乐"}

// moveOldData moves the user data kept under an old name to the directory
// of the app's name, unless that holds something already: the settings and
// where playback was left go on. It must run before anything reads them.
func moveOldData() {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	for _, name := range oldNames {
		moveDir(filepath.Join(filepath.Dir(dir), name), dir)
	}
}

// moveDir moves the directory old to dir when dir is empty, as Path made
// it, and old exists.
func moveDir(old, dir string) {
	if old == dir {
		return
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return
	}
	if _, err := os.Stat(old); err != nil {
		return
	}
	if os.Remove(dir) == nil {
		if err := os.Rename(old, dir); err != nil {
			os.MkdirAll(dir, 0o700)
		}
	}
}

// loadSettings reads the settings, or returns the defaults.
func loadSettings() settings {
	path, err := settingsPath()
	if err != nil {
		return defaultSettings()
	}
	return readSettings(path)
}

func readSettings(path string) settings {
	s := defaultSettings()
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &s)
	}
	s.path = path
	return s
}

// save keeps the settings where they were loaded from; defaults that were
// never loaded, as in tests, are not saved.
func (s settings) save() error {
	path := s.path
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// applyAppearance makes the windows light, dark, or follow the system.
func (s settings) applyAppearance() {
	switch s.Appearance {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}
