package app

import (
	"os"
	"path/filepath"
	"testing"
)

// The data kept under the app's old name moves to the directory of its
// name, which Path made empty, but not over data kept there already.
func TestMoveOldData(t *testing.T) {
	root := t.TempDir()
	old, dir := filepath.Join(root, oldNames[1]), filepath.Join(root, "TiaopinFM")
	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "settings.json"), []byte("old"), 0o600)
	os.MkdirAll(dir, 0o700)
	moveDir(old, dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != "old" {
		t.Fatalf("settings %q after moving", b)
	}

	os.MkdirAll(old, 0o700)
	os.WriteFile(filepath.Join(old, "settings.json"), []byte("older"), 0o600)
	moveDir(old, dir)
	if b, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(b) != "old" {
		t.Fatalf("settings %q, moved over what was there", b)
	}
}
