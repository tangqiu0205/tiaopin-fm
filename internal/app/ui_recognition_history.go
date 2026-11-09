package app

import (
	"log"
	"strings"

	"github.com/egoist/mygo/ui"
)

// songCover shows a song's artwork, or a music note while it is unavailable.
func (a *app) songCover(c *ui.Context, url string) {
	p := a.pal
	bmp := bitmap(a.art.get(url, false))
	cover := ui.Box(c).Size(44, 44).Shrink(0).Radius(8).Background(p.hover).Center()
	fade := artIn(cover, bmp)
	cover.Children(func() {
		ui.Icon(c, iconNote).Size(17, 17).TextColor(p.faint)
		artImage(c, bmp, 8, fade)
	})
}

// recognitionPage lists saved songs without changing the current station.
func (a *app) recognitionPage(c *ui.Context) {
	top, bottom := a.insets(c)
	a.underTop = true
	items := a.lib.Recognized
	ui.List(c.Key("recognition-history"), &a.listScroll, len(items)+1).
		ItemKey(func(i int) any {
			if i == 0 {
				return "header"
			}
			return recognitionKey(items[i-1].Song)
		}).Grow(1).Padding(top+pageGap, pagePad, bottom+24, pagePad).ScrollbarInsets(top, 0, bottom, 0).
		Rows(func(row ui.ListRow) {
			c, i := row.Context, row.Index
			if i == 0 {
				ui.Column(c).Padding(0, 0, 16, 0).Children(func() {
					a.startMark(c)
					a.heading(c, "识曲历史", "听过的歌曲")
					if len(items) == 0 {
						ui.Box(c).Height(56).Center().Children(func() {
							ui.Text(c, "识别过的歌曲会保存在这里").FontSize(13).TextColor(a.pal.muted)
						})
					}
				})
				return
			}
			item := items[i-1]
			key := recognitionKey(item.Song)
			ui.Row(c.Key(key)).Height(72).Padding(8, 12).Gap(12).AlignItems(ui.Center).
				Radius(10).Background(a.pal.surface).Border(1, a.pal.hairline).Margin(0, 0, 8, 0).Children(func() {
				a.songCover(c.Key("cover"), item.Song.Cover)
				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(4).Children(func() {
					ui.Text(c, item.Song.Title).FontSize(13.5).FontWeight(600).TextColor(a.pal.text).SingleLine()
					parts := []string{item.Song.Artist, item.Station}
					if !item.At.IsZero() {
						parts = append(parts, item.At.Local().Format("2006-01-02 15:04"))
					}
					var details []string
					for _, part := range parts {
						if part != "" {
							details = append(details, part)
						}
					}
					ui.Text(c, strings.Join(details, " · ")).FontSize(12).TextColor(a.pal.muted).SingleLine()
				})
				services := c.Services()
				iconButton(c.Key("copy"), iconCopy, 14, a.pal.muted, "复制歌曲："+item.Song.Title).OnClick(func() {
					text := item.Song.Title
					if item.Song.Artist != "" {
						text = item.Song.Artist + " - " + text
					}
					services.WriteClipboard(text)
					a.toast = "已复制歌曲信息"
				})
				iconButton(c.Key("remove"), iconTrash, 14, a.pal.muted, "删除识曲记录："+item.Song.Title).OnClick(func() {
					if err := a.lib.removeRecognition(key); err != nil {
						log.Printf("save recognition history: %v", err)
						a.toast = "无法保存识曲历史：" + err.Error()
					}
				})
			})
		})
}
