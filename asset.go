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
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/js"
)

type asset struct {
	reg   *Box
	kind  string
	media []uint32
}

// Add ...
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

// AddFile ...
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

// AddString ...
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

// AddURL ...
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

// Combine ...
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

// String ...
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
	return hash
}

// Tag ...
func (a *asset) Tag(root Dir) string {
	var name string
	if name = a.String(); name == "" {
		return ""
	}
	// Path with src directory, a build version to force browser
	// to clear its cache, its filename and extension.
	link := path.Join("/", root.String(), a.reg.buildVersion, name)

	// HTML5 tag with the relative path of the asset.
	if a.kind == JavaScript {
		return `<script src="` + link + `.js"></script>`
	}
	return `<link rel="stylesheet" href="` + link + `.css">`
}
