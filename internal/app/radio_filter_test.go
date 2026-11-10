package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// Content, region and language combine with name search;
// changing or clearing one constraint must preserve all the others. Resetting
// ListState inside the header previously crashed while its rows were building.
func TestRadioFiltersCombineAndClear(t *testing.T) {
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			requests := make(chan radio.Filter, 16)
			old := findStations
			findStations = func(_ context.Context, f radio.Filter, limit, offset int) (radio.Page, error) {
				requests <- f
				return radio.Page{}, nil
			}
			t.Cleanup(func() { findStations = old })
			a := newTestApp()
			a.settings = defaultSettings()
			a.radio.stations = sampleStations()
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			region, regionOK := tt.Find("地区")
			language, languageOK := tt.Find("语言")
			category, categoryOK := tt.Find("全部内容")
			if !regionOK || !languageOK || !categoryOK || region.X+region.W > language.X || language.X+language.W > float32(width)-pagePad || region.Y+region.H > category.Y {
				t.Fatalf("filters not aligned above categories: region=%+v language=%+v category=%+v", region, language, category)
			}
			click := func(label string) {
				t.Helper()
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			check := func(want radio.Filter) {
				t.Helper()
				select {
				case got := <-requests:
					if got != want {
						t.Fatalf("request %+v, want %+v", got, want)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("no directory request after changing a filter")
				}
			}
			click("音乐")
			want := radio.Filter{Tag: "music"}
			check(want)
			click("地区")
			click("美国")
			want.CountryCode = "US"
			check(want)
			click("语言")
			click("英语")
			want.Language = "english"
			check(want)
			click("搜索电台")
			tt.Type("jazz fm")
			tt.Key(0, ui.KeyEnter)
			want.Name = "jazz fm"
			check(want)
			click("清除搜索")
			want.Name = ""
			check(want)
			for _, category := range []struct{ label, tag string }{
				{"体育", "sports"}, {"文化", "culture"}, {"娱乐", "entertainment"},
			} {
				click(category.label)
				want.Tag = category.tag
				check(want)
			}
			click("全部内容")
			want.Tag = ""
			check(want)
			click("地区")
			click("全部地区")
			want.CountryCode = ""
			check(want)
			click("语言")
			click("全部语言")
			want.Language = ""
			check(want)
		})
	}
}
