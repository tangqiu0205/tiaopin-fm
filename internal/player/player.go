package player

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/egoist/mygo"
)

// Player plays a station, live: what plays goes on until the listener
// plays another, and a stream that drops is opened again. Its methods run
// on the main thread; loading happens on goroutines, whose results post
// brings back there.
type Player struct {
	post func(func())
	// Changed, if set, hears that what plays, or whether it plays,
	// changed.
	Changed func()
	// StreamRefreshed runs on the main thread after a failed connection
	// refreshed its directory URL, so callers can repair persisted copies.
	StreamRefreshed func(Track)

	queue  []Track
	at     int // where in the queue the station is, -1 for none
	volume float64

	track   *Track
	out     *output
	cancel  context.CancelFunc
	fade    fader
	loading bool
	playing bool // what the listener wants: false while paused
	err     string
	gen     int // counts loads, to drop the results of stale ones

	// A station's stream goes on until it drops, and is opened again when
	// it does: liveTries counts the tries since it last played a while,
	// and liveSince is when it last opened.
	liveTries int
	liveSince time.Time
}

func New(post func(func())) *Player {
	p := &Player{post: post, at: -1, volume: 0.8}
	// The media keys, and the controls of Control Center.
	mygo.NowPlaying.OnCommand(func(cmd mygo.MediaCommand, _ time.Duration) {
		switch cmd {
		case mygo.MediaPlay, mygo.MediaPause:
			if (cmd == mygo.MediaPlay) != p.playing {
				p.TogglePlay()
			}
		case mygo.MediaNext:
			p.Next()
		case mygo.MediaPrevious:
			p.Prev()
		}
		p.post(func() {})
	})
	return p
}

// sync shows the system, and Changed, where playback is.
func (p *Player) sync() {
	if p.Changed != nil {
		defer p.Changed()
	}
	if p.track == nil {
		mygo.NowPlaying.Clear()
		return
	}
	state := mygo.PlaybackPaused
	if p.playing {
		state = mygo.PlaybackPlaying
	}
	mygo.NowPlaying.SetPlayback(state, 0)
}

// announce shows the system the station, and its logo once loaded.
func (p *Player) announce(t *Track, artwork []byte) {
	state := mygo.PlaybackPaused
	if p.playing {
		state = mygo.PlaybackPlaying
	}
	// A station has nowhere to seek to.
	mygo.NowPlaying.Set(mygo.NowPlayingInfo{
		Title:    t.Name,
		Artist:   t.Detail,
		Artwork:  artwork,
		Duration: 0,
		Position: 0,
		State:    state,
		Commands: mygo.MediaPlay | mygo.MediaPause | mygo.MediaNext | mygo.MediaPrevious,
	})
}

// Play replaces the queue with stations and plays the one at start.
func (p *Player) Play(stations []Track, start int) {
	if len(stations) == 0 {
		return
	}
	start = min(max(start, 0), len(stations)-1)
	p.queue = append([]Track(nil), stations...)
	p.playing = true
	p.load(start)
}

// Current returns the station playing or loading, or nil.
func (p *Player) Current() *Track { return p.track }

// Queue returns the stations queued, in the order they were given.
func (p *Player) Queue() []Track { return p.queue }

// Playing reports what the listener wants: false while paused.
func (p *Player) Playing() bool { return p.playing }

// Loading reports whether the station is still connecting.
func (p *Player) Loading() bool { return p.loading }

// Ready reports whether the station has connected, to play.
func (p *Player) Ready() bool { return p.out != nil }

// Err returns why the station did not play, or "".
func (p *Player) Err() string { return p.err }

// Live reports whether a station is what plays: always, since nothing
// else does.
func (p *Player) Live() bool { return p.track != nil }

// Levels returns the spectrum of what plays now.
func (p *Player) Levels() ([Bands]float32, bool) {
	if p.out == nil || !p.playing {
		return [Bands]float32{}, false
	}
	return p.out.Levels()
}

// Next plays the station after this one, going round at the end.
func (p *Player) Next() {
	if len(p.queue) == 0 {
		return
	}
	p.load((p.at + 1) % len(p.queue))
}

// Prev plays the station before this one, going round at the start.
func (p *Player) Prev() {
	if len(p.queue) == 0 {
		return
	}
	p.load((p.at - 1 + len(p.queue)) % len(p.queue))
}

// Jump plays the station at the queue's index i.
func (p *Player) Jump(i int) {
	if i < 0 || i >= len(p.queue) {
		return
	}
	p.load(i)
}

// TogglePlay pauses, fading out, or plays, fading in.
func (p *Player) TogglePlay() {
	if p.out == nil {
		switch {
		case p.loading:
			// It plays once connected, or not.
			p.playing = !p.playing
			p.sync()
		case p.at >= 0 && len(p.queue) > 0:
			// Nothing plays: connect with the listener's intent to play.
			p.playing = true
			p.load(p.at)
			p.sync()
		}
		return
	}
	out := p.out
	if p.playing {
		p.playing = false
		p.fade.fade(0, fadeOut, func() {
			p.post(func() {
				if p.out == out && !p.playing {
					out.Pause()
					p.sync()
				}
			})
		})
		p.sync()
		return
	}
	p.playing = true
	if !out.Playing() {
		out.Play()
	}
	p.fade.fade(1, fadeIn, nil)
	p.sync()
}

// Pause fades playback out, including while a connection is pending.
func (p *Player) Pause() {
	if !p.playing {
		return
	}
	if p.out == nil && !p.loading {
		p.playing = false
		p.sync()
		return
	}
	p.TogglePlay()
}

func (p *Player) SetVolume(v float64) {
	p.volume = min(1, max(0, v))
	p.applyVolume()
}

func (p *Player) Volume() float64 { return p.volume }

// applyVolume sets the volume of what plays: the listener's, which the ear
// hears in powers.
func (p *Player) applyVolume() {
	p.fade.setGain(p.volume * p.volume)
}

func (p *Player) stop() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.fade.attach(nil, 1)
	if out := p.out; out != nil {
		p.out = nil
		// Destruction can wait for native network threads; keep it off the UI.
		go out.Close()
	}
}

// load connects the station at the queue's index at, and plays it.
func (p *Player) load(at int) {
	p.stop()
	p.gen++
	gen := p.gen
	p.at = at
	t := p.queue[at]
	if p.track == nil || p.track.ID != t.ID {
		// Another station: what dropped before dropped long ago.
		p.liveTries = 0
	}
	p.track = &t
	p.loading, p.err = true, ""
	p.announce(&t, nil)

	if t.Cover != "" {
		go func() {
			cover, err := fetch(t.Cover)
			if err != nil {
				return
			}
			p.post(func() {
				if gen == p.gen {
					p.announce(&t, cover)
				}
			})
		}()
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	refresh := refreshLiveStation
	go func() {
		s, loaded, refreshed, err := openStation(ctx, t, refresh)
		p.post(func() {
			if gen != p.gen {
				if s != nil {
					go s.Close()
				}
				return
			}
			if refreshed {
				p.queue[at], p.track = loaded, &loaded
				if p.StreamRefreshed != nil {
					p.StreamRefreshed(loaded)
				}
			}
			p.loading = false
			if err != nil {
				// The listener chose this station: it stays, saying why it
				// cannot play, rather than the next one playing instead.
				p.err = err.Error()
				p.playing = false
				p.sync()
				return
			}
			p.out = s
			p.liveSince = time.Now()
			p.fade.attach(p.out, 1)
			p.applyVolume()
			p.out.OnEnd(func() {
				p.post(func() {
					if gen == p.gen {
						p.ended()
					}
				})
			})
			if p.playing {
				p.out.Play()
			}
			p.sync()
		})
	}()
}

func (p *Player) ended() {
	// A station's stream ended: it comes back, rather than the next one
	// playing instead.
	p.reconnect()
}

// maxLiveTries is how many times a station that drops is opened again
// before it is left as gone.
const maxLiveTries = 3

const (
	// liveSteady is how long a station plays before it counts as playing
	// again: one that dropped before was never really playing.
	liveSteady = 20 * time.Second
	// liveRetryGap lets a station that dropped come back before it is
	// asked again.
	liveRetryGap = time.Second
)

// reconnect opens the station playing again, after its stream dropped.
func (p *Player) reconnect() {
	p.liveTries++
	if time.Since(p.liveSince) > liveSteady {
		p.liveTries = 1
	}
	if p.liveTries > maxLiveTries {
		p.err = "电台已断开，稍后再试"
		p.playing = false
		// Nothing plays: pressing play opens the station again.
		p.liveTries = 0
		p.stop()
		p.sync()
		return
	}
	at, gen := p.at, p.gen
	p.stop()
	p.err = "正在重新连接…"
	p.sync()
	time.AfterFunc(liveRetryGap, func() {
		p.post(func() {
			if gen == p.gen {
				p.retry(at)
			}
		})
	})
}

func (p *Player) retry(at int) {
	if p.at != at || len(p.queue) == 0 {
		// Another station plays, or none.
		return
	}
	p.load(at)
}

// Close cancels pending loads and releases native playback resources.
func (p *Player) Close() {
	p.gen++
	p.stop()
	p.playing, p.loading = false, false
}

// fetch returns what url serves, for a station's logo.
func fetch(url string) ([]byte, error) {
	res, err := liveClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 4<<20))
}
