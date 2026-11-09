package app

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// A quiet outlined action, not a navigation row: the dice and play arrow make
// the immediate playback action explicit, without changing the selected page.
func (a *app) randomPlayButton(c *ui.Context) {
	p := a.pal
	playable := false
	for _, st := range a.pageStations() {
		if stationPlayable(st) {
			playable = true
			break
		}
	}
	tip := "从当前列表随机选择一个电台，立即播放"
	if !playable {
		tip = "当前列表暂无可播放的电台"
	}
	b := ui.ButtonBase(c.Key("random-play")).AlignSelf(ui.Stretch).Height(56).Margin(8, 8, 2, 8).
		PaddingX(10).Gap(8).AlignItems(ui.Center).Radius(10).
		Border(1, p.accent.Alpha(0.14)).
		Disabled(!playable).Label("随机播放").Tooltip(tip)
	bg := p.accent.Alpha(0.035)
	if b.Hovered() && playable {
		bg = p.accent.Alpha(0.08)
		b.BorderColor(p.accent.Alpha(0.25))
	}
	b.Background(b.AnimateColor("bg", bg, ui.Animation{Duration: 120 * time.Millisecond}))
	pose := ui.Pose{}
	if b.Pressed() && playable {
		pose.Scale = 0.97
	}
	b.AnimatePose(pose, ui.Snappy)
	b.Children(func() {
		ui.Icon(c, iconDice).Size(24, 24).Shrink(0).TextColor(p.accent)
		ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, "随机播放").FontSize(12.5).FontWeight(500).TextColor(p.text).SingleLine()
			ui.Text(c, "点一下，立即开播").FontSize(10).TextColor(p.muted).SingleLine()
		})
		ui.Icon(c, iconPlay).Size(10, 10).Shrink(0).TextColor(p.accent)
	})
	if b.Clicked() {
		a.playRandom()
	}
}
