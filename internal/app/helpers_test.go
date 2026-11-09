package app

import (
	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

func newTestApp() *app {
	a := &app{}
	a.art = newArtCache(func(func()) {})
	a.pl = player.New(func(func()) {})
	a.pl.SetVolume(0.5)
	// A library that is never saved: no path, as one loaded outside a run.
	a.lib = &library{}
	return a
}
