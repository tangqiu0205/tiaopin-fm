package mpv

import (
	"path/filepath"

	"golang.org/x/sys/windows"
)

// libraryPath normalizes the native separator: LoadLibraryEx derives the
// dependency directory from this string and misses sibling DLLs given slashes.
func libraryPath(path string) (string, error) {
	return filepath.Abs(path)
}

func openLibrary(path string) (uintptr, error) {
	path, err := libraryPath(path)
	if err != nil {
		return 0, err
	}
	// Restrict dependent DLL lookup to the library's directory and System32.
	h, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	return uintptr(h), err
}
func librarySymbol(h uintptr, name string) (uintptr, error) {
	return windows.GetProcAddress(windows.Handle(h), name)
}
func closeLibrary(h uintptr) { _ = windows.FreeLibrary(windows.Handle(h)) }
