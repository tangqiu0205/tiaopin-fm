package app

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// Region and language selections used to disappear on restart; both must
// survive a disk round trip and constrain the very first directory request.
func TestRadioDropdownsPersistAcrossRestart(t *testing.T) {
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			requests := make(chan radio.Filter, 8)
			old := findStations
			findStations = func(_ context.Context, f radio.Filter, _, _ int) (radio.Page, error) {
				requests <- f
				return radio.Page{}, nil
			}
			t.Cleanup(func() { findStations = old })
			checkRequest := func(want radio.Filter) {
				t.Helper()
				select {
				case got := <-requests:
					if got != want {
						t.Fatalf("request %+v, want %+v", got, want)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("missing directory request")
				}
			}
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.path = filepath.Join(t.TempDir(), "settings.json")
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			click := func(label string) {
				t.Helper()
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			click("地区")
			click("美国")
			checkRequest(radio.Filter{CountryCode: "US"})
			click("语言")
			click("英语")
			want := radio.Filter{CountryCode: "US", Language: "english"}
			checkRequest(want)
			restarted := newTestApp()
			restarted.settings = readSettings(a.settings.path)
			if restarted.settings.RadioCountry != "US" || restarted.settings.RadioLanguage != "english" {
				t.Fatal("dropdown choices were not saved as stable API values")
			}
			restarted.restoreRadioFilters()
			restarted.loadStations()
			checkRequest(want)
			tt = ui.NewTester(restarted.view, width, 800)
			tt.Frame()
			if restarted.radio.country != "美国" || restarted.radio.language != "英语" {
				t.Fatal("the restored choices were lost while building the view")
			}
			click("地区")
			click("全部地区")
			checkRequest(radio.Filter{Language: "english"})
			click("语言")
			click("全部语言")
			checkRequest(radio.Filter{})
			again := newTestApp()
			again.settings = readSettings(a.settings.path)
			again.restoreRadioFilters()
			if f := again.stationFilter(); f != (radio.Filter{}) {
				t.Fatalf("cleared choices returned after restart: %+v", f)
			}
		})
	}
}

// A failed preference write must surface an error instead of claiming that
// the region and language will be remembered.
func TestRadioDropdownSaveFailureIsReported(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.path = t.TempDir()
	a.radio.country, a.radio.language = "美国", "英语"
	a.saveRadioFilters()
	if a.toast == "" {
		t.Fatal("a failed settings write was not reported")
	}
}
