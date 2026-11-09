package player

import (
	"context"
	"errors"
	"testing"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/player/mpv"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

func mockOpener(t *testing.T, fn func(context.Context, string) (*output, error)) {
	t.Helper()
	old := openOutput
	openOutput = fn
	t.Cleanup(func() { openOutput = old })
}

func TestStationURLRefreshRetriesOnlyOnce(t *testing.T) {
	attempts, lookups := 0, 0
	mockOpener(t, func(context.Context, string) (*output, error) { attempts++; return nil, errors.New("offline") })
	_, track, refreshed, err := openStation(context.Background(), Track{StationID: "a", Stream: "old"}, func(id string) (radio.Station, error) { lookups++; return radio.Station{ID: id, Stream: "new"}, nil })
	if err == nil || !refreshed || attempts != 2 || lookups != 1 || track.Stream != "new" {
		t.Fatalf("attempts=%d lookups=%d track=%+v refreshed=%v err=%v", attempts, lookups, track, refreshed, err)
	}
}

func TestWorkingStationDoesNotRefreshURL(t *testing.T) {
	mockOpener(t, func(context.Context, string) (*output, error) { return &output{}, nil })
	out, _, refreshed, err := openStation(context.Background(), Track{StationID: "a"}, func(string) (radio.Station, error) {
		t.Fatal("unexpected directory lookup")
		return radio.Station{}, nil
	})
	if out == nil || err != nil || refreshed {
		t.Fatalf("output=%v refreshed=%v err=%v", out, refreshed, err)
	}
}

func TestStationURLLookupFailureIsReported(t *testing.T) {
	mockOpener(t, func(context.Context, string) (*output, error) { return nil, errors.New("offline") })
	want := errors.New("directory offline")
	_, _, refreshed, err := openStation(context.Background(), Track{StationID: "a"}, func(string) (radio.Station, error) { return radio.Station{}, want })
	if !errors.Is(err, want) || refreshed {
		t.Fatalf("refreshed=%v err=%v", refreshed, err)
	}
}

func TestMissingMPVDoesNotRefreshDirectory(t *testing.T) {
	mockOpener(t, func(context.Context, string) (*output, error) { return nil, mpv.ErrUnavailable })
	_, _, _, err := openStation(context.Background(), Track{StationID: "a"}, func(string) (radio.Station, error) {
		t.Fatal("local install failure queried directory")
		return radio.Station{}, nil
	})
	if !errors.Is(err, mpv.ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestStationURLRefreshDoesNotReplaceNewSelection(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	mockOpener(t, func(ctx context.Context, source string) (*output, error) {
		if source == "old" {
			close(started)
			<-release
		}
		return nil, errors.New("offline")
	})
	posts := make(chan func(), 8)
	p := New(func(fn func()) { posts <- fn })
	defer p.Close()
	p.Play([]Track{{ID: 1, Stream: "old"}}, 0)
	<-started
	p.Play([]Track{{ID: 2, Stream: "new"}}, 0)
	close(release)
	next(t, posts)
	next(t, posts)
	if p.Current().ID != 2 || p.Queue()[0].ID != 2 {
		t.Fatal("stale load replaced new station")
	}
}

func TestReconnectTimerDoesNotReloadReselectedStation(t *testing.T) {
	posts := make(chan func(), 8)
	p := New(func(fn func()) { posts <- fn })
	p.queue = []Track{{ID: 1}}
	p.at = 0
	p.gen = 10
	p.liveSince = time.Now()
	p.reconnect()
	p.gen++ // a new selection, even if its queue index is also zero
	next(t, posts)
	if p.loading {
		t.Fatal("stale retry restarted a new selection")
	}
}

func next(t *testing.T, ch chan func()) {
	t.Helper()
	select {
	case fn := <-ch:
		fn()
	case <-time.After(5 * time.Second):
		t.Fatal("player did not post a result")
	}
}
