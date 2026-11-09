package app

import (
	"log"

	"github.com/egoist/mygo/ui"
)

var applyTheme = func(s settings) { s.applyAppearance() }

// themeButton cycles the saved appearance without opening a settings window.
func (a *app) themeButton(c *ui.Context) {
	icon, label := iconThemeSystem, "跟随系统"
	switch a.settings.Appearance {
	case "light":
		icon, label = iconThemeLight, "浅色"
	case "dark":
		icon, label = iconThemeDark, "深色"
	}
	iconButton(c.Key("theme"), icon, 16, a.pal.muted, "切换主题").
		Tooltip("主题：" + label + "，点击切换").OnClick(a.cycleAppearance)
}

func (a *app) cycleAppearance() {
	label := "跟随系统"
	switch a.settings.Appearance {
	case "system":
		a.settings.Appearance, label = "light", "浅色"
	case "light":
		a.settings.Appearance, label = "dark", "深色"
	default:
		a.settings.Appearance = "system"
	}
	applyTheme(a.settings)
	if err := a.settings.save(); err != nil {
		log.Printf("save appearance: %v", err)
		a.toast = "无法保存主题：" + err.Error()
	} else {
		a.toast = "主题：" + label
	}
	a.redraw()
}
