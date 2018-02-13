// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/rvflash/combine"
)

func TestAsset_Add(t *testing.T) {
	// Creates the registry
	c := combine.New("", "")
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
	c := combine.New("./example/src", "")
	// Create an asset
	js := c.NewJS()
	var dt = []struct {
		in  string
		err error
	}{
		{in: "f1.js"},
		{in: "f33.js", err: errors.New(`stat example/src/f33.js: no such file or directory`)},
		{in: ".", err: combine.ErrUnexpectedEOF},
		{in: "", err: combine.ErrUnexpectedEOF},
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
	c := combine.New("", "")
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
	c := combine.New("", "")
	// Create an asset (any type).
	css := c.NewCSS()
	var dt = []struct {
		in  string
		err error
	}{
		{in: "http://rv.com/f1.css"},
		{in: "rv.com/f1.css"},
		{in: ":", err: errors.New(`parse :: missing protocol scheme`)},
		{in: "", err: combine.ErrUnexpectedEOF},
	}
	for i, tt := range dt {
		if err := css.AddURL(tt.in); err != nil {
			if err.Error() != tt.err.Error() {
				t.Fatalf("%d. error mismatch: got=%q, exp=%q", i, err, tt.err)
			}
		}
	}
}

func TestAsset_Combine(t *testing.T) {
	// Creates the registry
	c := combine.New("./example/src", "")
	css := c.NewCSS()
	if err := css.Add([]byte(".rv{color:#333;}")); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	w := &bytes.Buffer{}
	if err := css.Combine(w); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	exp := []byte(".rv{color:#333}")
	if got := w.Bytes(); !bytes.Equal(got, exp) {
		t.Fatalf("content mismatch: got=%q, exp=%q", got, exp)
	}
}

func TestAsset_Tag(t *testing.T) {
	// Creates the registry
	c := combine.New("", "")
	// Disables the build version to avoid variance.
	c.UseBuildVersion("")

	js := c.NewJS()
	if err := js.AddString("var a = 56;"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	css := c.NewCSS()
	if err := css.AddString(".black{color:#000;}"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	var dt = []struct {
		in  combine.File
		out string
	}{
		{in: c.NewCSS()},
		{in: js, out: `<script src="/2925958264.0.js"></script>`},
		{in: css, out: `<link rel="stylesheet" href="/3236089261.0.css">`},
	}
	for i, tt := range dt {
		if out := tt.in.Tag(""); out != tt.out {
			t.Fatalf("%d. extension mismatch: got=%q, exp=%q", i, out, tt.out)
		}
	}
}
