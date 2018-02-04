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
	media map[int]uint32
}

func (a *Asset) append(r *raw) error {
	id, err := r.crc()
	if err != nil {
		return err
	}
	a.reg.storeRawSrc(id, r)
	a.media[len(a.media)] = id
	return nil
}

// Add ...
func (a *Asset) Add(b []byte) error {
	c := &raw{
		kind:   inlineSrc,
		reader: bytes.NewReader(b),
	}
	return a.append(c)
}

// AddFile ...
func (a *Asset) AddFile(name string, more ...string) (err error) {
	for _, name := range prepend(name, more) {
		name = filepath.Join(a.reg.root.String(), name)
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
	c := &raw{
		kind:   inlineSrc,
		reader: strings.NewReader(s),
	}
	return a.append(c)
}

// AddURL ...
func (a *Asset) AddURL(rawURL string, more ...string) error {
	for _, rawURL := range prepend(rawURL, more) {
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

func prepend(one string, more []string) []string {
	return append([]string{one}, more...)
}

// Combine ...
func (a *Asset) Combine(w io.Writer) error {
	m := minify.New()
	for _, id := range a.media {
		r, ok := a.reg.loadRawSrc(id)
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
	path, err := r.readAll()
	if err != nil {
		return err
	}
	resp, err := a.reg.http.Get(path)
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
	var min uint32
	for _, h := range a.media {
		if min == 0 || min > h {
			min = h
		}
	}
	var fUint32 = func(i uint32) string {
		return strconv.FormatUint(uint64(i), 10)
	}
	var hash string
	for k, h := range a.media {
		if k == 0 {
			hash = fUint32(min)
		}
		hash += "." + fUint32(h-min)
	}
	return hash
}

// Tag ...
func (a *Asset) Tag(root string) string {
	if len(a.media) == 0 {
		return ""
	}
	if root != "" {
		root = strings.TrimSuffix(root, "/")
	}
	switch a.kind {
	case JavaScript:
		return `<script src="` + root + `/` + a.String() + a.Ext() + `"></script>`
	case CSS:
		return `<link rel="stylesheet" href="` + root + `/` + a.String() + a.Ext() + `">`
	default:
		return ""
	}
}
