package app

import "github.com/egoist/mygo/ui"

// toastViewport gives every in-app notification the same surface as the app.
func (a *app) toastViewport(c *ui.Context) {
	p := a.pal
	width, _ := c.Size()
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Bottom(playerBarHeight + 12).AlignItems(ui.Center).Gap(8)
		for _, notice := range toasts {
			toast := ui.ToastBase(c, notice)
			toast.Root.Row().AlignItems(ui.Center).Gap(10).Padding(10, 14).Radius(10).
				MaxWidth(max(0, width-32)).Background(p.surface).Border(1, p.hairline).
				Shadow(0, 4, 18, -4, p.cardShade).Children(func() {
				ui.Box(c).Size(6, 6).Shrink(0).Radius(3).Background(p.accent)
				ui.Column(c).Shrink(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, notice.Title).FontSize(12.5).FontWeight(500).TextColor(p.text)
					if notice.Description != "" {
						ui.Text(c, notice.Description).FontSize(12).TextColor(p.muted)
					}
				})
				if notice.Action != "" {
					toast.ActionButton().Padding(4, 8).Radius(6).TextColor(p.accent).Children(func() {
						ui.Text(c, notice.Action).FontSize(12).FontWeight(600)
					})
				}
			})
		}
	})
}
