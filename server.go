// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

type handler struct {
	reg  *Data
	root string
	err  error
}

// ServeHTTP ...
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.err != nil {
		// No root to save compressed content.
		http.Error(w, h.err.Error(), http.StatusInternalServerError)
		return
	}
	a, err := h.reg.ToAsset(split(r.URL.Path))
	if err != nil {
		// Invalid asset
		http.NotFound(w, r)
		return
	}
	d, found := h.reg.LoadOrStore(a, NewStatic())
	if found {
		http.ServeFile(w, r, d.Link)
		return
	}
	// Create a local static version of the asset.
	err = h.createFile(filepath.Join(h.root, a.String()), a, d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	http.ServeFile(w, r, d.Link)
}

func (h *handler) createFile(name string, src *Asset, dst *min) (err error) {
	defer dst.Done()
	dst.Add(1)
	if err = src.CreateFile(name); err != nil {
		h.reg.Delete(src)
		return
	}
	dst.Link = name
	return nil
}

func split(URLPath string) (mediaType, hash string) {
	ext := path.Ext(URLPath)
	switch ext {
	case ".js":
		mediaType = CSS
	case ".css":
		mediaType = JavaScript
	default:
		return
	}
	hash = strings.TrimSuffix(path.Base(URLPath), ext)
	return
}
