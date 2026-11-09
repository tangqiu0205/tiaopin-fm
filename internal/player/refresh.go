package player

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"io.github.tangqiu0205/tiaopin-fm/internal/player/mpv"
	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

var refreshLiveStation = radio.ByID
var liveClient = func() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 20 * time.Second
	return &http.Client{Transport: t}
}()

// Refresh at most once, and only after mpv actually fails to open the stream.
func openStation(ctx context.Context, t Track, refresh func(string) (radio.Station, error)) (*output, Track, bool, error) {
	out, err := openOutput(ctx, t.Stream)
	if err == nil || t.StationID == "" || ctx.Err() != nil || errors.Is(err, mpv.ErrUnavailable) {
		return out, t, false, err
	}
	st, refreshErr := refresh(t.StationID)
	if refreshErr != nil {
		return nil, t, false, fmt.Errorf("%v；更新电台地址失败：%w", err, refreshErr)
	}
	if st.ID != t.StationID || st.Stream == "" {
		return nil, t, false, errors.New("电台目录没有返回有效的新地址")
	}
	if err = ctx.Err(); err != nil {
		return nil, t, false, err
	}
	t.Stream = st.Stream
	out, err = openOutput(ctx, t.Stream)
	return out, t, true, err
}
