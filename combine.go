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
	return filepath.Clean(dir)
}

// Data ...
type Data struct {
	raw          *rawMap
	min          *minMap
	root         Dir
	http         HTTPGetter
	buildVersion string
}

// New ...
func New(root Dir) *Data {
	return &Data{
		raw:          &rawMap{src: make(map[uint32]*raw)},
		min:          &minMap{src: make(map[uint32]*Static)},
		root:         root,
		http:         newHTTPClient(),
		buildVersion: strconv.FormatInt(time.Now().Unix(), 10),
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

type minMap struct {
	src map[uint32]*Static
	sync.Mutex
}

// NewStatic ...
func NewStatic() *Static {
	return &Static{}
}

// Static ...
type Static struct {
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

func (d *Data) loadRaw(key uint32) (value *raw, ok bool) {
	defer d.raw.Unlock()
	d.raw.Lock()
	if value, ok = d.raw.src[key]; ok {
		return
	}
	return nil, false
}

func (d *Data) storeRaw(key uint32, value *raw) {
	d.raw.Lock()
	d.raw.src[key] = value
	d.raw.Unlock()
}

// NewCSS ...
func (d *Data) NewCSS() *Asset {
	return &Asset{
		kind:  CSS,
		media: make([]uint32, 0),
		reg:   d,
	}
}

// NewJS ...
func (d *Data) NewJS() *Asset {
	return &Asset{
		kind:  JavaScript,
		media: make([]uint32, 0),
		reg:   d,
	}
}

// FileServer implements the http.handler.
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
func (d *Data) Load(key *Asset) (value *Static, ok bool) {
	defer d.min.Unlock()
	d.min.Lock()
	if value, ok = d.min.src[key.ID()]; ok {
		return
	}
	return nil, false
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (d *Data) LoadOrStore(key *Asset, value *Static) (actual *Static, loaded bool) {
	actual, loaded = d.Load(key)
	if loaded {
		actual.Wait()
		return
	}
	return value, false
}

// Store sets the path for the given identifier.
func (d *Data) Store(key *Asset, value *Static) {
	d.min.Lock()
	d.min.src[key.ID()] = value
	d.min.Unlock()
}

// ToAsset...
func (d *Data) ToAsset(mediaType, hash string) (a *Asset, err error) {
	// Checksum
	keys := strings.Split(toHash(hash, a.Ext()), ".")
	if len(keys)-1 < 1 {
		err = ErrUnexpectedEOF
		return
	}
	// Initialization by king of media
	switch mediaType {
	case CSS:
		a = d.NewCSS()
	case JavaScript:
		a = d.NewJS()
	default:
		err = ErrMime
		return
	}
	// Defines the number of media inside
	a.media = make([]uint32, len(keys)-1)
	// Extracts media keys behind it.
	var i uint64
	var min uint32
	for k, v := range keys {
		if i, err = strconv.ParseUint(v, 10, 32); err != nil {
			return
		}
		if k == 0 {
			min = uint32(i)
			continue
		}
		a.media[k-1] = uint32(i) + min
	}
	return
}

// UseBuildVersion ...
func (d *Data) UseBuildVersion(value string) *Data {
	d.buildVersion = value
	return d
}

// HTTPGetter represents the mean to get data from HTTP.
type HTTPGetter interface {
	Get(url string) (*http.Response, error)
}

// UseHTTPClient ...
func (d *Data) UseHTTPClient(client HTTPGetter) *Data {
	d.http = client
	return d
}
