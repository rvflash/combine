// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

// Package combine provides interface to create assets with multiple
// source of contents and to combine it on the fly.
// It also provides methods to launch a file server to serve them.
package combine

import (
	"errors"
	"fmt"
	"hash/fnv"
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

// Dir defines the current workspace.
// An empty Dir is treated as ".".
type Dir string

// String implements the fmt.Stringer interface.
func (d Dir) String() string {
	dir := string(d)
	if dir == "" {
		return "."
	}
	return filepath.Clean(dir)
}

// Box represent a virtual folder to store or retrieve
// combined and minified assets.
type Box struct {
	raw          *rawMap
	min          *minMap
	src, dst     Dir
	http         HTTPGetter
	buildVersion string
}

type minMap struct {
	src map[uint32]*Static
	sync.RWMutex
}

type rawMap struct {
	src map[uint32]*raw
	sync.RWMutex
}

// NewBox returns a new instance of Box.
func NewBox(src, dst Dir) *Box {
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
	// Transforms the file name to an asset
	a, err := b.ToAsset(basename(name))
	if err != nil {
		return nil, os.ErrNotExist
	}
	// Tries to retrieve it if exists.
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

// Close cleans it workspace by removing cache files.
// It implements the io.Closer interface.
func (b *Box) Close() error {
	b.min.Lock()
	defer b.min.Unlock()

	for i, s := range b.min.src {
		fmt.Printf("%d. %s\n", i, s)
	}
	return nil
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

func basename(name string) (mediaType, hash string) {
	ext := path.Ext(name)
	switch ext {
	case ".js":
		mediaType = JavaScript
	case ".css":
		mediaType = CSS
	default:
		return
	}
	hash = toHash(name, ext)
	return
}

func toHash(name, ext string) string {
	return strings.TrimSuffix(path.Base(name), ext)
}

// NewCSS returns a new resource CSS.
func (b *Box) NewCSS() File {
	a, _, _ := b.newAsset(CSS)
	return a
}

// NewCSS returns a new resource JS.
func (b *Box) NewJS() File {
	a, _, _ := b.newAsset(JavaScript)
	return a
}

// ToAsset transforms a hash with its media type to a CSS or JS asset.
// If it fails, an error is returned instead.
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
	// Extracts the media keys behind it.
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

// UseBuildVersion overwrites the default buidd version by the given value.
// This build ID prevents unwanted browser caching after changing of the asset.
func (b *Box) UseBuildVersion(value string) *Box {
	b.buildVersion = value
	return b
}

// HTTPGetter represents the mean to get data from HTTP.
type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

// UseHTTPClient allows to use your own HTTP client or proxy.
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

// Static represents the minified and combined version of the asset.
type Static struct {
	Link string
	sync.WaitGroup
}

// Delete deletes the value for a key.
func (b *Box) Delete(key fmt.Stringer) {
	id, err := crc32([]byte(key.String()))
	if err != nil {
		return
	}
	b.min.Lock()
	delete(b.min.src, id)
	b.min.Unlock()
}

// Load returns the value stored in the map for a key,
// or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (b *Box) Load(key fmt.Stringer) (value *Static, ok bool) {
	id, err := crc32([]byte(key.String()))
	if err != nil {
		return
	}
	b.min.RLock()
	value, ok = b.min.src[id]
	b.min.RUnlock()
	return
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (b *Box) LoadOrStore(key fmt.Stringer, value *Static) (actual *Static, loaded bool) {
	actual, loaded = b.Load(key)
	if loaded {
		actual.Wait()
		return
	}
	return value, false
}

// Store sets the path for the given identifier.
func (b *Box) Store(key fmt.Stringer, value *Static) {
	id, err := crc32([]byte(key.String()))
	if err != nil {
		return
	}
	b.min.Lock()
	b.min.src[id] = value
	b.min.Unlock()
}

func (b *Box) loadRaw(key uint32) (value *raw, ok bool) {
	b.raw.RLock()
	value, ok = b.raw.src[key]
	b.raw.RUnlock()
	return
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
