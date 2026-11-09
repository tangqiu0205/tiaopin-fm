package player

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alexgorbatchev/goshazam"
	shazamaudio "github.com/alexgorbatchev/goshazam/pkg/audio"

	"io.github.tangqiu0205/tiaopin-fm/internal/player/mpv"
)

const recognitionRate = 16000
const recognitionSeconds = 10

// sampleStation uses a separate, silent mpv instance. PCM is bounded by length,
// the context and the read size, never retained after the request. Fast PCM
// output decodes without waiting for an audio device or wall-clock playback.
func sampleStation(ctx context.Context, source string) ([]int16, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	file, err := os.CreateTemp("", "tiaopin-recognition-*.pcm")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return nil, err
	}
	defer os.Remove(path)
	inst, err := mpv.New(map[string]string{
		"ao": "pcm", "ao-pcm-file": path, "ao-pcm-waveheader": "no",
		"audio-format": "s16", "audio-samplerate": "16000", "audio-channels": "mono",
		// Resampling can leave a nominal 10-second cut one sample short.
		// Decode a little extra; the bounded read below still takes exactly 10 seconds.
		"length": fmt.Sprint(recognitionSeconds + 1), "gapless-audio": "no",
	})
	if err != nil {
		return nil, err
	}
	defer inst.Close()
	if err := inst.Command("loadfile", source, "replace"); err != nil {
		return nil, err
	}
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		for e := inst.Poll(); e.ID != 0; e = inst.Poll() {
			if e.ID != mpv.EventEndFile {
				continue
			}
			if e.Err != nil {
				return nil, e.Err
			}
			if e.Reason != mpv.ReasonEOF {
				continue
			}
			// Closing flushes the PCM writer before reading it, also on Windows.
			inst.Close()
			f, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			const size = recognitionRate * recognitionSeconds * 2
			buf := make([]byte, size)
			n, err := io.ReadFull(f, buf)
			if err != nil {
				return nil, fmt.Errorf("电台采样不足 %d 秒（得到 %d/%d 字节）：%w", recognitionSeconds, n, size, err)
			}
			samples := make([]int16, n/2)
			for i := range samples {
				samples[i] = int16(binary.LittleEndian.Uint16(buf[i*2:]))
			}
			return samples, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tick.C:
		}
	}
}

// ErrNoSong means Shazam did not match the sample to a song.
var ErrNoSong = errors.New("没有识别到歌曲")

// Song is the title and artist identified from a station's audio.
type Song struct {
	ID     string
	Title  string
	Artist string
	Cover  string
}

// Recognize connects separately so sampling never interrupts playback. Only
// the generated fingerprint, not the audio, is sent to Shazam.
func Recognize(ctx context.Context, track Track) (Song, error) {
	return recognize(ctx, track, goshazam.New(goshazam.WithTimeout(15*time.Second)))
}

func recognize(ctx context.Context, track Track, client *goshazam.Shazam) (Song, error) {
	if err := ctx.Err(); err != nil {
		return Song{}, err
	}
	samples, err := sampleStation(ctx, track.Stream)
	if err != nil {
		return Song{}, fmt.Errorf("电台采样失败：%w", err)
	}
	if err := ctx.Err(); err != nil {
		return Song{}, err
	}
	result, err := client.RecognizeAudio(ctx, shazamaudio.NewSegment(samples, recognitionRate, 1))
	if err != nil {
		return Song{}, fmt.Errorf("Shazam 识曲请求失败：%w", err)
	}
	if result.Track == nil || len(result.Matches) == 0 || result.Track.Title == "" {
		return Song{}, ErrNoSong
	}
	return Song{ID: result.Track.Key, Title: result.Track.Title, Artist: result.Track.Subtitle, Cover: result.Track.PhotoURL}, nil
}
