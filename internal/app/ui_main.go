package app

import (
	"runtime"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

const sidebarWidth = 220

// pageRef is one of the pages the sidebar shows.
type pageRef int

const (
	pageRadio       pageRef = iota // the stations, from the directory or the built-in list
	pageFavorites                  // the stations the listener favorited
	pageRecent                     // the stations they played lately
	pageRecognition                // the songs they identified
)

func (a *app) view(c *ui.Context) {
	t := *c.Theme()
	a.pal = paletteFor(&t)
	p := a.pal
	t.Accent, t.AccentText = p.accent, p.onAccent
	t.Selection = p.accent.Alpha(0.3)
	t.Text, t.TextMuted, t.Background = p.text, p.muted, p.bg
	t.Inverse, t.InverseText = p.surface, p.text
	t.Focus = p.accent.Alpha(0.45)
	if runtime.GOOS == "windows" {
		t.Font = "Microsoft YaHei UI"
	}
	c.SetTheme(&t)

	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	root := ui.Box(c).Fill()
	if !c.Focused(a.stationBox) {
		// Space plays and pauses wherever the focus is, as in Music. On
		// the container around everything, it comes before a button the
		// pointer focused, which would take Space as a click; on the
		// window, it would come after.
		root.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
		// And where nothing has the focus.
		c.OnShortcut(0, ui.KeySpace, a.pl.TogglePlay)
	}
	// Command-F searches the stations, from wherever the window is.
	c.OnShortcut(ui.Cmd, ui.KeyF, func() { a.stationBox.Focus(c) })
	root.Children(func() {
		page := ui.Row(c).Fill().AlignItems(ui.Stretch)
		page.Children(func() {
			side := ui.Column(c).Width(sidebarWidth)
			if !c.Vibrancy() {
				side.Background(p.sidebar)
			}
			side.Children(func() { a.sidebar(c) })
			// Keyed, so that a popover of the sidebar opening or closing
			// keeps what the page and its bars hold, their edges' fades.
			ui.Box(c.Key("main")).Grow(1).Background(p.bg).BorderWidth(0, 0, 0, 1).BorderColor(p.hairline).Children(func() {
				if a.page == pageRecognition {
					a.recognitionPage(c)
				} else {
					a.radioPage(c)
				}
				// The bars float over the page, which scrolls under them:
				// positioned, they paint over it.
				a.pageTop(c)
			})
		})
		a.playerBar(c)
	})
	if a.toast != "" {
		c.Toast(a.toast)
		a.toast = ""
	}
	a.toastViewport(c)
	a.syncWindows()
}

// showStations brings the window back to the stations, from the menu bar
// or the player bar.
func (a *app) showStations() {
	a.showPage(pageRadio)
}

// showPage brings the window to one of the sidebar's pages, at its start.
func (a *app) showPage(page pageRef) {
	if a.page != page {
		a.page = page
		a.listScroll = ui.ListState{}
	}
	a.showMain()
}

func (a *app) sidebar(c *ui.Context) {
	// Under the window buttons where they sit over the sidebar, as the
	// traffic lights do; elsewhere, as close to the top as to the sides.
	top := float32(14)
	if c.TitleBar().Left > 0 {
		top = topBar(c) + 4
	}
	// Where nothing in it takes the press, the sidebar moves the window, as
	// the title bar it reaches up into would.
	ui.Column(c).Fill().Padding(top, 14, playerBarHeight+18, 14).Gap(6).DragWindow().Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(8, 10, 24, 10).Children(func() {
			ui.Box(c).Size(32, 32).Radius(10).Background(a.pal.accent).Center().Children(func() {
				ui.Icon(c, iconRadio).Size(20, 20).TextColor(a.pal.onAccent)
			})
			ui.Text(c, appTitle).FontSize(18).FontWeight(800).TextColor(a.pal.text)
		})
		ui.Text(c, "电台探索").FontSize(11).FontWeight(600).TextColor(a.pal.faint).Margin(10, 10, 8, 0)
		a.navLink(c, "发现电台", iconRadio, pageRadio, 0)
		ui.Text(c, "我的电台").FontSize(11).FontWeight(600).TextColor(a.pal.faint).Margin(24, 10, 8, 0)
		a.navLink(c, "我的收藏", iconHeart, pageFavorites, len(a.lib.Favorites))
		a.navLink(c, "最近收听", iconClock, pageRecent, 0)
		a.navLink(c, "识曲历史", iconNote, pageRecognition, len(a.lib.Recognized))
		ui.Box(c).Grow(1)
		a.randomPlayButton(c)
	})
}

// navLink is one of the pages, marked while it shows, with how many
// stations it holds where that is worth saying.
func (a *app) navLink(c *ui.Context, label string, icon *ui.SVG, page pageRef, count int) {
	p := a.pal
	on := a.page == page
	b := ui.ButtonBase(c.Key("nav-" + label)).Height(42).PaddingX(12).Gap(12).Radius(11).
		AlignItems(ui.Center).Justify(ui.Start).FocusRing(false).Label(label)
	bg := ui.Transparent
	switch {
	case on:
		bg = p.accent.Alpha(0.16)
	case b.Hovered():
		bg = p.hover
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	fg := p.muted
	if on {
		fg = p.accent
	}
	b.Children(func() {
		ui.Icon(c, icon).Size(17, 17).TextColor(fg)
		ui.Text(c, label).FontSize(13.5).FontWeight(500).TextColor(fg).Grow(1).SingleLine()
		if count > 0 {
			ui.Textf(c, "%d", count).FontSize(11.5).FontFeatures("tnum").TextColor(p.faint)
		}
	})
	if b.Clicked() {
		a.showPage(page)
	}
}

// playerBarHeight is the height of the player along the foot of the window.
const playerBarHeight = 88

// pageGap is the room between the bar over a page and its content.
const pageGap = 12

// insets returns the room the bars over a page take at its top and its
// bottom: pages pad their content by it, and scroll under the bars.
func (a *app) insets(c *ui.Context) (top, bottom float32) {
	top = topBar(c)
	if runtime.GOOS == "darwin" {
		// The edge reaches a little below the traffic lights' strip.
		top += topEdgePad
	}
	return top, playerBarHeight
}

// topEdgePad is the room the edge over a page leaves below the strip, on
// macOS.
const topEdgePad = 6

// buttonsHeight is the height of the window buttons off macOS, which fill
// the edge over a page.
const buttonsHeight = topBarHeight + topEdgePad

// topBar returns the height of the strip along the top of the window, as
// tall as the window buttons and the room above and below them, which
// what sits in it centers on.
func topBar(c *ui.Context) float32 {
	if runtime.GOOS != "darwin" {
		// The whole edge, which the buttons fill: what sits in the strip
		// centers on their glyphs.
		return buttonsHeight
	}
	return max(c.TitleBar().Height, topBarHeight)
}

// topBarHeight is the least height of the strip along the top.
const topBarHeight = 40

// startMark marks where a page's content starts, first in what scrolls:
// once it has moved up, content is under the bar at the top.
func (a *app) startMark(c *ui.Context) {
	b := ui.Box(c.Key("start-mark")).Height(0).Bounds()
	if b.W > 0 {
		top, _ := a.insets(c)
		a.underTop = b.Y < top+pageGap-0.5
	}
}

// edgeShown returns how much of a scroll edge shows over el: fully while
// content is under its bar, and none at rest, as AppKit shows its edges.
func edgeShown(el ui.Element, under bool) float32 {
	target := float32(0)
	if under {
		target = 1
	}
	return el.AnimateTo("shown", target, ui.Animation{Duration: 180 * time.Millisecond})
}

// pageTop is the strip over the top of the page, frosting what scrolls
// under it as macOS 26's hard scroll edge does: it drags the window.
func (a *app) pageTop(c *ui.Context) {
	top, _ := a.insets(c)
	edge := ui.Box(c.Key("top-edge")).Absolute().Top(0).Left(0).Right(0).Height(top).DragWindow().
		Material(glass.ScrollEdge{Hard: true, Background: a.pal.bg})
	edge.Opacity(edgeShown(edge, a.underTop))
	bar := c.TitleBar()
	const actionWidth = 40
	const actionsWidth = 2 * actionWidth
	ui.Box(c.Key("update-bar")).Absolute().Top(0).Right(bar.Right + actionWidth).Width(actionWidth).
		Height(topBar(c)).Center().Children(func() {
		iconButton(c.Key("check-updates"), iconUpdate, 16, a.pal.muted, "检查更新…").
			Tooltip("检查更新").OnClick(updater.CheckForUpdates)
	})
	ui.Box(c.Key("theme-bar")).Absolute().Top(0).Right(bar.Right).Width(actionWidth).
		Height(topBar(c)).Center().Children(func() { a.themeButton(c) })
	if a.page == pageRadio {
		// The search container covers the edge behind it, so its empty
		// space must also drag the window; the input remains interactive.
		width, _ := c.Size()
		left := max(0, bar.Left-sidebarWidth)
		right := bar.Right + actionsWidth
		available := width - sidebarWidth - left - right
		ui.Box(c.Key("station-search-bar")).Absolute().Top(0).Left(left).Right(right).
			Height(topBar(c)).DragWindow().Center().Children(func() {
			ui.Box(c).Width(min(360, max(0, available-2*pagePad))).Children(func() {
				a.stationSearch(c)
			})
		})
	}
}
