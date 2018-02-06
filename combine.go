// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine

import (
	"errors"
	"hash/fnv"
	"io"
	"io/ioutil"
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

// String ...
func (d Dir) String() string {
	dir := string(d)
	if dir == "" {
		return "."
	}
	return path.Clean(dir)
}

// New ...
func New(root Dir) *Data {
	return &Data{
		raw:  &rawMap{src: make(map[uint32]*raw)},
		min:  &minMap{src: make(map[uint32]*min)},
		root: root,
		http: newHTTPClient(),
	}
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

// Data ...
type Data struct {
	raw  *rawMap
	min  *minMap
	root Dir
	http HTTPGetter
}

type minMap struct {
	src map[uint32]*min
	sync.Mutex
}

// NewStatic ...
func NewStatic() *min {
	return &min{}
}

type min struct {
	Link string
	sync.WaitGroup
}

type rawMap struct {
	src map[uint32]*raw
	sync.Mutex
}

// List of content type
const (
	fileSrc   = iota // local file
	inlineSrc        // block
	onlineSrc        // online file
)

type raw struct {
	kind   int
	reader io.Reader
}

func (d *raw) crc() (uint32, error) {
	buf, err := ioutil.ReadAll(d.reader)
	if err != nil {
		return 0, err
	}
	return crc32(buf)
}

func crc32(buf []byte) (uint32, error) {
	h := fnv.New32()
	if _, err := h.Write(buf); err != nil {
		return 0, err
	}
	return h.Sum32(), nil
}

func (d *raw) readAll() (string, error) {
	name, err := ioutil.ReadAll(d.reader)
	if err != nil {
		return "", err
	}
	return string(name[:]), nil
}

func (d *Data) loadRawSrc(key uint32) (value *raw, ok bool) {
	defer d.raw.Unlock()
	d.raw.Lock()
	if value, ok = d.raw.src[key]; ok {
		return
	}
	return nil, false
}

func (d *Data) storeRawSrc(key uint32, value *raw) {
	d.raw.Lock()
	d.raw.src[key] = value
	d.raw.Unlock()
}

// NewCSS ...
func (d *Data) NewCSS() *Asset {
	return &Asset{
		kind:  CSS,
		media: make(map[int]uint32),
		reg:   d,
	}
}

// NewJS ...
func (d *Data) NewJS() *Asset {
	return &Asset{
		kind:  JavaScript,
		media: make(map[int]uint32),
		reg:   d,
	}
}

// FileServer implements the http.Handler.
func (d *Data) FileServer(root Dir) http.Handler {
	dir := filepath.Join(root.String(), "combine")
	return &handler{
		reg:  d,
		root: dir,
		err:  os.MkdirAll(dir, os.ModePerm),
	}
}

// Delete deletes the value for a key.
func (d *Data) Delete(a *Asset) {
	d.min.Lock()
	delete(d.min.src, a.ID())
	d.min.Unlock()
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (d *Data) Load(a *Asset) (media *min, ok bool) {
	defer d.min.Unlock()
	d.min.Lock()
	if media, ok = d.min.src[a.ID()]; ok {
		return
	}
	return nil, false
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (d *Data) LoadOrStore(a *Asset, media *min) (actual *min, loaded bool) {
	actual, loaded = d.Load(a)
	if loaded {
		actual.Wait()
		return
	}
	return media, false
}

// Store sets the path for the given identifier.
func (d *Data) Store(a *Asset, media *min) {
	d.min.Lock()
	d.min.src[a.ID()] = media
	d.min.Unlock()
}

// ToAsset...
func (d *Data) ToAsset(mediaType, hash string) (a *Asset, err error) {
	switch mediaType {
	case CSS:
		a = d.NewCSS()
	case JavaScript:
		a = d.NewJS()
	default:
		err = ErrMime
		return
	}
	// Extracts media keys behind it.
	var min uint32
	for k, v := range strings.Split(toHash(hash, a.Ext()), ".") {
		i, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return
		}
		if k == 0 {
			min = uint32(i)
			continue
		}
		a.media[k-1] = uint32(i) + min
	}
	if len(a.media) == 0 {
		err = ErrUnexpectedEOF
	}
	return
}

// HTTPGetter represents the mean to get data from HTTP.
type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

func (d *Data) UseHTTPClient(client HTTPGetter) *Data {
	d.http = client
	return d
}
