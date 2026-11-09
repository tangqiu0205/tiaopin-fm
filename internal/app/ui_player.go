package app

import (
	"github.com/egoist/mygo/ui"
)

// volumeButton is the speaker, which opens the volume in a popover above
// it, as an AppKit popover: a native slider standing up, the volume in
// percent over it. The wheel over the speaker turns it up and down
// without opening it.
func (a *app) volumeButton(c *ui.Context, color ui.Color) {
	b := iconButton(c.Key("volume-button"), volumeIcon(a.volume), 16, color, "音量")
	if b.Clicked() {
		a.volumeOpen = !a.volumeOpen
	}
	b.HandleInput(func(ev ui.InputEvent) bool { return a.scrollVolume(ev) })
	pop := popoverColors(a.pal.dark)
	ui.PopoverBase(c.Key("volume-popover"), b, &a.volumeOpen, func(panel ui.Element) {
		const arrow = 7
		panel.AttachTo(b, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, arrow+3, 0).Width(52).Padding(11, 0, 12, 0).
			Radius(12).Background(pop.fill).Border(0.5, pop.edge).Shadow(0, 8, 24, -4, ui.RGBA(0, 0, 0, 0.2)).
			AlignItems(ui.Center).Gap(8).Label("音量")
		panel.Draw(func(p *ui.Painter, r ui.Rect) {
			// The arrow pointing at the speaker, its sides edged as the
			// panel is, over the panel's border.
			cx, foot := r.X+r.W/2, r.Y+r.H
			tip := (&ui.Path{}).MoveTo(cx-arrow-1, foot-0.75).LineTo(cx-1.2, foot+arrow-1.2).
				QuadTo(cx, foot+arrow, cx+1.2, foot+arrow-1.2).LineTo(cx+arrow+1, foot-0.75).Close()
			p.FillPath(tip, pop.fill)
			edge := (&ui.Path{}).MoveTo(cx-arrow-1, foot).LineTo(cx-1.2, foot+arrow-1.2).
				QuadTo(cx, foot+arrow, cx+1.2, foot+arrow-1.2).LineTo(cx+arrow+1, foot)
			p.StrokePath(edge, 0.5, pop.edge)
		})
		ui.Textf(c, "%d%%", int(a.volume*100+0.5)).FontSize(11).FontWeight(500).FontFeatures("tnum").TextColor(a.pal.muted)
		a.nativeVolumeSlider(c, pop)
	})
}

// popover holds the colors of a popover as AppKit draws them.
type popover struct {
	fill, edge, track, knob ui.Color
}

func popoverColors(dark bool) popover {
	if dark {
		return popover{fill: ui.Hex("#2b2b2e"), edge: ui.RGBA(255, 255, 255, 0.14), track: ui.RGBA(255, 255, 255, 0.16), knob: ui.Hex("#ececec")}
	}
	return popover{fill: ui.Hex("#f6f6f6"), edge: ui.RGBA(0, 0, 0, 0.12), track: ui.RGBA(0, 0, 0, 0.12), knob: ui.Hex("#ffffff")}
}

// nativeVolumeSlider is a vertical slider as AppKit draws one: a thin
// track, filled with the accent from the foot to a round knob.
func (a *app) nativeVolumeSlider(c *ui.Context, pop popover) {
	const knob, track = 20, 4
	s := ui.SliderBase(c.Key("volume"), &a.volume, 0, 1).Vertical().Size(knob+8, 124).PaddingY(knob / 2).
		Step(0.01).AutoFocus().FocusRing(false).Label("音量")
	if s.Changed() {
		a.pl.SetVolume(a.volume)
	}
	s.HandleInput(func(ev ui.InputEvent) bool { return a.scrollVolume(ev) })
	pressed := s.Pressed()
	v := float32(a.volume)
	accent := a.pal.accent
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		top, foot := r.Y+knob/2, r.Y+r.H-knob/2
		x := r.X + (r.W-track)/2
		y := foot - (foot-top)*v
		p.Fill(ui.Rect{X: x, Y: top - track/2, W: track, H: foot - top + track}, pop.track, track/2)
		p.Fill(ui.Rect{X: x, Y: y, W: track, H: foot - y + track/2}, accent, track/2)
		k := ui.Rect{X: r.X + (r.W-knob)/2, Y: y - knob/2, W: knob, H: knob}
		p.Shadow(k, knob/2, 0, 0.5, 1.5, 0, ui.RGBA(0, 0, 0, 0.3))
		p.Shadow(k, knob/2, 0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.12))
		fill := pop.knob
		if pressed {
			fill = fill.Mix(ui.RGB(0, 0, 0), 0.06)
		}
		p.Fill(k, fill, knob/2)
		p.Stroke(k, ui.RGBA(0, 0, 0, 0.08), knob/2, 0.5)
	})
}

// scrollVolume turns the volume up as the wheel or the fingers scroll up.
func (a *app) scrollVolume(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	a.volume = min(1, max(0, a.volume-float64(ev.DY)/400))
	a.pl.SetVolume(a.volume)
	return true
}

// volumeIcon is the speaker for volume v: silent, or with one or two waves.
func volumeIcon(v float64) *ui.SVG {
	switch {
	case v == 0:
		return iconMute
	case v < 0.4:
		return iconVolumeLow
	default:
		return iconVolume
	}
}

// muteButton is the speaker, which mutes and unmutes.
func (a *app) muteButton(c *ui.Context, color ui.Color) {
	icon := iconVolume
	if a.volume == 0 {
		icon = iconMute
	}
	if iconButton(c.Key("mute"), icon, 16, color, "静音").Clicked() {
		if a.volume > 0 {
			a.mutedFrom, a.volume = a.volume, 0
		} else {
			a.volume = max(a.mutedFrom, 0.5)
		}
		a.pl.SetVolume(a.volume)
	}
}

// volumeSlider shows a mute button, a slim slider and a fixed-width percentage.
func (a *app) volumeSlider(c *ui.Context, color, track ui.Color, width float32) {
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		a.muteButton(c, color)
		s := ui.SliderBase(c.Key("volume"), &a.volume, 0, 1).Width(width).Height(16).FocusRing(false).Label("音量")
		if s.Changed() {
			a.pl.SetVolume(a.volume)
		}
		s.HandleInput(func(ev ui.InputEvent) bool { return a.scrollVolume(ev) })
		hot := s.Hovered() || s.Pressed()
		thick := float32(4)
		if hot {
			thick = 5
		}
		thick = s.AnimateTo("thick", thick, ui.Snappy)
		v := float32(a.volume)
		s.Draw(func(p *ui.Painter, r ui.Rect) {
			y := r.Y + (r.H-thick)/2
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: thick}, track, thick/2)
			p.Fill(ui.Rect{X: r.X, Y: y, W: max(thick, r.W*v), H: thick}, color, thick/2)
		})
		ui.Textf(c, "%d%%", int(a.volume*100+0.5)).Width(36).Shrink(0).
			FontSize(11).FontFeatures("tnum").TextColor(a.pal.muted).SingleLine()
	})
}

// transport is the row of controls: the station before, play, and the
// one after.
func (a *app) transport(c *ui.Context, color, dim, on ui.Color, size float32) {
	pl := a.pl
	ui.Row(c).Gap(size * 0.5).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if iconButton(c.Key("prev"), iconPrev, size*0.85, color, "上一个电台").Clicked() {
			pl.Prev()
		}
		if playButton(c.Key("play"), pl.Playing(), size*1.25, color).Clicked() {
			pl.TogglePlay()
		}
		if iconButton(c.Key("next"), iconNext, size*0.85, color, "下一个电台").Clicked() {
			pl.Next()
		}
	})
}

// liveBadge marks what plays as a station's, live, in the place a song's
// progress and length go: it has neither.
func (a *app) liveBadge(c *ui.Context) {
	// Nothing plays: there is no live to speak of.
	if a.pl.Current() == nil {
		return
	}
	p := a.pal
	label := "直播中"
	if a.pl.Loading() {
		label = "正在连接…"
	} else if !a.pl.Playing() {
		label = "已暂停"
	}
	ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "LIVE").FontSize(9).FontWeight(800).LetterSpacing(0.7).TextColor(p.accent).
			Padding(1.5, 5).Radius(3).Background(p.accent.Alpha(0.12))
		ui.Text(c, label).FontSize(11).TextColor(p.muted)
	})
}

// playerBar runs along the foot of the content: what plays, its controls,
// and the volume.
func (a *app) playerBar(c *ui.Context) {
	p := a.pal
	pl := a.pl
	tr := pl.Current()
	// An opaque surface keeps the controls readable above scrolling cards.
	ui.Box(c.Key("bottom-edge")).Absolute().Left(0).Right(0).Bottom(0).Height(playerBarHeight).
		Background(p.surface).BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).
		Shadow(0, -4, 20, -8, p.cardShade)
	// The controls keep the middle, between sides that share what is left.
	width, _ := c.Size()
	controls := float32(32)
	if width >= 1000 {
		controls = 140
	}
	ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Height(playerBarHeight).PaddingX(20).Gap(20).AlignItems(ui.Center).Children(func() {
		// What plays.
		ui.Row(c).Grow(1).Basis(0).MinWidth(150).Children(func() {
			if tr == nil {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(48, 48).Radius(8).Background(p.hover).Center().Children(func() {
						ui.Icon(c, iconRadio).Size(18, 18).TextColor(p.faint)
					})
					ui.Text(c, "未在播放").FontSize(13).TextColor(p.muted)
				})
				return
			}
			info := ui.ButtonBase(c.Key("info")).Gap(12).AlignItems(ui.Center).FocusRing(false).Shrink(1).MinWidth(0).Label("打开电台")
			hovered := info.Hovered()
			info.Children(func() {
				ui.Box(c).Size(48, 48).Shrink(0).Children(func() {
					a.cover(c, tr, 48, 8).Shadow(0, 2, 8, 0, p.cardShade)
					if hovered {
						ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Radius(8).Center().
							Background(ui.RGBA(0, 0, 0, 0.35)).TextColor(ui.RGB(255, 255, 255)).Children(func() {
							ui.Icon(c, iconRadio).Size(18, 18)
						})
					}
				})
				ui.Column(c).Shrink(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, tr.Name).FontSize(13.5).FontWeight(600).SingleLine()
					// What it is waiting on shows in place of where it is.
					sub, color := tr.Detail, p.muted
					switch {
					case pl.Loading():
						sub = "正在连接…"
					case pl.Err() != "":
						sub, color = pl.Err(), p.accent
					}
					ui.Text(c, sub).FontSize(12).TextColor(color).SingleLine()
				})
			})
			if info.Clicked() {
				a.showStations()
			}
		})

		// Controls, over the live badge in the place of a song's progress.
		ui.Column(c).Width(260).Shrink(1).MinWidth(210).Gap(4).AlignItems(ui.Stretch).Children(func() {
			ui.Row(c).Gap(18).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				if iconButton(c.Key("prev"), iconPrev, 18, p.muted, "上一个电台").Clicked() {
					pl.Prev()
				}
				play := ui.ButtonBase(c.Key("play")).Size(44, 44).Radius(22).Center().
					Background(p.accent).TextColor(p.onAccent).Label("播放/暂停").
					Shadow(0, 3, 12, -3, p.accent.Alpha(0.35))
				if play.Hovered() {
					play.Background(p.accent.Mix(p.text, 0.08))
				}
				play.Children(func() { playPause(c, play, pl.Playing(), 20) })
				if play.Clicked() {
					pl.TogglePlay()
				}
				if iconButton(c.Key("next"), iconNext, 18, p.muted, "下一个电台").Clicked() {
					pl.Next()
				}
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Margin(-2, 0, 0, 0).Children(func() {
				a.liveBadge(c)
			})
		})

		// The volume.
		ui.Row(c).Grow(1).Basis(0).MinWidth(controls).Justify(ui.End).AlignItems(ui.Center).Gap(4).Children(func() {
			if tr != nil {
				if a.recognizing && a.recognizedTrack == tr {
					ui.Spinner(c.Key("recognize-spinner")).FontSize(16).TextColor(p.muted).Label("正在识别歌曲")
				} else if iconButton(c.Key("recognize"), iconNote, 16, p.muted, "识别歌曲（发送音频指纹至 Shazam）").Clicked() {
					a.recognizeCurrent()
				}
			}
			a.sleepButton(c)
			if width >= 1000 {
				a.volumeSlider(c, p.accent, p.hairline, 80)
			} else {
				a.volumeButton(c, p.muted)
			}
		})
	})
	if tr != nil && a.recognizedTrack == tr {
		a.recognitionCard(c)
	}
}

// recognitionCard keeps the station visible while showing a separate song result.
func (a *app) recognitionCard(c *ui.Context) {
	p := a.pal
	ui.Column(c.Key("recognition-card")).Absolute().Right(20).Bottom(playerBarHeight+8).
		Width(280).Padding(8, 12, 12, 12).Gap(4).Radius(12).Background(p.surface).
		Border(1, p.hairline).Shadow(0, 4, 20, -4, p.cardShade).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
			ui.Icon(c, iconNote).Size(13, 13).TextColor(p.accent)
			ui.Text(c, "识曲结果").FontSize(11).FontWeight(600).TextColor(p.muted).Grow(1)
			if a.recognitionText == "" {
				song := a.recognitionSong
				services := c.Services()
				iconButton(c.Key("copy-recognition"), iconCopy, 14, p.muted, "复制歌曲信息").OnClick(func() {
					text := song.Title
					if song.Artist != "" {
						text = song.Artist + " - " + song.Title
					}
					services.WriteClipboard(text)
					a.dismissRecognition()
					a.toast = "已复制歌曲信息"
				})
			}
			if iconButton(c.Key("dismiss-recognition"), iconX, 14, p.muted, "关闭识曲结果").Clicked() {
				a.dismissRecognition()
			}
		})
		if a.recognitionText != "" {
			ui.Text(c, a.recognitionText).FontSize(12).TextColor(p.text)
			return
		}
		song := a.recognitionSong
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			a.songCover(c.Key("recognition-cover"), song.Cover)
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, song.Title).FontSize(13).FontWeight(600).TextColor(p.text).SingleLine()
				if song.Artist != "" {
					ui.Text(c, song.Artist).FontSize(12).TextColor(p.muted).SingleLine()
				}
			})
		})
	})
}

// white is what the notch paints its controls in, over the black of the
// housing.
var white = ui.RGB(255, 255, 255)

// b2f is a bool as 0 or 1, for animating between.
func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
