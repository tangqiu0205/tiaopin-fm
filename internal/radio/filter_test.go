package radio

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

// Facets must be combined on the server before limiting, with exact tags
// and languages rather than a name query masquerading as a category.
func TestFindCombinesFacets(t *testing.T) {
	requests := make(chan url.Values, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stations/search" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		requests <- r.URL.Query()
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	_, err := Find(Filter{Name: "  Jazz & Soul  ", Tag: "Jazz", CountryCode: "us", Language: "English"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	want := url.Values{
		"name": {"Jazz & Soul"}, "tag": {"jazz"}, "tagExact": {"true"},
		"countrycode": {"US"}, "language": {"english"}, "languageExact": {"true"},
		"order": {"clickcount"}, "reverse": {"true"}, "hidebroken": {"true"}, "limit": {"60"},
	}
	if got := <-requests; !reflect.DeepEqual(got, want) {
		t.Fatalf("query %v, want %v", got, want)
	}
	if _, err := Find(Filter{}, 10); err != nil {
		t.Fatal(err)
	}
	want = url.Values{"order": {"clickcount"}, "reverse": {"true"}, "hidebroken": {"true"}, "limit": {"10"}}
	if got := <-requests; !reflect.DeepEqual(got, want) {
		t.Fatalf("unfiltered query %v, want %v", got, want)
	}
}

// Refreshing a stream must query the exact UUID and use its newly resolved
// URL, without falling back to another station when the UUID disappears.
func TestByIDRefreshesOnlyRequestedStation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stations/byuuid/00000000-0000-0000-0000-000000000001" {
			w.Write([]byte(`[{"stationuuid":"00000000-0000-0000-0000-000000000001","name":"A","url":"http://old","url_resolved":"http://new","lastcheckok":1}]`))
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	mirrors(t, srv.URL)
	st, err := ByID("00000000-0000-0000-0000-000000000001")
	if err != nil || st.ID != "00000000-0000-0000-0000-000000000001" || st.Stream != "http://new" {
		t.Fatalf("refreshed station: %+v, %v", st, err)
	}
	if _, err := ByID("00000000-0000-0000-0000-000000000002"); err == nil {
		t.Fatal("a removed station was reported as available")
	}
}
