package player

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const Bands = 24

// spectrumFilter keeps the original stereo channels untouched. The other 24
// channels only measure logarithmic frequency bands. astats attaches their RMS
// to frame metadata, then pan removes the analysis channels before output.
// Unlike a second decoder, this measures the very stream being heard.
func spectrumFilter() string {
	channels := strings.Fields("FL FR FC LFE BL BR FLC FRC BC SL SR TC TFL TFC TFR TBL TBC TBR LFE2 TSL TSR BFC BFL BFR DL DR")
	var g strings.Builder
	g.WriteString("aformat=sample_rates=44100:channel_layouts=stereo,asplit=2[dry][s];[s]pan=mono|c0=0.5*c0+0.5*c1,asplit=24")
	for b := range Bands {
		fmt.Fprintf(&g, "[s%d]", b)
	}
	g.WriteByte(';')
	for b := range Bands {
		lo := 40 * math.Pow(350, float64(b)/Bands)
		hi := 40 * math.Pow(350, float64(b+1)/Bands)
		fmt.Fprintf(&g, "[s%d]bandpass=f=%.3f:t=h:w=%.3f[b%d];", b, math.Sqrt(lo*hi), hi-lo, b)
	}
	g.WriteString("[dry]")
	for b := range Bands {
		fmt.Fprintf(&g, "[b%d]", b)
	}
	fmt.Fprintf(&g, "join=inputs=25:channel_layout=%s:map=0.0-FL|0.1-FR", strings.Join(channels, "+"))
	for b, c := range channels[2:] {
		fmt.Fprintf(&g, "|%d.0-%s", b+1, c)
	}
	g.WriteString(",astats=metadata=1:reset=1:measure_perchannel=RMS_level:measure_overall=none,pan=stereo|c0=c0|c1=c1")
	return "@spectrum:lavfi=[" + g.String() + "]"
}

func spectrumLevel(raw string, band int) (float32, bool) {
	db, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(db) || math.IsInf(db, 1) {
		return 0, false
	}
	db += 20 * math.Log10(1+float64(band)/6)
	return float32(math.Max(0, math.Min(1, (db+60)/52))), true
}
