package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPortableDataDir(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker string
		block  bool
		want   bool
		fail   bool
	}{
		{name: "installed"},
		{name: "portable", marker: "file", want: true},
		{name: "invalid marker", marker: "directory", fail: true},
		{name: "unusable data directory", marker: "file", block: true, fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			marker := filepath.Join(root, "portable.flag")
			if tc.marker == "file" {
				if err := os.WriteFile(marker, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			} else if tc.marker == "directory" {
				if err := os.Mkdir(marker, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			data := filepath.Join(root, "data")
			if tc.block {
				if err := os.WriteFile(data, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := portableDataDir(filepath.Join(root, "TiaopinFM.exe"))
			if (err != nil) != tc.fail {
				t.Fatalf("path=%q err=%v", got, err)
			}
			if tc.want {
				if got != data {
					t.Fatalf("path=%q, want %q", got, data)
				}
				// Reopening preserves data and leaves no write-probe files.
				settings := filepath.Join(data, "settings.json")
				if err := os.WriteFile(settings, []byte(`{"volume":0.5}`), 0o600); err != nil {
					t.Fatal(err)
				}
				if _, err := portableDataDir(filepath.Join(root, "TiaopinFM.exe")); err != nil {
					t.Fatal(err)
				}
				entries, err := os.ReadDir(data)
				if err != nil || len(entries) != 1 || entries[0].Name() != "settings.json" {
					t.Fatalf("data entries=%v err=%v", entries, err)
				}
			} else if got != "" {
				t.Fatalf("unexpected path %q", got)
			}
			if tc.marker == "" {
				if _, err := os.Stat(data); !os.IsNotExist(err) {
					t.Fatalf("installed mode created portable data: %v", err)
				}
			}
		})
	}
}
