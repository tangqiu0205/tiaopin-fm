package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// sleepAfter posts the deadline to the main thread even when no view draws.
var sleepAfter = func(a *app, delay time.Duration, fn func()) func() {
	t := time.AfterFunc(delay, func() { a.post(fn) })
	return func() { t.Stop() }
}

func (a *app) cancelSleepTimer() {
	if a.sleepStop != nil {
		a.sleepStop()
		a.sleepStop = nil
	}
	a.sleepGen++
	a.sleepDeadline = time.Time{}
}

func (a *app) startSleepTimer(now time.Time, minutes int) {
	if minutes < 1 || minutes > 24*60 {
		a.sleepError = "请输入 1–1440 分钟"
		return
	}
	a.cancelSleepTimer()
	a.sleepError = ""
	a.sleepOpen = false
	duration := time.Duration(minutes) * time.Minute
	a.sleepDeadline = now.Add(duration)
	gen := a.sleepGen
	a.sleepStop = sleepAfter(a, duration, func() {
		if gen != a.sleepGen || a.quitting {
			return
		}
		a.cancelSleepTimer()
		a.pl.Pause()
		a.toast = "定时结束，已暂停播放"
	})
}

func (a *app) startCustomSleepTimer(now time.Time) {
	minutes, err := strconv.Atoi(strings.TrimSpace(a.sleepCustom))
	if err != nil {
		a.sleepError = "请输入 1–1440 分钟"
		return
	}
	a.startSleepTimer(now, minutes)
}

func sleepRemaining(d time.Duration) string {
	seconds := int((max(d, 0) + time.Second - 1) / time.Second)
	if seconds >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", seconds/3600, seconds%3600/60, seconds%60)
	}
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// sleepButton anchors the timer popover alongside the volume controls.
func (a *app) sleepButton(c *ui.Context) {
	p := a.pal
	now := c.Now()
	active := !a.sleepDeadline.IsZero()
	color := p.muted
	if active {
		color = p.accent
		c.After(time.Second)
	}
	b := ui.ButtonBase(c.Key("sleep-timer")).Height(32).PaddingX(8).Gap(4).Radius(16).
		AlignItems(ui.Center).TextColor(color).FocusRing(false).Label("定时停止").Tooltip("定时停止播放")
	if b.Hovered() {
		b.Background(p.hover)
	}
	b.Children(func() {
		ui.Icon(c, iconThemeDark).Size(16, 16)
		if active {
			ui.Text(c, sleepRemaining(a.sleepDeadline.Sub(now))).FontSize(11).FontFeatures("tnum").TextColor(color).SingleLine()
		}
	})
	if b.Clicked() {
		a.sleepOpen = !a.sleepOpen
	}
	pop := popoverColors(p.dark)
	ui.PopoverBase(c.Key("sleep-popover"), b, &a.sleepOpen, func(panel ui.Element) {
		panel.AttachTo(b, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 8, 0).Width(290).Padding(14).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			Gap(10).Label("定时停止播放")
		ui.Text(c, "定时停止").FontSize(13).FontWeight(600).TextColor(p.text)
		ui.Row(c).Gap(6).Children(func() {
			for _, minutes := range []int{15, 30, 60} {
				label := fmt.Sprintf("%d 分钟", minutes)
				ui.Button(c.Key(label), label).Grow(1).OnClick(func() { a.startSleepTimer(now, minutes) })
			}
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.TextInputBase(c.Key("sleep-custom"), &a.sleepCustom).Grow(1).MinWidth(0).Height(32).PaddingX(8).
				Background(p.surface).Border(0.5, p.hairline).Radius(6).
				Placeholder("自定义分钟").Label("自定义分钟数").OnSubmit(func() { a.startCustomSleepTimer(now) })
			ui.Button(c.Key("sleep-set"), "确定").OnClick(func() { a.startCustomSleepTimer(now) })
		})
		if a.sleepError != "" {
			ui.Text(c, a.sleepError).FontSize(11).TextColor(p.accent)
		}
		if active {
			ui.Button(c.Key("sleep-cancel"), "关闭定时").OnClick(func() {
				a.cancelSleepTimer()
				a.sleepOpen = false
				a.sleepError = ""
			})
		}
	})
}
