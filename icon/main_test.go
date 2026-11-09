package main

import "testing"

func TestAppIconUsesMoreCanvasWithoutChangingTray(t *testing.T) {
	bounds := func(app bool) (int, int) {
		img := render(128, false, app)
		min, max := 128, -1
		for y := 0; y < 128; y++ {
			for x := 0; x < 128; x++ {
				if img.NRGBAAt(x, y).A > 127 {
					if x < min {
						min = x
					}
					if x > max {
						max = x
					}
				}
			}
		}
		return min, max
	}
	appMin, appMax := bounds(true)
	trayMin, trayMax := bounds(false)
	if appMin >= trayMin || appMax <= trayMax || appMin != 127-appMax {
		t.Fatalf("app icon not enlarged and centered: app=%d..%d tray=%d..%d", appMin, appMax, trayMin, trayMax)
	}
}
