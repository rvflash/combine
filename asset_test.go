// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rvflash/combine"
)

func TestAsset_Add(t *testing.T) {
	// Creates the registry
	c := combine.New("")
	// Create an asset (any type)
	js := c.NewJS()
	var dt = []struct {
		in  []byte
		err error
	}{
		{in: nil},
		{in: []byte(`alert("hi!")`)},
	}
	for i, tt := range dt {
		if err := js.Add(tt.in); err != nil {
			t.Fatalf("%d. unexpected error: got=%q", i, err)
		}
	}
}

func TestAsset_AddFile(t *testing.T) {
	// Creates the registry
	c := combine.New("./example/src")
	// Create an asset
	js := c.NewJS()
	var dt = []struct {
		in  string
		err error
	}{
		{
			in: "f1.js",
		},
		{
			in:  "f33.js",
			err: errors.New(`stat example/src/f33.js: no such file or directory`),
		},
		{
			in:  ".",
			err: combine.ErrUnexpectedEOF,
		},
		{
			in:  "",
			err: combine.ErrUnexpectedEOF,
		},
	}
	for i, tt := range dt {
		if err := js.AddFile(tt.in); err != nil {
			if err.Error() != tt.err.Error() {
				t.Fatalf("%d. error mismatch: got=%q, exp=%q", i, err, tt.err)
			}
		}
	}
}

func TestAsset_AddString(t *testing.T) {
	// Creates the registry
	c := combine.New("")
	// Create an asset (any type)
	css := c.NewCSS()
	var dt = []struct {
		in  string
		err error
	}{
		{in: ""},
		{in: "body{display:none}"},
	}
	for i, tt := range dt {
		if err := css.AddString(tt.in); err != nil {
			t.Fatalf("%d. unexpected error: got=%q", i, err)
		}
	}
}

func TestAsset_AddURL(t *testing.T) {
	// Creates the registry
	c := combine.New("")
	// Create an asset (any type).
	css := c.NewCSS()
	var dt = []struct {
		in  string
		err error
	}{
		{
			in: "http://rv.com/f1.css",
		},
		{
			in: "rv.com/f1.css",
		},
		{
			in:  ":",
			err: errors.New(`parse :: missing protocol scheme`),
		},
		{
			in:  "",
			err: combine.ErrUnexpectedEOF,
		},
	}
	for i, tt := range dt {
		if err := css.AddURL(tt.in); err != nil {
			if err.Error() != tt.err.Error() {
				t.Fatalf("%d. error mismatch: got=%q, exp=%q", i, err, tt.err)
			}
		}
	}
}

func TestAsset_Ext(t *testing.T) {
	// Creates the registry
	c := combine.New("")
	var dt = []struct {
		in  *combine.Asset
		out string
	}{
		{in: c.NewCSS(), out: ".css"},
		{in: c.NewJS(), out: ".js"},
		{in: &combine.Asset{}},
	}
	for i, tt := range dt {
		if out := tt.in.Ext(); out != tt.out {
			t.Fatalf("%d. extension mismatch: got=%q, exp=%q", i, out, tt.out)
		}
	}
}

var errNoTransport = errors.New("no transport")

// Builds a fake http client by mocking main methods.
type fakeHTTPClient struct{}

// Get mocks the method of same name of the http package.
func (c *fakeHTTPClient) Get(url string) (*http.Response, error) {
	if !strings.HasPrefix(url, "http") {
		return nil, errNoTransport
	}
	// Mocks responses base on the URL.
	urlHandler := func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "" {
			p = "/"
		}
		switch p {
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"success":false,"error":404,"message":"Not Found"}`)
		}
	}
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	urlHandler(w, req)
	return w.Result(), nil
}
