package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The compact update button stays beside the theme button on every page,
// clear of both the search input and the native window controls.
func TestUpdateButtonVisible(t *testing.T) {
	for _, width := range []int{880, 1200} {
		for _, controls := range []ui.TitleBar{{Right: 138, Height: 46}, {Left: 80, Height: 46}} {
			for _, page := range []pageRef{pageRadio, pageFavorites, pageRecent, pageRecognition} {
				t.Run(fmt.Sprintf("%d/%v/%d", width, controls, page), func(t *testing.T) {
					a := newTestApp()
					a.page = page
					tt := ui.NewTester(a.view, width, 580)
					tt.SetTitleBar(controls)
					tt.Frame()
					r, ok := tt.Find("检查更新…")
					if !ok || r.W != 32 || r.H != 32 || r.Y < 0 || r.Y+r.H > buttonsHeight || r.X < sidebarWidth {
						t.Fatalf("update button unavailable: bounds=%+v found=%v", r, ok)
					}
					theme, ok := tt.Find("切换主题")
					if !ok || r.X+r.W > theme.X || r.Y != theme.Y || theme.X+theme.W > float32(width)-controls.Right {
						t.Fatalf("title-bar actions overlap: update=%+v theme=%+v", r, theme)
					}
					if page == pageRadio {
						search, ok := tt.Find("搜索电台")
						if !ok || search.X+search.W > r.X {
							t.Fatalf("search overlaps update button: search=%+v update=%+v", search, r)
						}
					}
				})
			}
		}
	}
}
