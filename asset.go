// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine

import (
	"bytes"
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
)

// Asset ...
type Asset struct {
	reg   *Data
	kind  string
	media []uint32
}

// Add ...
func (a *Asset) Add(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	c := &raw{
		kind:   inlineSrc,
		reader: bytes.NewReader(b),
	}
	return a.append(c)
}

// AddFile ...
func (a *Asset) AddFile(name string, more ...string) (err error) {
	for _, name := range prepend(name, more) {
		file := Dir(name)
		if file.String() == "." {
			return ErrUnexpectedEOF
		}
		name = filepath.Join(a.reg.root.String(), file.String())
		if _, err = os.Stat(name); err != nil {
			return
		}
		c := &raw{
			kind:   fileSrc,
			reader: strings.NewReader(name),
		}
		if err = a.append(c); err != nil {
			return
		}
	}
	return
}

// AddString ...
func (a *Asset) AddString(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	c := &raw{
		kind:   inlineSrc,
		reader: strings.NewReader(s),
	}
	return a.append(c)
}

// AddURL ...
func (a *Asset) AddURL(rawURL string, more ...string) error {
	for _, rawURL := range prepend(rawURL, more) {
		if strings.TrimSpace(rawURL) == "" {
			return ErrUnexpectedEOF
		}
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		c := &raw{
			kind:   onlineSrc,
			reader: strings.NewReader(u.String()),
		}
		if err = a.append(c); err != nil {
			return err
		}
	}
	return nil
}

func (a *Asset) append(r *raw) (err error) {
	var key uint32
	if key, err = r.crc(); err != nil {
		return err
	}
	a.reg.storeRaw(key, r)
	a.media = append(a.media, key)
	return
}

func prepend(one string, more []string) []string {
	return append([]string{one}, more...)
}

// Combine ...
func (a *Asset) Combine(w io.Writer) error {
	m := minify.New()
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

func (a *Asset) minify(r *raw, m *minify.M, w io.Writer) error {
	return m.Minify(a.kind, w, r.reader)
}

func (a *Asset) minifyFile(r *raw, m *minify.M, w io.Writer) error {
	name, err := r.readAll()
	if err != nil {
		return err
	}
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	return m.Minify(a.kind, w, file)
}

func (a *Asset) minifyURL(r *raw, m *minify.M, w io.Writer) error {
	name, err := r.readAll()
	if err != nil {
		return err
	}
	resp, err := a.reg.http.Get(name)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return ErrNotFound
	}
	return m.Minify(a.kind, w, resp.Body)
}

// CreateFile ...
func (a *Asset) CreateFile(name string) (err error) {
	f, err := os.Create(name)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	return a.Combine(f)
}

// Ext ...
func (a *Asset) Ext() string {
	switch a.kind {
	case JavaScript:
		return ".js"
	case CSS:
		return ".css"
	default:
		return ""
	}
}

// ID ...
func (a *Asset) ID() uint32 {
	buf := []byte(a.String())
	if len(buf) == 0 {
		// No content.
		return 0
	}
	id, err := crc32(buf)
	if err != nil {
		// Really ?
		return 0
	}
	return id
}

// String ...
func (a *Asset) String() string {
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
	return hash
}

// Tag ...
func (a *Asset) Tag(root string) string {
	if len(a.media) == 0 {
		return ""
	}

	// Path with root directory, a build version to force browser
	// to clear its cache and its filename with extension.
	link := path.Join("/", root, a.reg.buildVersion, a.String()+a.Ext())

	// HTML5 tag with the relative path of the asset.
	switch a.kind {
	case JavaScript:
		return `<script src="` + link + `"></script>`
	case CSS:
		return `<link rel="stylesheet" href="` + link + `">`
	default:
		return ""
	}
}
