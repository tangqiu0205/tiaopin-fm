package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Search used to disappear with the virtual list's header; scrolling must
// leave it in the title bar, clear of window controls and still editable.
func TestStationSearchStaysFixedWhileScrolling(t *testing.T) {
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			for i := range 60 {
				st := sampleStations()[0]
				st.ID, st.Name = fmt.Sprint(i), fmt.Sprintf("Station %02d", i)
				a.radio.stations = append(a.radio.stations, st)
			}
			tt := ui.NewTester(a.view, width, 800)
			bar := ui.TitleBar{Height: buttonsHeight, Right: 138}
			tt.SetTitleBar(bar)
			tt.Frame()
			search, ok := tt.Find("搜索电台")
			if !ok {
				t.Fatal("search is not available")
			}
			if search.Y < 0 || search.Y+search.H > bar.Height || search.X+search.W > float32(width)-bar.Right {
				t.Fatalf("search overlaps the page or window controls: %v", search)
			}
			first, ok := tt.Find("Station 00")
			if !ok {
				t.Fatal("the first station is not available")
			}
			tt.Scroll(float32(width)-100, 450, 0, 1000)
			tt.Frame()
			if after, visible := tt.Find("Station 00"); visible && after.Y == first.Y {
				t.Fatal("the list did not scroll")
			}
			if after, visible := tt.Find("搜索电台"); !visible || after != search {
				t.Fatalf("search moved or disappeared: before %v, after %v", search, after)
			}
			tt.Key(ui.Cmd, ui.KeyF)
			tt.Type("jazz fm")
			if a.radio.query != "jazz fm" {
				t.Fatalf("search shortcut after scrolling typed %q", a.radio.query)
			}
			if err := tt.Click("搜索电台"); err != nil {
				t.Fatal(err)
			}
			tt.Type(" live")
			if a.radio.query != "jazz fm live" {
				t.Fatalf("fixed search did not accept input: %q", a.radio.query)
			}
		})
	}
}
