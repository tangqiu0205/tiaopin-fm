// Command icon generates the app and tray icons. Run from the repository root:
//
//	go run ./icon
//
// The Icon Composer documents use the same glyph as the PNGs.
package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

// Match iconRadio in internal/app/icons.go: a ring and two pairs of arcs.
func glyph(x, y float64) bool {
	x, y = (x-512)/28, (y-512)/28
	d := math.Hypot(x, y)
	if math.Abs(d-2) <= 0.875 {
		return true
	}
	for _, radius := range []float64{6, 10} {
		// Reflect both sides onto the right-hand arc and clamp to its ends.
		angle := math.Max(-math.Pi/4, math.Min(math.Pi/4, math.Atan2(y, math.Abs(x))))
		if math.Hypot(math.Abs(x)-radius*math.Cos(angle), y-radius*math.Sin(angle)) <= 0.875 {
			return true
		}
	}
	return false
}

func svg() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024">
<g transform="translate(176 176) scale(28)" fill="none" stroke="#FFFFFF" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">
<circle cx="12" cy="12" r="2"/><path d="M16.24 7.76a6 6 0 0 1 0 8.49"/><path d="M7.76 16.24a6 6 0 0 1 0-8.49"/><path d="M19.07 4.93a10 10 0 0 1 0 14.14"/><path d="M4.93 19.07a10 10 0 0 1 0-14.14"/>
</g>
</svg>
`
}

func rounded(x, y float64) bool {
	// The app body occupies the standard 824-point macOS icon grid.
	dx := math.Max(math.Abs(x-512)-262, 0)
	dy := math.Max(math.Abs(y-512)-262, 0)
	return math.Hypot(dx, dy) <= 150
}

func render(size int, template bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const samples = 4
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/samples) * 1024 / float64(size)
					py := (float64(y) + (float64(sy)+0.5)/samples) * 1024 / float64(size)
					if template {
						// Expand the glyph for the 18-point menu bar.
						if glyph((px-512)*0.75+512, (py-512)*0.75+512) {
							a++
						}
						continue
					}
					if !rounded(px, py) {
						continue
					}
					t := math.Max(0, math.Min(1, (py-100)/824))
					cr, cg, cb := 239-35*t, 58-28*t, 73-22*t
					if glyph(px, py) {
						cr, cg, cb = 255, 255, 255
					}
					r += cr
					g += cg
					b += cb
					a++
				}
			}
			if a > 0 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(r / a), uint8(g / a), uint8(b / a), uint8(math.Round(a * 255 / (samples * samples)))})
			}
		}
	}
	return img
}

func save(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err = png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func main() {
	for _, item := range []struct {
		path     string
		size     int
		template bool
	}{
		{"resources/icon.png", 1024, false},
		{"resources/icon-dev.png", 1024, false},
		{"internal/app/tray/tray.png", 64, false},
		{"internal/app/tray/tray-template.png", 36, true},
	} {
		if err := save(item.path, render(item.size, item.template)); err != nil {
			log.Fatal(err)
		}
	}
	for _, dir := range []string{"icon/AppIcon.icon", "icon/AppIcon-dev.icon"} {
		if err := os.WriteFile(filepath.Join(dir, "Assets", "signal.svg"), []byte(svg()), 0644); err != nil {
			log.Fatal(err)
		}
		top, bottom := "srgb:0.93725,0.22745,0.28627,1.00000", "srgb:0.80000,0.11765,0.20000,1.00000"
		doc := map[string]any{
			"groups":               []any{map[string]any{"layers": []any{map[string]any{"image-name": "signal.svg", "name": "signal", "glass": true}}}},
			"supported-platforms":  map[string]any{"circles": []string{"watchOS"}, "squares": "shared"},
			"fill-specializations": []any{map[string]any{"value": map[string]any{"linear-gradient": []string{top, bottom}, "orientation": map[string]any{"start": map[string]float64{"x": .5, "y": 0}, "stop": map[string]float64{"x": .5, "y": 1}}}}},
		}
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "icon.json"), append(data, '\n'), 0644); err != nil {
			log.Fatal(err)
		}
	}
}
