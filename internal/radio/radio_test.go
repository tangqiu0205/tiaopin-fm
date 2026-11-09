package radio

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// directory serves stations at stations, whatever it is asked for.
func directory(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

// mirrors makes the mirrors the ones given, for the test, and puts back
// the directory's after.
func mirrors(t *testing.T, urls ...string) {
	t.Helper()
	old := Mirrors
	Mirrors = urls
	t.Cleanup(func() { Mirrors = old })
}

// A directory that answers lists its stations, but those with nothing to
// play.
func TestPopularHidesSilentStations(t *testing.T) {
	srv := directory(t, `[
		{"stationuuid":"a","name":"A","url":"http://a","url_resolved":"http://a/resolved","favicon":"http://a/i.png",
		 "codec":"MP3","bitrate":128,"countrycode":"SG","clickcount":9,"lastcheckok":1},
		{"stationuuid":"b","name":"B","url":"","lastcheckok":1},
		{"stationuuid":"c","name":"C","url":"http://c/live.m3u8","hls":1,"lastcheckok":1},
		{"stationuuid":"d","name":"D","url":"http://d","lastcheckok":0}
	]`, http.StatusOK)
	defer srv.Close()
	mirrors(t, srv.URL)
	list, err := Popular(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("stations %v", list)
	}
	// The resolved stream plays, and the favicon shows as its logo.
	if st := list[0]; st.ID != "a" || st.Stream != "http://a/resolved" || st.Icon != "http://a/i.png" || st.Clicks != 9 {
		t.Errorf("station %+v", st)
	}
	// The HLS station stays: its playlist is played as segments.
	if st := list[1]; st.ID != "c" {
		t.Errorf("station %+v", st)
	}
}

// A station with nothing to play is left out; an HLS one is not: its
// playlist is played as segments now, so dropping it here hid whole
// categories of stations, the Chinese ones among them.
func TestStationsKeepHLS(t *testing.T) {
	srv := directory(t, `[
		{"stationuuid":"a","name":"A","url":"http://a/live.m3u8","hls":1,"lastcheckok":1},
		{"stationuuid":"b","name":"B","url":"http://b","lastcheckok":0}
	]`, http.StatusOK)
	defer srv.Close()
	mirrors(t, srv.URL)
	list, err := Popular(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Stream != "http://a/live.m3u8" {
		t.Fatalf("stations %v", list)
	}
}

// The first mirror that answers is the one asked, and one that fails is
// passed over for the next.
func TestMirrorsFailOver(t *testing.T) {
	dead := directory(t, "no", http.StatusInternalServerError)
	defer dead.Close()
	junk := directory(t, "<html>", http.StatusOK)
	defer junk.Close()
	live := directory(t, `[{"stationuuid":"a","name":"A","url":"http://a","lastcheckok":1}]`, http.StatusOK)
	defer live.Close()
	mirrors(t, dead.URL, junk.URL, live.URL)
	list, err := Popular(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("stations %v", list)
	}
}

// The country's code and the tag are part of the path the directory is
// asked for: asked for as a query, it answers 404, and the page lists
// nothing at all.
func TestCountryAndTagArePath(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Write([]byte(`[{"stationuuid":"a","name":"A","url":"http://a","lastcheckok":1}]`))
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	if _, err := ByCountry("cn", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := ByTag("Jazz", 5); err != nil {
		t.Fatal(err)
	}
	want := "/stations/bycountrycodeexact/CN /stations/bytag/jazz"
	if got := strings.Join(asked, " "); got != want {
		t.Errorf("asked %s, want %s", got, want)
	}
}
