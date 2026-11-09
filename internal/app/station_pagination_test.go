package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"io.github.tangqiu0205/tiaopin-fm/internal/radio"
)

type stationRequest struct {
	ctx    context.Context
	filter radio.Filter
	offset int
}

func recordStationRequests(t *testing.T) <-chan stationRequest {
	t.Helper()
	requests := make(chan stationRequest, 16)
	old := findStations
	release := make(chan struct{})
	findStations = func(ctx context.Context, f radio.Filter, limit, offset int) (radio.Page, error) {
		if limit != stationLimit {
			t.Errorf("batch limit=%d", limit)
		}
		requests <- stationRequest{ctx, f, offset}
		select {
		case <-ctx.Done():
		case <-release:
		}
		return radio.Page{}, context.Canceled
	}
	t.Cleanup(func() {
		close(release)
		findStations = old
	})
	return requests
}

func nextStationRequest(t *testing.T, requests <-chan stationRequest, offset int) stationRequest {
	t.Helper()
	select {
	case req := <-requests:
		if req.offset != offset {
			t.Fatalf("offset=%d, want %d", req.offset, offset)
		}
		return req
	case <-time.After(5 * time.Second):
		t.Fatal("no directory request")
		return stationRequest{}
	}
}

func TestStationPaginationAppendRetryAndCache(t *testing.T) {
	requests := recordStationRequests(t)
	a := newTestApp()
	key := a.stationKey()
	a.radio.asked, a.radio.generation = key, 1
	a.receiveStations(key, 1, 0, radio.Page{Stations: []radio.Station{{ID: "a"}}, NextOffset: 60, HasMore: true}, nil, time.Now())
	a.loadMoreStations()
	a.loadMoreStations()
	nextStationRequest(t, requests, 60)
	a.receiveStations(key, 1, 60, radio.Page{Stations: []radio.Station{{ID: "a"}, {ID: "b"}, {ID: "b"}}, NextOffset: 120, HasMore: true}, nil, time.Now())
	if len(a.radio.stations) != 2 || a.radio.stations[0].ID != "a" || a.radio.stations[1].ID != "b" {
		t.Fatalf("append changed order or left duplicates: %v", a.radio.stations)
	}
	cached, _, _ := a.radio.cache.lookup(key, time.Now())
	if len(cached.Stations) != 1 || cached.NextOffset != 60 || !cached.HasMore {
		t.Fatalf("continuation overwrote first-page cache: %+v", cached)
	}
	a.loadMoreStations()
	nextStationRequest(t, requests, 120)
	a.receiveStations(key, 1, 120, radio.Page{}, errors.New("offline"), time.Now())
	if a.radio.moreErr == "" || a.radio.loading || a.radio.nextOffset != 120 || len(a.radio.stations) != 2 {
		t.Fatal("failed continuation lost data, advanced the cursor, or hid the error")
	}
	a.loadMoreStations()
	select {
	case <-requests:
		t.Fatal("loading twice or auto-retrying requested an extra batch")
	default:
	}
	a.radio.moreErr = "" // explicit retry
	a.loadMoreStations()
	nextStationRequest(t, requests, 120)
	a.receiveStations(key, 1, 120, radio.Page{NextOffset: 120}, nil, time.Now())
	if a.radio.hasMore || a.radio.moreErr != "" {
		t.Fatal("final page did not end pagination")
	}
	a.loadMoreStations()
	select {
	case <-requests:
		t.Fatal("end of directory requested another page")
	default:
	}
}

func TestStationPaginationRejectsOldGeneration(t *testing.T) {
	requests := recordStationRequests(t)
	a := newTestApp()
	a.loadStations()
	first := nextStationRequest(t, requests, 0)
	oldKey, oldGen := a.radio.asked, a.radio.generation
	a.radio.tag = 2
	a.loadStations()
	nextStationRequest(t, requests, 0)
	select {
	case <-first.ctx.Done():
	default:
		t.Fatal("changing filters did not cancel the old request")
	}
	a.radio.tag = 0
	a.loadStations()
	nextStationRequest(t, requests, 0)
	a.receiveStations(oldKey, oldGen, 0, radio.Page{Stations: []radio.Station{{ID: "obsolete"}}}, nil, time.Now())
	if len(a.radio.stations) != 0 || !a.radio.loading || len(a.radio.cache.entries) != 0 {
		t.Fatal("A -> B -> A accepted an obsolete first page")
	}
	a.receiveStations(oldKey, oldGen, 60, radio.Page{Stations: []radio.Station{{ID: "obsolete"}}}, nil, time.Now())
	if len(a.radio.stations) != 0 || !a.radio.loading {
		t.Fatal("an obsolete continuation changed the active request")
	}
}

func TestStationPaginationScrollAndFooter(t *testing.T) {
	for _, width := range []int{880, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			requests := recordStationRequests(t)
			a := newTestApp()
			key := a.stationKey()
			a.radio.asked = key
			stations := make([]radio.Station, 60)
			for i := range stations {
				stations[i] = radio.Station{ID: fmt.Sprint(i), Name: fmt.Sprintf("Station %02d", i), Stream: "http://example.com/live"}
			}
			a.receiveStations(key, 0, 0, radio.Page{Stations: stations, NextOffset: 60, HasMore: true}, nil, time.Now())
			tt := ui.NewTester(a.view, width, 800)
			tt.Frame()
			if a.radio.loading {
				t.Fatal("first frame loaded a page before nearing the end")
			}
			tt.Scroll(float32(width)-100, 450, 0, 100000)
			tt.Frame()
			nextStationRequest(t, requests, 60)
			a.receiveStations(key, 0, 60, radio.Page{}, errors.New("offline"), time.Now())
			tt.Frame()
			if _, ok := tt.Find("加载更多失败，请重试"); !ok {
				t.Fatal("continuation failure was not visible at the bottom")
			}
			if err := tt.Click("重试加载更多"); err != nil {
				t.Fatal(err)
			}
			nextStationRequest(t, requests, 60)
			a.receiveStations(key, 0, 60, radio.Page{NextOffset: 60}, nil, time.Now())
			tt.Frame()
			if _, ok := tt.Find("已全部加载"); !ok {
				t.Fatal("end-of-directory state was not visible")
			}
		})
	}
}

func TestStationLoadingIndicatorIsNotDuplicated(t *testing.T) {
	for _, width := range []int{880, 1200} {
		for _, state := range []string{"first", "refresh", "empty continuation", "continuation"} {
			t.Run(fmt.Sprintf("%d/%s", width, state), func(t *testing.T) {
				a := newTestApp()
				a.radio.asked, a.radio.loading = a.stationKey(), true
				if state != "first" {
					a.radio.nextOffset, a.radio.hasMore = 60, true
				}
				if state == "refresh" || state == "continuation" {
					a.radio.stations = sampleStations()
				}
				a.radio.stale = state == "refresh"
				tt := ui.NewTester(a.view, width, 800)
				tt.Frame()
				count := 0
				for _, text := range tt.Texts() {
					if strings.Contains(text, "正在加载") || strings.Contains(text, "后台更新中") {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("got %d loading indicators: %v", count, tt.Texts())
				}
			})
		}
	}
}

func TestEmptyStationContinuationStillOffersRetry(t *testing.T) {
	requests := recordStationRequests(t)
	a := newTestApp()
	a.radio.asked, a.radio.nextOffset, a.radio.hasMore = a.stationKey(), 60, true
	a.radio.moreErr = "加载更多失败，请重试"
	tt := ui.NewTester(a.view, 880, 800)
	tt.Frame()
	if tt.HasText("没有符合当前条件") {
		t.Fatal("a continuation error was presented as an empty result")
	}
	if err := tt.Click("重试加载更多"); err != nil {
		t.Fatal(err)
	}
	nextStationRequest(t, requests, 60)
}

func TestLibraryPagesDoNotPaginate(t *testing.T) {
	for _, page := range []pageRef{pageFavorites, pageRecent} {
		a := newTestApp()
		a.page = page
		a.radio.asked, a.radio.hasMore, a.radio.nextOffset = a.stationKey(), true, 60
		a.loadMoreStations()
		if a.radio.loading {
			t.Fatal("a local library page requested directory pagination")
		}
	}
}
