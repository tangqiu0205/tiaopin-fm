// Package mpv is the small, dynamically loaded libmpv client used by playback
// and recognition. No native code or executable is downloaded at runtime.
package mpv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

var ErrUnavailable = errors.New("无法加载 libmpv，请重新安装完整应用，或先准备开发环境的 mpv 组件")
var ErrClosed = errors.New("mpv: instance closed")

var api struct {
	create      func() uintptr
	initialize  func(uintptr) int32
	destroy     func(uintptr)
	command     func(uintptr, **byte) int32
	option      func(uintptr, string, string) int32
	set         func(uintptr, string, string) int32
	get         func(uintptr, string) *byte
	getProperty func(uintptr, string, int32, *nativeNode) int32
	freeNode    func(*nativeNode)
	free        func(unsafe.Pointer)
	wait        func(uintptr, float64) *nativeEvent
	errorString func(int32) string
}
var loadMu sync.Mutex
var loaded bool

func libraryName() string {
	switch runtime.GOOS {
	case "windows":
		return "mpv-2.dll"
	case "darwin":
		return "libmpv.dylib"
	default:
		return "libmpv.so.2"
	}
}

func load() error {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loaded {
		return nil
	}
	var paths []string
	if override := os.Getenv("TIAOPIN_MPV_LIBRARY"); override != "" {
		paths = []string{override}
	} else {
		dir, err := mygo.App.Path(mygo.PathResources)
		if err != nil {
			return err
		}
		paths = []string{filepath.Join(dir, "mpv", libraryName()), filepath.Join(dir, runtime.GOOS+"-"+runtime.GOARCH, "mpv", libraryName())}
	}
	var path string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		return fmt.Errorf("%w（查找路径：%s）", ErrUnavailable, strings.Join(paths, ", "))
	}
	h, err := openLibrary(path)
	if err != nil {
		return fmt.Errorf("%w：%s: %v", ErrUnavailable, path, err)
	}
	symbols := map[string]any{
		"mpv_create": &api.create, "mpv_initialize": &api.initialize,
		"mpv_terminate_destroy": &api.destroy, "mpv_command": &api.command,
		"mpv_set_option_string": &api.option, "mpv_set_property_string": &api.set,
		"mpv_get_property_string": &api.get, "mpv_free": &api.free,
		"mpv_get_property": &api.getProperty, "mpv_free_node_contents": &api.freeNode,
		"mpv_wait_event": &api.wait, "mpv_error_string": &api.errorString,
	}
	for name, fn := range symbols {
		addr, err := librarySymbol(h, name)
		if err != nil || addr == 0 {
			closeLibrary(h)
			return fmt.Errorf("%w：missing symbol %s (%v)", ErrUnavailable, name, err)
		}
		purego.RegisterFunc(fn, addr)
	}
	// Keep the module loaded for the lifetime of the process; function pointers
	// and all instances refer to it.
	loaded = true
	return nil
}

// Instance serializes all C calls, including event polling and destruction.
// Poll is deliberately non-blocking: destroying a handle can never race a
// native thread still using it in mpv_wait_event.
type Instance struct {
	mu     sync.Mutex
	handle uintptr
}

func New(options map[string]string) (*Instance, error) {
	if err := load(); err != nil {
		return nil, err
	}
	h := api.create()
	if h == 0 {
		return nil, errors.New("mpv: create failed")
	}
	i := &Instance{handle: h}
	defaults := map[string]string{
		// Scripting and ytdl are excluded from the bundled audio build, so their
		// options do not exist; the library simply never runs scripts.
		"config": "no", "terminal": "no",
		"vid": "no", "audio-display": "no", "idle": "yes", "keep-open": "no",
		"audio-client-name": "TiaopinFM", "network-timeout": "20",
		"demuxer-max-bytes": "8MiB", "demuxer-max-back-bytes": "0",
	}
	for k, v := range options {
		defaults[k] = v
	}
	for k, v := range defaults {
		if code := api.option(h, k, v); code < 0 {
			i.Close()
			return nil, fmt.Errorf("mpv option %s: %s", k, api.errorString(code))
		}
	}
	if code := api.initialize(h); code < 0 {
		i.Close()
		return nil, fmt.Errorf("mpv initialize: %s", api.errorString(code))
	}
	return i, nil
}

func (i *Instance) Set(name, value string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handle == 0 {
		return ErrClosed
	}
	if code := api.set(i.handle, name, value); code < 0 {
		return fmt.Errorf("mpv set %s: %s", name, api.errorString(code))
	}
	return nil
}

func (i *Instance) Get(name string) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handle == 0 {
		return ""
	}
	ptr := api.get(i.handle, name)
	if ptr == nil {
		return ""
	}
	defer api.free(unsafe.Pointer(ptr))
	return cString(ptr)
}

const (
	formatString  = 1
	formatNode    = 6
	formatNodeMap = 8
)

// The union in mpv_node is eight bytes, including on 32-bit platforms.
type nativeNode struct {
	Value  unsafe.Pointer
	Format int32
}

type nativeNodeList struct {
	Num    int32
	Values *nativeNode
	Keys   **byte
}

// Metadata reads the whole map: older libmpv versions can dereference a nil
// metadata pointer when querying the type of an individual by-key property.
func (i *Instance) Metadata(name string) (map[string]string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handle == 0 {
		return nil, ErrClosed
	}
	var node nativeNode
	if code := api.getProperty(i.handle, name, formatNode, &node); code < 0 {
		if code == -10 { // MPV_ERROR_PROPERTY_UNAVAILABLE: filter not ready.
			return nil, nil
		}
		return nil, fmt.Errorf("mpv metadata %s: %s", name, api.errorString(code))
	}
	defer api.freeNode(&node)
	if node.Format != formatNodeMap {
		return nil, fmt.Errorf("mpv metadata %s: expected map, got format %d", name, node.Format)
	}
	return nodeStringMap(node), nil
}

func nodeStringMap(node nativeNode) map[string]string {
	if node.Format != formatNodeMap || node.Value == nil {
		return nil
	}
	list := (*nativeNodeList)(node.Value)
	if list.Num <= 0 {
		return nil
	}
	values := unsafe.Slice(list.Values, int(list.Num))
	keys := unsafe.Slice(list.Keys, int(list.Num))
	result := make(map[string]string, list.Num)
	for n, value := range values {
		if value.Format == formatString {
			result[cString(keys[n])] = cString((*byte)(value.Value))
		}
	}
	return result
}

func cString(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	var b []byte
	for n := uintptr(0); ; n++ {
		c := *(*byte)(unsafe.Add(unsafe.Pointer(ptr), n))
		if c == 0 {
			return string(b)
		}
		b = append(b, c)
	}
}

func (i *Instance) Command(args ...string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handle == 0 {
		return ErrClosed
	}
	data := make([][]byte, len(args))
	ptrs := make([]*byte, len(args)+1)
	for n, arg := range args {
		if strings.IndexByte(arg, 0) >= 0 {
			return errors.New("mpv: NUL in command")
		}
		data[n] = append([]byte(arg), 0)
		ptrs[n] = &data[n][0]
	}
	code := api.command(i.handle, &ptrs[0])
	runtime.KeepAlive(data)
	if code < 0 {
		return fmt.Errorf("mpv command: %s", api.errorString(code))
	}
	return nil
}

const (
	EventEndFile         = 7
	EventFileLoaded      = 8
	EventPlaybackRestart = 21
	ReasonEOF            = 0
	ReasonError          = 4
)

type Event struct {
	ID     int32
	Reason int32
	Err    error
}
type nativeEvent struct {
	ID       int32
	Error    int32
	Userdata uint64
	Data     unsafe.Pointer
}
type endFile struct {
	Reason int32
	Error  int32
}

func (i *Instance) Poll() Event {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handle == 0 {
		return Event{}
	}
	ptr := api.wait(i.handle, 0)
	if ptr == nil {
		return Event{}
	}
	e := *ptr
	result := Event{ID: e.ID}
	if e.ID == EventEndFile && e.Data != nil {
		end := *(*endFile)(e.Data)
		result.Reason = end.Reason
		if end.Error < 0 {
			result.Err = fmt.Errorf("mpv: %s", api.errorString(end.Error))
		}
	}
	return result
}

func (i *Instance) Close() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.handle != 0 {
		api.destroy(i.handle)
		i.handle = 0
	}
}
