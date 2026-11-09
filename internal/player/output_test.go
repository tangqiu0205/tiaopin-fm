package player

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func requireMPV(t *testing.T) {
	t.Helper()
	if os.Getenv("TIAOPIN_TEST_MPV") == "" {
		t.Skip("set TIAOPIN_TEST_MPV=1 and TIAOPIN_MPV_LIBRARY to test native libmpv")
	}
}

// Real native decoding and filter metadata, without an audio device or network.
func TestMPVFormatsAndSpectrum(t *testing.T) {
	requireMPV(t)
	for _, file := range []string{
		"seek.mp3", "seek.flac", "aac/lc-mono-32000.aac", "aac/lc-stereo-48000.aac",
		"ogg/vorbis-mono-32000.ogg", "ogg/vorbis-stereo-48000.ogg",
		"ogg/opus-mono-24000.ogg", "ogg/opus-stereo-48000.ogg",
		"hls/adts-aac.aac", "hls/ts-aac.ts",
	} {
		t.Run(file, func(t *testing.T) {
			source, err := filepath.Abs(filepath.Join("testdata", file))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			out, err := newOutputWithOptions(ctx, source, map[string]string{"ao": "null", "loop-file": "inf"})
			if err != nil {
				t.Fatal(err)
			}
			defer out.Close()
			out.Play()
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				levels, ok := out.Levels()
				if ok {
					var sum float32
					for _, level := range levels {
						if math.IsNaN(float64(level)) || level < 0 || level > 1 {
							t.Fatal("invalid spectrum", levels)
						}
						sum += level
					}
					if sum > 0 {
						return
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Fatalf("mpv did not expose spectrum for decoded audio: %v", out.Err())
		})
	}
}

func TestMPVHLS(t *testing.T) {
	requireMPV(t)
	data, err := os.ReadFile("testdata/hls/ts-aac.ts")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/live.m3u8" {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
			_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-TARGETDURATION:10\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:10,\nsegment.ts\n#EXT-X-ENDLIST\n"))
			return
		}
		w.Header().Set("Content-Type", "video/mp2t")
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := newOutputWithOptions(ctx, srv.URL+"/live.m3u8", map[string]string{"ao": "null"})
	if err != nil {
		t.Fatal(err)
	}
	out.Close()
}

func TestMPVPauseVolumeAndClose(t *testing.T) {
	requireMPV(t)
	source, _ := filepath.Abs("testdata/seek.mp3")
	out, err := newOutputWithOptions(context.Background(), source, map[string]string{"ao": "null"})
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	out.SetVolume(0.25)
	if got := out.inst.Get("volume"); got != "25.000000" && got != "25" {
		t.Fatalf("volume=%q", got)
	}
	out.Play()
	out.Pause()
	if out.Playing() || out.inst.Get("pause") != "yes" {
		t.Fatal("pause was not applied")
	}
	out.Close()
	out.Close()
}

func TestSpectrumLevel(t *testing.T) {
	for _, raw := range []string{"", "NaN", "+Inf", "garbage"} {
		if _, ok := spectrumLevel(raw, 0); ok {
			t.Fatalf("accepted %q", raw)
		}
	}
	if v, ok := spectrumLevel("-inf", 0); !ok || v != 0 {
		t.Fatal("silence must be zero", v, ok)
	}
}

func TestCancelledOutputDoesNotLoadMPV(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newOutput(ctx, "invalid")
	if err != context.Canceled {
		t.Fatal(err)
	}
}
