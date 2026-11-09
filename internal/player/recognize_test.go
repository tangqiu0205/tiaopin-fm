package player

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexgorbatchev/goshazam"
)

// Recognition samples a station and sends a fingerprint, not audio, to Shazam.
func TestRecognizeStation(t *testing.T) {
	requireMPV(t)
	data, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	station := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		for range 4 {
			if _, err := w.Write(data); err != nil {
				return
			}
		}
	}))
	defer station.Close()
	var signature string
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Signature struct {
				URI string `json:"uri"`
			} `json:"signature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
		}
		signature = payload.Signature.URI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"matches":[{"id":"1"}],"track":{"title":"Song","subtitle":"Artist","images":{"coverarthq":"https://example.com/cover.jpg"}}}`))
	}))
	defer service.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	song, err := recognize(ctx, Track{Stream: station.URL}, goshazam.New(goshazam.WithDiscoveryURL(service.URL)))
	if err != nil {
		t.Fatal(err)
	}
	if song.Title != "Song" || song.Artist != "Artist" || song.Cover != "https://example.com/cover.jpg" || !strings.HasPrefix(signature, "data:audio/vnd.shazam.sig;base64,") {
		t.Fatalf("got song %+v and signature prefix %.40q", song, signature)
	}
}

// A 48 kHz source resampled to 16 kHz can lose one sample at mpv's
// duration cutoff. Sampling must still return ten full seconds without padding.
func TestSampleStationResampled(t *testing.T) {
	requireMPV(t)
	const rate = 48000
	const seconds = 12
	const size = rate * seconds * 2
	data := make([]byte, 44+size)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], 36+size)
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], rate)
	binary.LittleEndian.PutUint32(data[28:], rate*2)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], size)
	path := filepath.Join(t.TempDir(), "sample.wav")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	samples, err := sampleStation(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != recognitionRate*recognitionSeconds {
		t.Fatalf("samples=%d", len(samples))
	}
}

// A genuinely short source must not be padded or sent to Shazam.
func TestSampleStationTooShort(t *testing.T) {
	requireMPV(t)
	_, err := sampleStation(context.Background(), "testdata/seek.mp3")
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v, want unexpected EOF", err)
	}
}

// Cancelling during a stalled network read must stop the native decoder and
// remove its temporary sample, not wait for mpv's network timeout.
func TestRecognizeCancelsPendingSample(t *testing.T) {
	requireMPV(t)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case started <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer func() { close(release); srv.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := sampleStation(ctx, srv.URL); result <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("sample did not connect")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("mpv sample did not cancel promptly")
	}
}

// A cancelled request must not open a station connection.
func TestRecognizeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Recognize(ctx, Track{Stream: ":invalid"})
	if err != context.Canceled {
		t.Fatalf("got %v, want context canceled", err)
	}
}
