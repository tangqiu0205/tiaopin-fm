package app

import (
	"context"
	"errors"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

// app is the state the window shows. Everything in it is read and changed
// on the main thread: by the views, and by goroutines through post.
// appTitle is what the window, the sidebar and the menus call the app: the
// project is TiaopinFM, and listeners know it by what it does.
const appTitle = "调频 FM"

type app struct {
	win   *mygo.Window
	notch *notch
	tray  *mygo.Tray // the icon in the menu bar or notification area, while it shows
	art   *artCache
	pl    *player.Player
	pal   palette

	toast string // a message to show once

	recognizing            bool
	recognizedTrack        *player.Track
	recognitionText        string
	recognitionSong        player.Song
	recognitionCancel      context.CancelFunc
	recognitionTimerCancel func()
	recognitionGen         int

	radio      radioState // the stations: what the page lists, and what it asked for
	stationBox ui.Handle  // the field searching the stations page
	lib        *library   // the stations the listener kept, and played lately
	page       pageRef    // which of the pages the sidebar shows

	volume     float64
	volumeOpen bool // the volume popover of the player bar shows
	mutedFrom  float64

	sleepOpen     bool
	sleepCustom   string
	sleepError    string
	sleepDeadline time.Time
	sleepStop     func()
	sleepGen      int

	rowBars    barState
	navScroll  ui.ScrollState
	listScroll ui.ListState
	// underTop says whether content has scrolled beneath the title bar.
	underTop bool

	sig      playerSig
	quitting bool

	stationDisk *stationCacheDisk
	settings    settings

	restored     bool   // the station picked up where it was left
	savedSession string // the session as last kept
	sessionDue   bool   // a save of the session is due
}

// post runs fn on the main thread and redraws the window. Without a
// window, as in tests, it drops fn.
func (a *app) post(fn func()) {
	if a.win == nil {
		return
	}
	a.win.Update(func() {
		fn()
		a.redraw()
	})
}

func (a *app) redraw() {
	if a.win != nil {
		a.win.Invalidate()
	}
	if a.notch != nil && a.notch.win != nil {
		a.notch.win.Invalidate()
	}
}

func (a *app) showMain() {
	if a.win != nil {
		if a.win.IsMinimized() {
			a.win.Restore()
		}
		a.win.Show()
		a.win.Focus()
		// From the menu bar alone the app may be inactive.
		mygo.App.Focus()
	}
}

// Run runs the app until it quits.
func Run() error {
	portable, err := configurePortable()
	if err != nil {
		mygo.App.WhenReady(func() {
			mygo.Dialog.Error("无法启动调频 FM", err.Error())
			mygo.App.Quit()
		})
		return errors.Join(err, mygo.App.Run())
	}
	mygo.Use(native.New(updater.Options{Language: "zh-CN"}))
	a := &app{}
	if !portable {
		moveOldData()
	}
	mygo.App.WhenReady(func() {
		a.settings = loadSettings()
		a.restoreRadioFilters()
		a.settings.applyAppearance()
		a.lib = loadLibrary()
		// Before the window: on Windows it lays out its content as it is
		// made, and the view asks the player what plays.
		a.art = newArtCache(a.post)
		a.pl = player.New(a.post)
		a.pl.Changed = a.keepSession
		a.pl.StreamRefreshed = a.stationStreamRefreshed
		a.volume = a.settings.Volume
		a.pl.SetVolume(a.volume)
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         appTitle,
			Width:         1180,
			Height:        780,
			MinWidth:      880,
			MinHeight:     580,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			// As tall as the edge over a page (Linux, Windows).
			TitleBarHeight: buttonsHeight,
			// Inset from the corner, within the sidebar, as macOS 26's
			// apps with a sidebar place them.
			TrafficLightPosition: &mygo.Point{X: 20, Y: 20},
			Vibrancy:             mygo.VibrancySidebar,
			BackgroundColor:      "light-dark(#ffffff, #141416)",
			Content:              ui.View(a.view),
		})
		a.setMenu()
		a.restoreSession()
		a.loadStationCache()
		// The notch player is macOS's, over its menu bar: elsewhere it would
		// hang at the top of the screen as a black box.
		if runtime.GOOS == "darwin" {
			a.notch = newNotch(a)
		}
		// From the menu bar alone, the Dock icon shows while the window
		// does.
		a.syncDock()
		a.win.OnShow(a.syncDock)
		a.win.OnHide(a.syncDock)
		a.applyTray()
		// Closing the window hides it: the station plays on, and the Dock
		// icon or the tray's brings it back. A Linux desktop without a
		// tray has neither, so there closing it quits.
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if a.quitting {
				return
			}
			e.PreventDefault()
			if runtime.GOOS == "darwin" || a.tray != nil {
				a.win.Hide()
			} else {
				mygo.App.Quit()
			}
		})
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		if a.radio.cancel != nil {
			a.radio.cancel()
		}
		a.cancelSleepTimer()
		a.settings.Volume = a.volume
		a.settings.save()
		a.saveSession()
		if a.pl != nil {
			a.pl.Close()
		}
		if a.recognitionCancel != nil {
			a.recognitionCancel()
		}
		a.flushStationCache()
	})
	// Windows leaves an icon in the notification area after its app until
	// the pointer passes over it.
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		if a.tray != nil {
			a.tray.Destroy()
		}
	})
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(bool) { a.showMain() })
	return mygo.App.Run()
}
