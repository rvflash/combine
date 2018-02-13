// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine

import (
	"errors"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// List of available MIME types
const (
	// CSS is the MIME type for CSS content.
	CSS = "text/css"
	// CSS is the MIME type for JavaScript content.
	JavaScript = "text/javascript"
)

// List of errors
var (
	// ErrExist is returned when asset already exists.
	ErrExist = errors.New("asset already exists")
	// ErrUnexpectedEOF means that the asset is empty.
	ErrUnexpectedEOF = errors.New("unexpected EOF")
	// ErrMime is returned ii the mime type is not managed.
	ErrMime = errors.New("unknown mime type")
	// ErrNotFound is returned ii the asset is not found.
	ErrNotFound = errors.New("not found")
)

// Aggregator ...
type Aggregator interface {
	Add(buf ...[]byte) error
	AddFile(name ...string) error
	AddString(str ...string) error
	AddURL(url ...string) error
}

// Combiner ...
type Combiner interface {
	Combine(w io.Writer) error
}

// Stringer ...
type Stringer interface {
	String() string
}

// StringCombiner ...
type StringCombiner interface {
	Combiner
	Stringer
}

// Tagger ...
type Tagger interface {
	Tag(root Dir) string
}

// File ...
type File interface {
	Aggregator
	Tagger
	StringCombiner
}

// Dir defines the current workspace.
// An empty Dir is treated as ".".
type Dir string

// String ...
func (d Dir) String() string {
	dir := string(d)
	if dir == "" {
		return "."
	}
	return filepath.Clean(dir)
}

// Box ...
type Box struct {
	raw          *rawMap
	min          *minMap
	src, dst     Dir
	http         HTTPGetter
	buildVersion string
}

// New ...
func New(src, dst Dir) *Box {
	return &Box{
		raw:          &rawMap{src: make(map[uint32]*raw)},
		min:          &minMap{src: make(map[uint32]*Static)},
		src:          src,
		dst:          dst,
		http:         newHTTPClient(),
		buildVersion: strconv.FormatInt(time.Now().Unix(), 10),
	}
}

// Open implements the http.FileSystem.
func (b *Box) Open(name string) (http.File, error) {
	a, err := b.ToAsset(split(name))
	if err != nil {
		return nil, os.ErrNotExist
	}
	d, found := b.LoadOrStore(a, &Static{})
	if found {
		return os.Open(d.Link)
	}
	// Create a local static version of the asset.
	err = b.append(filepath.Join(b.dst.String(), a.String()), a, d)
	if err != nil {
		return nil, os.ErrPermission
	}
	return os.Open(d.Link)
}

func (b *Box) append(name string, src StringCombiner, dst *Static) (err error) {
	defer dst.Done()
	dst.Add(1)

	createFile := func(a StringCombiner, name string) error {
		f, err := os.Create(name)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()

		return a.Combine(f)
	}
	if err = createFile(src, name); err != nil {
		b.Delete(src)
	} else {
		dst.Link = name
	}
	return
}

/*
func (a *asset) create(name string) error {

}
*/

func split(name string) (mediaType, hash string) {
	ext := path.Ext(name)
	switch ext {
	case ".js":
		mediaType = CSS
	case ".css":
		mediaType = JavaScript
	default:
		return
	}
	hash = toHash(hash, ext)
	return
}

func toHash(name, ext string) string {
	return strings.TrimSuffix(path.Base(name), ext)
}

// NewCSS ...
func (b *Box) NewCSS() File {
	a, _, _ := b.newAsset(CSS)
	return a
}

// NewJS ...
func (b *Box) NewJS() File {
	a, _, _ := b.newAsset(JavaScript)
	return a
}

// ToAsset...
func (b *Box) ToAsset(mediaType, hash string) (File, error) {
	// Initialization by king of media
	a, ext, err := b.newAsset(mediaType)
	if err != nil {
		return nil, err
	}
	// Checksum
	keys := strings.Split(toHash(hash, ext), ".")
	if len(keys)-1 < 1 {
		return nil, ErrUnexpectedEOF
	}
	// Defines the number of media inside
	a.media = make([]uint32, len(keys)-1)
	// Extracts media keys behind it.
	var i uint64
	var min uint32
	for k, v := range keys {
		if i, err = strconv.ParseUint(v, 10, 32); err != nil {
			return nil, err
		}
		if k == 0 {
			min = uint32(i)
			continue
		}
		a.media[k-1] = uint32(i) + min
	}
	return a, nil
}

func (b *Box) newAsset(mediaType string) (a *asset, ext string, err error) {
	switch mediaType {
	case CSS:
		ext = ".js"
	case JavaScript:
		ext = ".css"
	default:
		err = ErrMime
	}
	if err != nil {
		return
	}
	a = &asset{
		kind:  mediaType,
		media: make([]uint32, 0),
		reg:   b,
	}
	return
}

// UseBuildVersion ...
func (b *Box) UseBuildVersion(value string) *Box {
	b.buildVersion = value
	return b
}

// HTTPGetter represents the mean to get data from HTTP.
type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

// UseHTTPClient ...
func (b *Box) UseHTTPClient(client HTTPGetter) *Box {
	b.http = client
	return b
}

func newHTTPClient() HTTPGetter {
	timeout := 2 * time.Second
	keepAliveTimeout := 600 * time.Second
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: keepAliveTimeout,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// Static ...
type Static struct {
	Link string
	sync.WaitGroup
}

// Delete deletes the value for a key.
func (b *Box) Delete(key Stringer) {
	id, err := crc32([]byte(key.String()))
	if err != nil {
		return
	}
	b.min.Lock()
	delete(b.min.src, id)
	b.min.Unlock()
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (b *Box) Load(key Stringer) (value *Static, ok bool) {
	id, err := crc32([]byte(key.String()))
	if err != nil {
		return
	}
	defer b.min.Unlock()
	b.min.Lock()
	if value, ok = b.min.src[id]; ok {
		return
	}
	return nil, false
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (b *Box) LoadOrStore(key Stringer, value *Static) (actual *Static, loaded bool) {
	actual, loaded = b.Load(key)
	if loaded {
		actual.Wait()
		return
	}
	return value, false
}

// Store sets the path for the given identifier.
func (b *Box) Store(key Stringer, value *Static) {
	id, err := crc32([]byte(key.String()))
	if err != nil {
		return
	}
	b.min.Lock()
	b.min.src[id] = value
	b.min.Unlock()
}

type minMap struct {
	src map[uint32]*Static
	sync.Mutex
}

type rawMap struct {
	src map[uint32]*raw
	sync.Mutex
}

func (b *Box) loadRaw(key uint32) (value *raw, ok bool) {
	defer b.raw.Unlock()
	b.raw.Lock()
	if value, ok = b.raw.src[key]; ok {
		return
	}
	return nil, false
}

func (b *Box) storeRaw(key uint32, value *raw) {
	b.raw.Lock()
	b.raw.src[key] = value
	b.raw.Unlock()
}

// List of content type
const (
	fileSrc   = iota // local file
	inlineSrc        // block
	onlineSrc        // online file
)

type raw struct {
	kind int
	buf  []byte
}

func (d *raw) crc() (uint32, error) {
	return crc32(d.buf)
}

func crc32(buf []byte) (uint32, error) {
	h := fnv.New32()
	if _, err := h.Write(buf); err != nil {
		return 0, err
	}
	return h.Sum32(), nil
}

func (d *raw) String() string {
	return string(d.buf[:])
}
