package app

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

func (a *app) requestStations(offset int) {
	ctx, cancel := context.WithCancel(context.Background())
	a.radio.cancel, a.radio.loading = cancel, true
	key, generation, filter := a.radio.asked, a.radio.generation, a.stationFilter()
	find := findStations
	go func() {
		defer cancel()
		page, err := find(ctx, filter, stationLimit, offset)
		if err == nil && page.HasMore && page.NextOffset <= offset {
			err = fmt.Errorf("radio: continuation did not advance past %d", offset)
		}
		a.post(func() { a.receiveStations(key, generation, offset, page, err, time.Now()) })
	}()
}

func (a *app) loadMoreStations() {
	if a.page != pageRadio || a.quitting || a.radio.cacheLoading || a.radio.loading ||
		a.radio.stale || a.radio.err != "" || a.radio.moreErr != "" || !a.radio.hasMore ||
		a.radio.asked != a.stationKey() {
		return
	}
	a.requestStations(a.radio.nextOffset)
}

func (a *app) stationListFooter(c *ui.Context) {
	ui.Row(c.Key("station-list-footer")).Height(48).Gap(12).Center().Children(func() {
		switch {
		case a.radio.loading:
			ui.Text(c, "正在加载更多电台…").FontSize(12.5).TextColor(a.pal.muted)
		case a.radio.moreErr != "":
			ui.Text(c, a.radio.moreErr).FontSize(12.5).TextColor(a.pal.muted)
			ui.Button(c.Key("retry-more"), "重试加载更多").OnClick(func() {
				a.radio.moreErr = ""
				a.loadMoreStations()
			})
		case !a.radio.hasMore:
			ui.Text(c, "已全部加载").FontSize(12.5).TextColor(a.pal.faint)
		default:
			ui.Text(c, "继续向下浏览，加载更多电台").FontSize(12.5).TextColor(a.pal.faint)
		}
	})
}

// Keep already displayed rows in place when popularity shifts between requests.
func appendUniqueStations(list, incoming []radio.Station) []radio.Station {
	seen := make(map[string]bool, len(list)+len(incoming))
	key := func(st radio.Station) string {
		if st.ID != "" {
			return "id:" + st.ID
		}
		return "stream:" + st.Stream
	}
	for _, st := range list {
		seen[key(st)] = true
	}
	for _, st := range incoming {
		id := key(st)
		if !seen[id] {
			seen[id] = true
			list = append(list, st)
		}
	}
	return list
}
