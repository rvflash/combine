// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/js"
)

type asset struct {
	reg   *Box
	kind  string
	media []uint32
}

// StringCombiner ...
type StringCombiner interface {
	fmt.Stringer
	Combiner
}

// File ...
type File interface {
	Aggregator
	Tagger
	StringCombiner
}

// Aggregator is the interface implemented by asset to add content inside.
type Aggregator interface {
	// Add adds a slice of byte as part of the asset.
	// An error is returned if we fails to deal with it.
	Add(buf ...[]byte) error
	// AddFile stores the file names as future part of the asset.
	// Only checks stats to verify if it exists.
	// If not, an error is returned.
	AddFile(name ...string) error
	// AddString adds each string as part of the asset.
	// An error is returned if we fails to deal with it.
	AddString(str ...string) error
	// AddURL stores the file URLs as future part of the asset.
	// An error is returned is one URL is invalid.
	AddURL(url ...string) error
}

// Add adds a slice of byte as part of the asset.
// An error is returned if we fails to deal with it.
func (a *asset) Add(buf ...[]byte) (err error) {
	for _, buf := range buf {
		if len(buf) == 0 {
			continue
		}
		c := &raw{kind: inlineSrc, buf: buf}
		if err = a.append(c); err != nil {
			return
		}
	}
	return
}

// AddFile stores the file names as future part of the asset.
// Only checks stats to verify if it exists.
// If not, an error is returned.
func (a *asset) AddFile(name ...string) (err error) {
	for _, name := range name {
		file := Dir(name)
		if file.String() == "." {
			return ErrUnexpectedEOF
		}
		name = filepath.Join(a.reg.src.String(), file.String())
		if _, err = os.Stat(name); err != nil {
			return
		}
		c := &raw{kind: fileSrc, buf: []byte(name)}
		if err = a.append(c); err != nil {
			return
		}
	}
	return
}

// AddString adds each string as part of the asset.
// An error is returned if we fails to deal with it.
func (a *asset) AddString(s ...string) (err error) {
	for _, s := range s {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		c := &raw{kind: inlineSrc, buf: []byte(s)}
		if err = a.append(c); err != nil {
			return
		}
	}
	return
}

// AddURL stores the file URLs as future part of the asset.
// An error is returned is one URL is invalid.
func (a *asset) AddURL(rawURL ...string) error {
	for _, rawURL := range rawURL {
		if rawURL = strings.TrimSpace(rawURL); rawURL == "" {
			return ErrUnexpectedEOF
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		c := &raw{kind: onlineSrc, buf: []byte(u.String())}
		if err = a.append(c); err != nil {
			return err
		}
	}
	return nil
}

func (a *asset) append(r *raw) (err error) {
	var key uint32
	if key, err = r.crc(); err != nil {
		return err
	}
	a.reg.storeRaw(key, r)
	a.media = append(a.media, key)
	return
}

// Combiner must be implement to combine minified contents.
type Combiner interface {
	// Combine tries to write the result of all combined and minified
	// parts of the content of the asset to w or returns an error.
	Combine(w io.Writer) error
}

// Combine tries to write the result of all combined and minified
// parts of the content of the asset to w or returns an error.
func (a *asset) Combine(w io.Writer) error {
	m, err := newMinify(a.kind)
	if err != nil {
		return err
	}
	for i := 0; i < len(a.media); i++ {
		r, ok := a.reg.loadRaw(a.media[i])
		if !ok {
			return ErrNotFound
		}
		switch r.kind {
		case fileSrc:
			if err := a.minifyFile(r, m, w); err != nil {
				return errors.Wrap(ErrNotFound, err.Error())
			}
		case onlineSrc:
			if err := a.minifyURL(r, m, w); err != nil {
				return errors.Wrap(ErrNotFound, err.Error())
			}
		default:
			if err := a.minify(r, m, w); err != nil {
				return errors.Wrap(ErrNotFound, err.Error())
			}
		}
	}
	return nil
}

func newMinify(mimeType string) (m *minify.M, err error) {
	m = minify.New()
	switch mimeType {
	case JavaScript:
		m.AddFunc(mimeType, js.Minify)
	case CSS:
		m.AddFunc(mimeType, css.Minify)
	default:
		err = ErrMime
	}
	return
}

func (a *asset) minify(r *raw, m *minify.M, w io.Writer) error {
	return m.Minify(a.kind, w, bytes.NewReader(r.buf))
}

func (a *asset) minifyFile(r *raw, m *minify.M, w io.Writer) error {
	file, err := os.Open(r.String())
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return m.Minify(a.kind, w, file)
}

func (a *asset) minifyURL(r *raw, m *minify.M, w io.Writer) error {
	resp, err := a.reg.http.Get(r.String())
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return ErrNotFound
	}
	return m.Minify(a.kind, w, resp.Body)
}

// String implements the fmt.Stinger interface.
func (a *asset) String() string {
	if len(a.media) == 0 {
		return ""
	}
	var fUint32 = func(i uint32) string {
		return strconv.FormatUint(uint64(i), 10)
	}
	// Searches the smallest key.
	var min uint32
	for _, key := range a.media {
		if min == 0 || min > key {
			min = key
		}
	}
	// Builds the hast to identify its asset.
	var hash string
	for i := 0; i < len(a.media); i++ {
		if i == 0 {
			hash = fUint32(min)
		}
		hash += "." + fUint32(a.media[i]-min)
	}
	if a.kind == JavaScript {
		return hash + ".js"
	}
	return hash + ".css"
}

// Tagger must be implemented by an asset to be used in HTML5.
type Tagger interface {
	// Link returns the Link HTTP response header to preload the asset.
	Link(root Dir) string
	// Path returns the relative path to the asset
	Path(root Dir) string
	// Tag returns the tag to link to the minified and combined version of the asset.
	Tag(root Dir) string
	// SrcTags returns all original resources in HTML5 tags.
	SrcTags(root Dir, stripPrefix ...string) string
}

// Link returns the Link HTTP response header to preload the asset.
func (a *asset) Link(root Dir) string {
	if a.kind == JavaScript {
		return "<" + a.Path(root) + ">; rel=preload; as=script"
	}
	return "<" + a.Path(root) + ">; rel=preload; as=style"
}

// Path returns the relative path to the asset including the root directory
// and the current build version as "folder".
// The build version is dedicated to force browser to clear its cache.
// After this prefixes, comes the file name and the extension.
func (a *asset) Path(root Dir) string {
	name := a.String()
	if name == "" {
		return ""
	}
	return path.Join("/", filePathToPath(root.String()), a.reg.buildVersion, name)
}

// SrcTags returns all original resources in HTML5 tags.
// It purposes only to be used on a development server.
// The root directory allow to uses an other static handler.
// The list of optional stripPrefix offers means to remove
// the given prefixes in the asset file path.
func (a *asset) SrcTags(root Dir, stripPrefix ...string) string {
	var (
		s    string
		src  *raw
		tags []string
	)
	a.reg.raw.RLock()
	for _, key := range a.media {
		src = a.reg.raw.src[key]
		s = string(src.buf)
		if src.kind == fileSrc {
			// Local link to the resource. We need to manage access
			// to this static with possibly an other relative path.
			s = filePathToPath(s)
			for _, prefix := range stripPrefix {
				s = strings.TrimPrefix(s, filePathToPath(prefix))
			}
			s = path.Join("/", s)
		}
		tags = append(tags, htmlTag(a.kind, s, src.kind == inlineSrc))
	}
	a.reg.raw.RUnlock()
	return strings.Join(tags, "\n")
}

// Converts a file path to an URL path.
func filePathToPath(s string) string {
	if filepath.Separator == '/' {
		// nothing to do, it's the same separator in an URL.
		return s
	}
	return strings.Join(strings.Split(s, string(filepath.Separator)), "/")
}

// Tag returns a HTML5 tag to link to the minified and combined version of the asset.
func (a *asset) Tag(root Dir) string {
	s := a.Path(root)
	if s == "" {
		return ""
	}
	return htmlTag(a.kind, s, false)
}

func htmlTag(kind, text string, inline bool) string {
	// Returns a HTML5 JS tag.
	if kind == JavaScript {
		if inline {
			return `<script>` + text + `</script>`
		}
		return `<script src="` + text + `"></script>`
	}
	// Returns a HTML5 CSS tag.
	if inline {
		return `<style>` + text + `"</style>`
	}
	return `<link rel="stylesheet" href="` + text + `">`
}
