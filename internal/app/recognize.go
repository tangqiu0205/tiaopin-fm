package app

import (
	"context"
	"errors"
	"log"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/player"
)

var recognizeSong = player.Recognize

const recognitionResultDuration = 8 * time.Second

// recognitionAfter posts timer callbacks back to the main thread.
var recognitionAfter = func(a *app, delay time.Duration, fn func()) func() {
	timer := time.AfterFunc(delay, func() { a.post(fn) })
	return func() { timer.Stop() }
}

func (a *app) expireRecognitionLater() {
	gen := a.recognitionGen
	if a.recognitionTimerCancel != nil {
		a.recognitionTimerCancel()
	}
	a.recognitionTimerCancel = recognitionAfter(a, recognitionResultDuration, func() {
		if gen == a.recognitionGen && !a.recognizing && !a.quitting {
			a.dismissRecognition()
		}
	})
}

func (a *app) dismissRecognition() {
	if a.recognitionTimerCancel != nil {
		a.recognitionTimerCancel()
		a.recognitionTimerCancel = nil
	}
	if a.recognitionCancel != nil {
		a.recognitionCancel()
		a.recognitionCancel = nil
	}
	a.recognitionGen++
	a.recognizing = false
	a.recognizedTrack = nil
	a.recognitionSong = player.Song{}
	a.recognitionText = ""
}

func (a *app) recognizeCurrent() {
	track := a.pl.Current()
	if track == nil || (a.recognizing && a.recognizedTrack == track) {
		return
	}
	a.dismissRecognition()
	a.recognizedTrack = track
	if !a.pl.Playing() || !a.pl.Ready() {
		a.recognitionText = "请先播放电台再识曲"
		a.expireRecognitionLater()
		return
	}
	a.recognizing = true
	a.recognitionText = "正在识别歌曲…"
	station, gen := *track, a.recognitionGen
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	a.recognitionCancel = cancel
	go func() {
		defer cancel()
		song, err := recognizeSong(ctx, station)
		a.post(func() { a.finishRecognition(track, station.Name, gen, song, err) })
	}()
}

func (a *app) finishRecognition(track *player.Track, station string, gen int, song player.Song, err error) {
	if gen != a.recognitionGen || a.quitting {
		return
	}
	a.recognizing = false
	a.recognitionCancel = nil
	if a.pl.Current() != track {
		a.dismissRecognition()
		return
	}
	if errors.Is(err, player.ErrNoSong) {
		a.recognitionText = "未识别到歌曲，可能是讲话或广告"
		a.expireRecognitionLater()
		return
	}
	if err != nil {
		log.Printf("recognize station: %v", err)
		a.recognitionText = "识曲失败，请稍后再试"
		a.expireRecognitionLater()
		return
	}
	a.recognitionText = ""
	a.recognitionSong = song
	a.expireRecognitionLater()
	if err := a.lib.addRecognition(song, station, time.Now()); err != nil {
		log.Printf("save recognition history: %v", err)
		a.toast = "无法保存识曲历史：" + err.Error()
	}
}
