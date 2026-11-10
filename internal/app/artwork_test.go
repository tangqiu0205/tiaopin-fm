package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(0, 0, color.RGBA{R: 200, G: 40, B: 90, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// serve answers every request with body and a type.
func serve(body []byte, ct string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		_, _ = w.Write(body)
	}))
}

func TestStationIconDownscalesAndPreservesAspect(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 512, 256))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	bmp, resized, err := decodeStationIcon(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	w, h := bmp.Size()
	if w != stationIconSize || h != stationIconSize/2 || resized.Bounds().Dx() != w {
		t.Fatalf("unexpected resized icon: %dx%d", w, h)
	}
}

func TestStationIconCacheBounded(t *testing.T) {
	c := newArtCache(func(f func()) { f() })
	base := time.Unix(1000, 0)
	c.art["playing-cover"] = &artwork{used: base}
	for i := 0; i <= stationIconLimit; i++ {
		key := fmt.Sprintf("%03d#station-icon", i)
		c.art[key] = &artwork{used: base.Add(time.Duration(i) * time.Second)}
		c.trimStationIcons()
	}
	if len(c.art) != stationIconLimit+1 || c.art["000#station-icon"] != nil || c.art["playing-cover"] == nil {
		t.Fatal("station icon eviction did not preserve the limit and unrelated covers")
	}
}

func TestFetchKeepsImage(t *testing.T) {
	want := pngBytes(t)
	srv := serve(want, "image/png")
	defer srv.Close()
	got, err := fetchImageBytes(srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("body changed: %d bytes in, %d out", len(want), len(got))
	}
}

// A host that answers a logo's URL with a page: the directory's logos
// sometimes land on a challenge page, which is not a cover however the URL
// is named. It is turned down before its body is read.
func TestFetchTurnsDownPage(t *testing.T) {
	srv := serve([]byte(`<!DOCTYPE html><html><script>(function(){window.location.reload()}())</script></html>`), "text/html")
	defer srv.Close()
	if _, err := fetchImageBytes(srv.URL); err == nil {
		t.Fatal("a page was taken for a cover")
	}
}

// And a host that names an image's type but sends something else.
func TestFetchTurnsDownNamedImage(t *testing.T) {
	srv := serve([]byte("<html>not a cover</html>"), "image/jpeg")
	defer srv.Close()
	if _, err := fetchImageBytes(srv.URL); err == nil {
		t.Fatal("a page named an image was taken for a cover")
	}
}

// What says nothing of its type, or says bytes, is kept: hosts are careless
// with the headers of their logos, and the bytes say what it is.
func TestFetchKeepsCarelessType(t *testing.T) {
	want := pngBytes(t)
	for _, ct := range []string{"", "application/octet-stream"} {
		srv := serve(want, ct)
		if _, err := fetchImageBytes(srv.URL); err != nil {
			t.Errorf("type %q: %v", ct, err)
		}
		srv.Close()
	}
}

func TestImageData(t *testing.T) {
	cases := []struct {
		name string
		b    []byte
		want bool
	}{
		{"png", pngBytes(t), true},
		{"jpeg", []byte("\xff\xd8\xff\xe0rest"), true},
		{"gif", []byte("GIF89a..."), true},
		{"webp", []byte("RIFF\x10\x00\x00\x00WEBPVP8 "), true},
		{"html", []byte("<!DOCTYPE html>"), false},
		{"riff not webp", []byte("RIFF\x10\x00\x00\x00WAVEfmt "), false},
		{"empty", nil, false},
	}
	for _, c := range cases {
		if got := isImageData(c.b); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMediaType(t *testing.T) {
	cases := map[string]string{
		"image/png; charset=binary": "image/png",
		"TEXT/HTML":                 "text/html",
		"  image/webp  ":            "image/webp",
		"":                          "",
	}
	for in, want := range cases {
		if got := mediaType(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// A logo that did not load is remembered for the hour, the cover that did
// for the five minutes: the same host answers the same way, and asking it
// again only reads a page the app cannot use.
func TestFailedLogoRemembered(t *testing.T) {
	c := newArtCache(func(f func()) { f() })
	t0 := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	c.art["http://example.invalid/page"] = &artwork{failed: true, used: t0}
	c.art["http://example.invalid/logo.png"] = &artwork{used: t0}

	sweepAt := func(at time.Time) {
		c.swept = at.Add(-time.Minute)
		c.asked = at
		c.sweep(at)
	}
	sweepAt(t0.Add(10 * time.Minute))
	if _, ok := c.art["http://example.invalid/page"]; !ok {
		t.Error("a logo that did not load was asked for again within the hour")
	}
	if _, ok := c.art["http://example.invalid/logo.png"]; ok {
		t.Error("a cover not asked for in five minutes was kept")
	}
	sweepAt(t0.Add(70 * time.Minute))
	if _, ok := c.art["http://example.invalid/page"]; ok {
		t.Error("a logo that did not load was kept past the hour")
	}
}
