package radio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Music used to issue 21 genre searches; one popularity page must now cover
// multiple music genres, deduplicate stations and preserve all other filters.
func TestMusicUsesOnePageWhenEnoughMatches(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		q := r.URL.Query()
		if r.URL.Path != "/stations/search" || q.Get("countrycode") != "US" ||
			q.Get("language") != "english" || q.Get("name") != "FM" ||
			q.Has("tag") || q.Has("tagList") || q.Get("order") != "clickcount" ||
			q.Get("reverse") != "true" || q.Get("hidebroken") != "true" {
			t.Errorf("batch request lost its filters or still queries genres: %s", r.URL)
		}
		w.Write([]byte(`[
			{"stationuuid":"news","url":"http://example.com/news","tags":"news,talk","clickcount":200},
			{"stationuuid":"rock","url":"http://example.com/rock","tags":"rock","clickcount":100},
			{"stationuuid":"classical","url":"http://example.com/classical","tags":"classical","clickcount":90},
			{"stationuuid":"jazz","url":"http://example.com/jazz","tags":"jazz","clickcount":80},
			{"stationuuid":"shared","url":"http://example.com/shared","tags":"jazz,pop","clickcount":70},
			{"stationuuid":"shared","url":"http://example.com/shared","tags":"pop","clickcount":65},
			{"stationuuid":"pop","url":"http://example.com/pop","tags":"pop","clickcount":60},
			{"stationuuid":"music","url":"http://example.com/music","tags":"music","clickcount":10}
		]`))
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	stations, err := Find(Filter{Tag: "music", CountryCode: "US", Language: "english", Name: "FM"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, st := range stations {
		ids = append(ids, st.ID)
	}
	if want := []string{"rock", "classical", "jazz", "shared", "pop"}; !slices.Equal(ids, want) {
		t.Fatalf("music result %v, want %v", ids, want)
	}
	if n := requests.Load(); n != 1 {
		t.Fatalf("music made %d requests, want one", n)
	}
}

// Discarded HLS streams still consume directory offsets; sparse music
// must continue at the raw page boundary rather than stop after decoding.
func TestMusicContinuesPastNonMusicAndDiscardedStreams(t *testing.T) {
	var offsets []int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		mu.Lock()
		offsets = append(offsets, offset)
		mu.Unlock()
		if offset == 0 {
			rows := musicSparsePage()
			json.NewEncoder(w).Encode(rows)
			return
		}
		if offset != 60 {
			t.Errorf("offset %d skipped or repeated directory rows", offset)
		}
		w.Write([]byte(`[
			{"stationuuid":"0","url":"http://example.com/0","tags":"pop","clickcount":70},
			{"stationuuid":"jazz","url":"http://example.com/jazz","tags":"jazz","clickcount":60},
			{"stationuuid":"classical","url":"http://example.com/classical","tags":"classical","clickcount":50}
		]`))
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	stations, err := Find(Filter{Tag: "music"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, st := range stations {
		ids = append(ids, st.ID)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(ids, []string{"0", "jazz", "classical"}) || !slices.Equal(offsets, []int{0, 60}) {
		t.Fatalf("music result %v, requested offsets %v", ids, offsets)
	}
}

// A failed continuation must report failure, not cache or display the
// incomplete music matches from earlier pages as a successful result.
func TestMusicFailureDoesNotReturnPartialSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") != "" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(musicSparsePage())
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	stations, err := Find(Filter{Tag: "music"}, 5)
	if err == nil || len(stations) != 0 {
		t.Fatalf("partial result disguised as success: %v, %v", stations, err)
	}
}

// A short final page ends the music scan even if fewer matches exist than
// requested, rather than asking the directory for empty pages indefinitely.
func TestMusicStopsAtDirectoryEnd(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) > 1 {
			http.Error(w, "unexpected extra page", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`[
			{"stationuuid":"a","url":"http://example.com/a","tags":"news"},
			{"stationuuid":"b","url":"http://example.com/b","tags":"rock"}
		]`))
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	stations, err := Find(Filter{Tag: "music"}, 5)
	if err != nil || len(stations) != 1 || stations[0].ID != "b" || requests.Load() != 1 {
		t.Fatalf("end of directory: stations %v, error %v, requests %d", stations, err, requests.Load())
	}
}

// With LIVE set, verifies that the real directory supplies music matches
// through the paginated query and logs elapsed time without timing assertions.
func TestMusicLive(t *testing.T) {
	if os.Getenv("LIVE") == "" {
		t.Skip("set LIVE to query Radio-Browser")
	}
	start := time.Now()
	stations, err := Find(Filter{Tag: "music", CountryCode: "CN", Language: "chinese"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) == 0 {
		t.Fatal("the directory returned no music stations")
	}
	for _, st := range stations {
		if !matchesContentTag(st.Tags, "music") {
			t.Errorf("non-music station %q: %s", st.Name, st.Tags)
		}
	}
	t.Logf("loaded %d music stations in %s", len(stations), time.Since(start).Round(time.Millisecond))
}

func musicSparsePage() []map[string]any {
	rows := make([]map[string]any, 60)
	for i := range rows {
		rows[i] = map[string]any{
			"stationuuid": fmt.Sprint(i), "url": "http://example.com/" + fmt.Sprint(i),
			"tags": "news", "clickcount": 1000 - i,
		}
	}
	rows[0]["tags"] = "pop"
	rows[59]["hls"] = 1
	return rows
}

// Offline music matching uses the same broad genre membership as network
// queries, without treating a pure news or talk station as music.
func TestMusicMatchesGenresOffline(t *testing.T) {
	for _, tag := range []string{"music", "pop", "classical", "jazz", "rock", "lofi", "news, Jazz"} {
		if !matchesContentTag(tag, "music") {
			t.Errorf("music excluded %q", tag)
		}
	}
	for _, tag := range []string{"news", "talk", "", "rockets"} {
		if matchesContentTag(tag, "music") {
			t.Errorf("music included %q", tag)
		}
	}
	if matchesContentTag("jazz", "news") || !matchesContentTag("news,talk", "news") {
		t.Fatal("grouping music changed the news category")
	}
}
