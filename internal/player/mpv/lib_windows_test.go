package mpv

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryPathIsAbsoluteWithNativeSeparators(t *testing.T) {
	for _, path := range []string{"resources/windows-amd64/mpv/mpv-2.dll", "resources\\windows-amd64\\mpv\\mpv-2.dll"} {
		got, err := libraryPath(path)
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(got) {
			t.Fatalf("not absolute: %q", got)
		}
		if strings.Contains(got, "/") {
			t.Fatalf("forward slash breaks dependency lookup: %q", got)
		}
	}
}
