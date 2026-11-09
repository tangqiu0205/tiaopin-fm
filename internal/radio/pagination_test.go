package radio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestFindPageUsesRawOffset(t *testing.T) {
	rows := []map[string]any{
		{"stationuuid": "broken", "lastcheckok": 0, "url": "http://example.com/broken"},
		{"stationuuid": "a", "url": "http://example.com/a"},
		{"stationuuid": "b", "url": "http://example.com/b"},
		{"stationuuid": "c", "url": "http://example.com/c"},
		{"stationuuid": "d", "url": "http://example.com/d"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		offset, _ := strconv.Atoi(q.Get("offset"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		if q.Get("countrycode") != "US" || q.Get("tag") != "news" || q.Get("name") != "FM" || q.Get("language") != "english" {
			t.Errorf("page lost filters: %s", r.URL)
		}
		if offset > len(rows) || limit <= 0 {
			http.Error(w, "invalid page", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(rows[offset:min(offset+limit, len(rows))])
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	f := Filter{Name: "FM", Tag: "news", CountryCode: "US", Language: "english"}
	first, err := FindPage(context.Background(), f, 3, 0)
	if err != nil || len(first.Stations) != 2 || first.NextOffset != 3 || !first.HasMore {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	last, err := FindPage(context.Background(), f, 3, first.NextOffset)
	if err != nil || len(last.Stations) != 2 || last.Stations[0].ID != "c" || last.NextOffset != 5 || last.HasMore {
		t.Fatalf("last page=%+v err=%v", last, err)
	}
}

// A music batch can fill in the middle of an over-fetched raw page. The
// next batch must resume there, not skip the remaining music in that page.
func TestMusicPageResumesInsideRawPage(t *testing.T) {
	rows := []map[string]any{
		{"stationuuid": "news", "url": "http://example.com/news", "tags": "news"},
		{"stationuuid": "broken", "tags": "pop"},
		{"stationuuid": "rock", "url": "http://example.com/rock", "tags": "rock"},
		{"stationuuid": "jazz", "url": "http://example.com/jazz", "tags": "jazz"},
		{"stationuuid": "talk", "url": "http://example.com/talk", "tags": "talk"},
		{"stationuuid": "pop", "url": "http://example.com/pop", "tags": "pop"},
		{"stationuuid": "classical", "url": "http://example.com/classical", "tags": "classical"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		if offset != 0 && offset != 4 {
			t.Errorf("unexpected cursor: %d", offset)
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(rows[offset:])
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	first, err := FindPage(context.Background(), Filter{Tag: "music"}, 2, 0)
	if err != nil || len(first.Stations) != 2 || first.Stations[0].ID != "rock" || first.NextOffset != 4 || !first.HasMore {
		t.Fatalf("first music page=%+v err=%v", first, err)
	}
	last, err := FindPage(context.Background(), Filter{Tag: "music"}, 2, first.NextOffset)
	if err != nil || len(last.Stations) != 2 || last.Stations[0].ID != "pop" || last.NextOffset != 7 || last.HasMore {
		t.Fatalf("last music page=%+v err=%v", last, err)
	}
}

func TestFindPageCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tag := range []string{"", "music"} {
		if _, err := FindPage(ctx, Filter{Tag: tag}, 60, 0); err == nil {
			t.Fatalf("canceled %q request succeeded", tag)
		}
	}
}
