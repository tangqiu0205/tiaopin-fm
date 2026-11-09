package app

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestVolumePercentageVisibleAndStable(t *testing.T) {
	for _, width := range []int{1000, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp()
			tt := ui.NewTester(a.view, width, 580)
			var previous ui.Rect
			for i, volume := range []float64{0, 0.8, 1} {
				a.volume = volume
				tt.Frame()
				label := fmt.Sprintf("%d%%", int(volume*100+0.5))
				r, ok := tt.Find(label)
				slider, _ := tt.Find("音量")
				if !ok || r.X < slider.X+slider.W || r.X+r.W > float32(width) {
					t.Fatalf("percentage %q missing or clipped: %+v", label, r)
				}
				if i > 0 && r != previous {
					t.Fatalf("percentage changes shifted volume layout: %+v -> %+v", previous, r)
				}
				previous = r
			}
		})
	}
}

func TestVolumePercentageTracksInput(t *testing.T) {
	a := newTestApp()
	a.volume = 0.5
	tt := ui.NewTester(a.view, 1200, 580)
	tt.Frame()
	slider, _ := tt.Find("音量")
	tt.ClickAt(slider.X+slider.W*0.75, slider.Y+slider.H/2)
	before := a.volume
	tt.Scroll(slider.X+slider.W/2, slider.Y+slider.H/2, 0, -40)
	tt.Frame()
	if a.volume <= before {
		t.Fatal("scrolling over the horizontal slider did not increase volume")
	}
	if !tt.HasText(fmt.Sprintf("%d%%", int(a.volume*100+0.5))) {
		t.Fatal("percentage did not follow volume input")
	}
	if err := tt.Click("静音"); err != nil {
		t.Fatal(err)
	}
	if a.volume != 0 || !tt.HasText("0%") {
		t.Fatal("mute did not update the percentage")
	}
}

func TestNarrowVolumePopoverShowsPercentage(t *testing.T) {
	a := newTestApp()
	a.volume = 0.8
	tt := ui.NewTester(a.view, 880, 580)
	tt.Frame()
	if err := tt.Click("音量"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("80%") {
		t.Fatal("narrow-window volume popover lost its percentage")
	}
}
