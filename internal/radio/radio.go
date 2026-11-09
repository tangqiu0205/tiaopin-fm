// Package radio is the FM data source: the Radio-Browser open directory,
// asked of every mirror at once so one that is down does not hold the
// page up.
package radio

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Mirrors are the Radio-Browser API mirrors. They are all asked at once,
// so the one that answers first is the one listed from: from some
// networks only one of them is within reach.
var Mirrors = []string{
	"https://de1.api.radio-browser.info/json",
	"https://nl1.api.radio-browser.info/json",
	"https://at1.api.radio-browser.info/json",
	"https://all.api.radio-browser.info/json",
}

// UserAgent is what the directory is asked for stations as.
const UserAgent = "tiaopin-fm/1.0 (Desktop FM radio)"

// Station is a radio station as the app plays it.
type Station struct {
	// ID is the station's ID in the Radio-Browser directory.
	ID   string `json:"id"`
	Name string `json:"name"`
	// Stream is the station's live stream URL, played as it is.
	Stream string `json:"stream"`
	// Icon is the station's logo or favicon URL, "" for none.
	Icon string `json:"icon"`
	// Country and CountryCode say where the station broadcasts from.
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
	// Language is the station's language, and Tags its genres.
	Language string `json:"language"`
	Tags     string `json:"tags"`
	// Codec is the stream's audio codec: MP3, AAC...
	Codec string `json:"codec"`
	// Bitrate is the stream's bitrate in kbps, 0 unknown.
	Bitrate int `json:"bitrate"`
	// Clicks and Votes are how popular the station is.
	Clicks int `json:"clicks"`
	Votes  int `json:"votes"`
	// Curated is retained only for stations saved by older versions.
	Curated bool `json:"curated,omitempty"`
}

// ErrNoMirror says every mirror failed.
var ErrNoMirror = errors.New("radio: no mirror reachable")

var client = &http.Client{Timeout: 20 * time.Second}

// getTimeout is how long the mirrors are given to answer, asked all at
// once: past it, the caller receives an error.
const getTimeout = 12 * time.Second

// station is a station as the directory sends one: it names the stream
// url and url_resolved, the logo favicon, and the popularity clickcount.
type station struct {
	ID       string `json:"stationuuid"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Resolved string `json:"url_resolved"`
	Favicon  string `json:"favicon"`
	Country  string `json:"country"`
	CountryC string `json:"countrycode"`
	Language string `json:"language"`
	Tags     string `json:"tags"`
	Codec    string `json:"codec"`
	Bitrate  int    `json:"bitrate"`
	HLS      int    `json:"hls"`
	CheckOK  *int   `json:"lastcheckok"`
	Clicks   int    `json:"clickcount"`
	Votes    int    `json:"votes"`
}

// station returns the station as the app plays it, or false for one it
// cannot: without a stream, or one the directory last found broken. An HLS
// playlist plays as its segments, so it is kept like any other stream.
func (w station) station() (Station, bool) {
	stream := w.Resolved
	if stream == "" {
		stream = w.URL
	}
	switch {
	case stream == "", w.CheckOK != nil && *w.CheckOK == 0:
		return Station{}, false
	}
	return Station{
		ID: w.ID, Name: w.Name, Stream: stream, Icon: w.Favicon,
		Country: w.Country, CountryCode: w.CountryC, Language: w.Language,
		Tags: w.Tags, Codec: w.Codec, Bitrate: w.Bitrate,
		Clicks: w.Clicks, Votes: w.Votes,
	}, true
}

// get asks every mirror at once for path's stations, and takes the first
// answer: a mirror that is down, or slow, does not hold the page up, and
// one that cannot be reached at all from where the app runs does not
// either. Stations with nothing to play are left out.
func get(path string, query url.Values) ([]Station, error) {
	return getContext(context.Background(), path, query)
}

// stationPage retains the raw row count: discarded broken streams still
// occupy directory offsets and must not make pagination stop early.
type stationPage struct {
	stations []Station
	count    int
	indices  []int // raw row positions of retained stations, for a partial music batch
}

func getContext(parent context.Context, path string, query url.Values) ([]Station, error) {
	page, err := getPageContext(parent, path, query)
	return page.stations, err
}

func getPageContext(parent context.Context, path string, query url.Values) (stationPage, error) {
	ctx, cancel := context.WithTimeout(parent, getTimeout)
	defer cancel()
	type answer struct {
		page stationPage
		err  error
	}
	answers := make(chan answer, len(Mirrors))
	for _, base := range Mirrors {
		u := base + path
		if len(query) > 0 {
			u += "?" + query.Encode()
		}
		go func() {
			page, err := fetchPage(ctx, u)
			answers <- answer{page, err}
		}()
	}
	var lastErr error
	for range Mirrors {
		a := <-answers
		if a.err == nil {
			// Whoever answered first is the one asked: the rest fall away.
			cancel()
			return a.page, nil
		}
		lastErr = a.err
	}
	if lastErr == nil {
		lastErr = ErrNoMirror
	}
	return stationPage{}, lastErr
}

// fetchPage asks one mirror for stations and keeps its unfiltered row count.
func fetchPage(ctx context.Context, u string) (stationPage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return stationPage{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	res, err := client.Do(req)
	if err != nil {
		return stationPage{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return stationPage{}, fmt.Errorf("radio: %s: %s", u, res.Status)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return stationPage{}, err
	}
	var wire []station
	if err := json.Unmarshal(body, &wire); err != nil {
		return stationPage{}, err
	}
	stations := make([]Station, 0, len(wire))
	indices := make([]int, 0, len(wire))
	for i, w := range wire {
		if st, ok := w.station(); ok {
			stations = append(stations, st)
			indices = append(indices, i)
		}
	}
	return stationPage{stations: stations, count: len(wire), indices: indices}, nil
}

// Popular returns the limit most-clicked stations.
func Popular(limit int) ([]Station, error) {
	return get("/stations/topclick", url.Values{"limit": {strconv.Itoa(limit)}})
}

// Voted returns the limit most-voted stations.
func Voted(limit int) ([]Station, error) {
	return get("/stations/topvote", url.Values{"limit": {strconv.Itoa(limit)}})
}

// Filter combines independent directory facets. Empty fields impose no
// restriction.
type Filter struct {
	Name, Tag, CountryCode, Language string
}

// Page retains a raw directory cursor, not the number of usable stations.
// Filtering broken streams or music genres must not skip later results.
type Page struct {
	Stations   []Station `json:"stations"`
	NextOffset int       `json:"nextOffset"`
	HasMore    bool      `json:"hasMore"`
}

// Find filters on the server, then returns the most-clicked matches.
func Find(f Filter, limit int) ([]Station, error) {
	if limit <= 0 {
		return nil, nil
	}
	page, err := FindPage(context.Background(), f, limit, 0)
	return page.Stations, err
}

// FindPage returns a batch with the cursor to use for its continuation.
func FindPage(ctx context.Context, f Filter, limit, offset int) (Page, error) {
	if limit <= 0 || offset < 0 {
		return Page{}, errors.New("radio: invalid page limit or offset")
	}
	if strings.EqualFold(f.Tag, "music") {
		return findMusicPage(ctx, f, limit, offset)
	}
	page, err := findPage(ctx, f, limit, offset)
	if err != nil {
		return Page{}, err
	}
	return Page{Stations: page.stations, NextOffset: offset + page.count, HasMore: page.count >= limit}, nil
}

func findPage(ctx context.Context, f Filter, limit, offset int) (stationPage, error) {
	q := url.Values{
		"limit": {strconv.Itoa(limit)}, "hidebroken": {"true"},
		"order": {"clickcount"}, "reverse": {"true"},
	}
	if name := strings.TrimSpace(f.Name); name != "" {
		q.Set("name", name)
	}
	if f.Tag != "" {
		q.Set("tag", strings.ToLower(f.Tag))
		q.Set("tagExact", "true")
	}
	if f.CountryCode != "" {
		q.Set("countrycode", strings.ToUpper(f.CountryCode))
	}
	if f.Language != "" {
		q.Set("language", strings.ToLower(f.Language))
		q.Set("languageExact", "true")
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return getPageContext(ctx, "/stations/search", q)
}

// ByID refreshes one station without reloading a category or changing the
// click counter. A missing or unusable station remains an explicit failure.
func ByID(id string) (Station, error) {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return Station{}, errors.New("radio: invalid station UUID")
	}
	raw := strings.ReplaceAll(id, "-", "")
	if len(raw) != 32 {
		return Station{}, errors.New("radio: invalid station UUID")
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return Station{}, errors.New("radio: invalid station UUID")
	}
	id = strings.ToLower(id)
	stations, err := get("/stations/byuuid/"+url.PathEscape(id), nil)
	if err != nil {
		return Station{}, err
	}
	for _, st := range stations {
		if st.ID == id {
			return st, nil
		}
	}
	return Station{}, errors.New("radio: station has no available stream")
}

// Search returns the limit stations whose name matches query.
func Search(query string, limit int) ([]Station, error) {
	return Find(Filter{Name: query}, limit)
}

// ByCountry returns the limit stations of the country with code cc, as
// Radio-Browser codes it: "CN", "SG", "HK"... The code is part of the
// path, not a query: asked for in the query, the directory answers 404.
func ByCountry(cc string, limit int) ([]Station, error) {
	return get("/stations/bycountrycodeexact/"+url.PathEscape(strings.ToUpper(cc)), url.Values{
		"limit":      {strconv.Itoa(limit)},
		"hidebroken": {"true"}, "order": {"clickcount"}, "reverse": {"true"},
	})
}

// ByTag returns the limit stations tagged tag, as Radio-Browser tags
// them: "jazz", "pop", "news"... The tag is part of the path, as a
// country's code is.
func ByTag(tag string, limit int) ([]Station, error) {
	return get("/stations/bytag/"+url.PathEscape(strings.ToLower(tag)), url.Values{
		"limit":      {strconv.Itoa(limit)},
		"hidebroken": {"true"}, "order": {"clickcount"}, "reverse": {"true"},
	})
}

// Click tells the directory the station was played, for its counts.
func Click(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), getTimeout)
	defer cancel()
	for _, base := range Mirrors {
		req, err := http.NewRequestWithContext(ctx, "GET", base+"/url/"+id, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", UserAgent)
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			return
		}
	}
}
