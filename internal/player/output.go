package player

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/player/mpv"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

// An output owns one libmpv instance and its event poller. No callback runs
// under its mutex, and Close waits for the poller before destroying the handle.
type output struct {
	inst       *mpv.Instance
	mu         sync.Mutex
	playing    bool
	ended      bool
	err        error
	onEnd      func()
	levels     [Bands]float32
	haveLevels bool
	stop       chan struct{}
	done       chan struct{}
	once       sync.Once
}

var openOutput = newOutput

func newOutput(ctx context.Context, source string) (*output, error) {
	return newOutputWithOptions(ctx, source, nil)
}

func newOutputWithOptions(ctx context.Context, source string, extra map[string]string) (*output, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts := map[string]string{
		"volume": "0", "af": spectrumFilter(), "user-agent": radio.UserAgent,
		"audio-buffer": "0.2", "cache-secs": "2",
	}
	for k, v := range extra {
		opts[k] = v
	}
	inst, err := mpv.New(opts)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			inst.Close()
		}
	}()
	if err = inst.Command("loadfile", source, "replace"); err != nil {
		return nil, err
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		for e := inst.Poll(); e.ID != 0; e = inst.Poll() {
			switch e.ID {
			case mpv.EventPlaybackRestart:
				if err = ctx.Err(); err != nil {
					return nil, err
				}
				if err = inst.Set("pause", "yes"); err != nil {
					return nil, err
				}
				out := &output{inst: inst, stop: make(chan struct{}), done: make(chan struct{})}
				ok = true
				go out.run()
				return out, nil
			case mpv.EventEndFile:
				if e.Err != nil {
					return nil, e.Err
				}
				if e.Reason != mpv.ReasonEOF {
					continue
				} // playlists can redirect
				return nil, errors.New("电台没有可播放的音频")
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return nil, errors.New("电台连接或解码超时")
		case <-ticker.C:
		}
	}
}

func (o *output) run() {
	defer close(o.done)
	tick := time.NewTicker(40 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-o.stop:
			return
		case <-tick.C:
		}
		for e := o.inst.Poll(); e.ID != 0; e = o.inst.Poll() {
			if e.ID == mpv.EventEndFile && (e.Reason == mpv.ReasonEOF || e.Reason == mpv.ReasonError) {
				err := e.Err
				if err == nil && e.Reason == mpv.ReasonError {
					err = errors.New("电台音频加载或解码失败")
				}
				o.finish(err)
				return
			}
		}
		metadata, err := o.inst.Metadata("af-metadata/spectrum")
		if err != nil {
			o.finish(err)
			return
		}
		var levels [Bands]float32
		have := true
		for b := range Bands {
			raw := metadata[fmt.Sprintf("lavfi.astats.%d.RMS_level", b+3)]
			v, ok := spectrumLevel(raw, b)
			levels[b] = v
			have = have && ok
		}
		o.mu.Lock()
		o.levels, o.haveLevels = levels, have
		o.mu.Unlock()
	}
}

func (o *output) finish(err error) {
	o.mu.Lock()
	if o.ended {
		o.mu.Unlock()
		return
	}
	o.ended, o.playing, o.err = true, false, err
	fn := o.onEnd
	o.mu.Unlock()
	if fn != nil {
		fn()
	}
}
func (o *output) OnEnd(fn func()) {
	o.mu.Lock()
	o.onEnd = fn
	ended := o.ended
	o.mu.Unlock()
	if ended && fn != nil {
		fn()
	}
}
func (o *output) Play()  { o.setPaused(false) }
func (o *output) Pause() { o.setPaused(true) }
func (o *output) setPaused(paused bool) {
	value := "no"
	if paused {
		value = "yes"
	}
	if err := o.inst.Set("pause", value); err != nil {
		o.finish(err)
		return
	}
	o.mu.Lock()
	o.playing = !paused && !o.ended
	o.mu.Unlock()
}
func (o *output) Playing() bool { o.mu.Lock(); defer o.mu.Unlock(); return o.playing }
func (o *output) SetVolume(v float64) {
	if err := o.inst.Set("volume", strconv.FormatFloat(math.Max(0, math.Min(1, v))*100, 'f', 3, 64)); err != nil {
		o.finish(err)
	}
}
func (o *output) Err() error { o.mu.Lock(); defer o.mu.Unlock(); return o.err }
func (o *output) Levels() ([Bands]float32, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.levels, o.haveLevels && !o.ended
}
func (o *output) Close() { o.once.Do(func() { close(o.stop); <-o.done; o.inst.Close() }) }
