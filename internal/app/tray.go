package app

import (
	_ "embed"
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

// The app's icon in the menu bar on macOS, as the saved preference chooses,
// and always in the notification area on Windows and Linux.
// While it shows, closing the window leaves the music playing there, until
// the listener quits; the icon opens the window again.

// trayTemplate is the signal glyph, black on transparent for macOS to tint.
// It is generated at 2× the menu bar's 18-point size by icon/main.go.
//
//go:embed tray/tray-template.png
var trayTemplate []byte

// trayColor is the app's icon, for the notification area.
//
//go:embed tray/tray.png
var trayColor []byte

// showsTray tells whether the app's icon shows in the menu bar or the
// notification area.
func showsTray(s settings) bool {
	return runtime.GOOS != "darwin" || s.AppIcon != "dock"
}

// activationPolicy is macOS's for the settings: no Dock icon for the menu
// bar alone, unless a window is open, so the listener can switch to it
// with Command-Tab and the app's menu shows.
func activationPolicy(s settings, windowOpen bool) mygo.ActivationPolicy {
	if s.AppIcon == "menubar" && !windowOpen {
		return mygo.ActivationPolicyAccessory
	}
	return mygo.ActivationPolicyRegular
}

// windowOpen tells whether the main window is shown or minimized.
func (a *app) windowOpen() bool {
	return a.win != nil && (a.win.IsVisible() || a.win.IsMinimized())
}

// syncDock shows or hides the Dock icon, as the settings and the open
// windows say.
func (a *app) syncDock() {
	if runtime.GOOS == "darwin" {
		mygo.App.SetActivationPolicy(activationPolicy(a.settings, a.windowOpen()))
	}
}

// applyTray adds or removes the icon in the menu bar or notification area,
// as the settings say.
func (a *app) applyTray() {
	switch show := showsTray(a.settings); {
	case show && a.tray == nil:
		icon, template := trayColor, false
		if runtime.GOOS == "darwin" {
			icon, template = trayTemplate, true
		}
		t, err := mygo.NewTray(mygo.TrayOptions{ID: "tiaopin-fm", Icon: icon, IconIsTemplate: template, ToolTip: appTitle, Menu: a.trayMenu()})
		if err != nil {
			log.Println("tray:", err)
			return
		}
		// Windows' left click, where the right one opens the menu.
		t.OnClick(a.showMain)
		a.tray = t
	case !show && a.tray != nil:
		a.tray.Destroy()
		a.tray = nil
	}
}

// trayMenu is the icon's menu: the window, the player's controls,
// and quitting.
func (a *app) trayMenu() *mygo.Menu {
	run := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			fn()
			a.redraw()
		}
	}
	name := appTitle
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开" + name, Click: run(a.showMain)},
		mygo.Separator(),
		{Label: "播放/暂停", Click: run(func() { a.pl.TogglePlay() })},
		{Label: "下一个电台", Click: run(func() { a.pl.Next() })},
		{Label: "上一个电台", Click: run(func() { a.pl.Prev() })},
		mygo.Separator(),
		updateMenuItem(),
		{Label: "退出" + name, Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
}
