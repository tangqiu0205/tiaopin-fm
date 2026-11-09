package app

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The title-bar button cycles and saves all three appearances at either width.
func TestThemeButtonCyclesAndPersists(t *testing.T) {
	old := applyTheme
	var applied string
	applyTheme = func(s settings) { applied = s.Appearance }
	defer func() { applyTheme = old }()
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.path = filepath.Join(t.TempDir(), "settings.json")
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			for _, want := range []string{"light", "dark", "system"} {
				if err := tt.Click("切换主题"); err != nil {
					t.Fatal(err)
				}
				if a.settings.Appearance != want || readSettings(a.settings.path).Appearance != want || applied != want {
					t.Fatalf("appearance = %q, persisted = %q, applied = %q, want %q", a.settings.Appearance, readSettings(a.settings.path).Appearance, applied, want)
				}
			}
			if err := tt.Click("我的收藏"); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click("切换主题"); err != nil {
				t.Fatal(err)
			}
			if a.settings.Appearance != "light" {
				t.Fatal("theme button did not work on the favorites page")
			}
		})
	}
}

// A failed theme write reports that the chosen appearance could not be saved.
func TestThemeSaveFailure(t *testing.T) {
	old := applyTheme
	applyTheme = func(settings) {}
	defer func() { applyTheme = old }()
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.path = t.TempDir()
	a.cycleAppearance()
	if a.settings.Appearance != "light" || a.toast == "" {
		t.Fatalf("appearance = %q, toast = %q", a.settings.Appearance, a.toast)
	}
}
