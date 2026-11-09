package mpv

import (
	"errors"
	"runtime"
	"testing"
	"unsafe"
)

func TestMetadataReadsWholeMapAndFreesNode(t *testing.T) {
	oldGet, oldFree := api.getProperty, api.freeNode
	t.Cleanup(func() { api.getProperty, api.freeNode = oldGet, oldFree })
	key := append([]byte("lavfi.astats.3.RMS_level"), 0)
	value := append([]byte("-12.5"), 0)
	keys := []*byte{&key[0]}
	values := []nativeNode{{Value: unsafe.Pointer(&value[0]), Format: formatString}}
	list := &nativeNodeList{Num: 1, Values: &values[0], Keys: &keys[0]}
	freed := false
	api.getProperty = func(handle uintptr, name string, format int32, node *nativeNode) int32 {
		if handle != 1 || name != "af-metadata/spectrum" || format != formatNode {
			t.Fatalf("unexpected property request: %d %q %d", handle, name, format)
		}
		*node = nativeNode{Value: unsafe.Pointer(list), Format: formatNodeMap}
		return 0
	}
	api.freeNode = func(*nativeNode) { freed = true }
	inst := &Instance{handle: 1}
	got, err := inst.Metadata("af-metadata/spectrum")
	if err != nil || got["lavfi.astats.3.RMS_level"] != "-12.5" || !freed {
		t.Fatalf("metadata=%v err=%v freed=%v", got, err, freed)
	}
	runtime.KeepAlive(key)
	runtime.KeepAlive(value)
	runtime.KeepAlive(keys)
	runtime.KeepAlive(values)
	runtime.KeepAlive(list)
}

func TestMetadataUnavailableAndClosed(t *testing.T) {
	oldGet := api.getProperty
	t.Cleanup(func() { api.getProperty = oldGet })
	api.getProperty = func(uintptr, string, int32, *nativeNode) int32 { return -10 }
	inst := &Instance{handle: 1}
	if got, err := inst.Metadata("af-metadata/spectrum"); got != nil || err != nil {
		t.Fatalf("unavailable metadata=%v err=%v", got, err)
	}
	inst.handle = 0
	if _, err := inst.Metadata("af-metadata/spectrum"); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed metadata err=%v", err)
	}
}
