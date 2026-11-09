package app

import (
	"fmt"
	"math"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestSidebarShowsApplicationVersion(t *testing.T) {
	old := mygo.App.Version()
	t.Cleanup(func() { mygo.App.SetVersion(old) })
	for _, width := range []int{880, 1200} {
		for _, version := range []string{"0.1.0", "1.2.3-beta.1", ""} {
			t.Run(fmt.Sprintf("%d/%s", width, version), func(t *testing.T) {
				mygo.App.SetVersion(version)
				a := newTestApp()
				tt := ui.NewTester(a.view, width, 580)
				tt.Frame()
				label := "v" + version
				if version == "" {
					label = "开发版"
				}
				r, ok := tt.Find(label)
				title, _ := tt.Find(appTitle)
				if !ok || r.X < title.X+title.W || r.X+r.W > sidebarWidth ||
					math.Abs(float64(r.Y+r.H/2-title.Y-title.H/2)) > 1 {
					t.Fatalf("version not beside app title: title=%+v version=%+v found=%v", title, r, ok)
				}
			})
		}
	}
}
