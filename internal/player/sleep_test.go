package player

import "testing"

// Pausing during a reconnect must not be undone when the queued retry runs.
func TestPauseDuringReconnect(t *testing.T) {
	p := New(func(func()) {})
	p.Restore(Session{Stations: []Track{{ID: 1, Name: "Station", Stream: "http://127.0.0.1:1/stream"}}, At: 0})
	p.playing = true
	p.Pause()
	p.retry(0)
	if p.playing || !p.loading {
		t.Fatalf("retry after pause: playing=%v loading=%v", p.playing, p.loading)
	}
}
