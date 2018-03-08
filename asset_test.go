// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine_test

import (
	"bytes"
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
	c := combine.NewBox("", "")
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
	c := combine.NewBox("./example/src", "")
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
	c := combine.NewBox("", "")
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
	c := combine.NewBox("", "")
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
	c := combine.NewBox("./example/src", "")
	// Mocks the HTTP client.
	c.UseHTTPClient(&fakeHTTPClient{})
	// Tests it.
	var dt = []struct {
		file     combine.File
		buf      [][]byte
		str      []string
		filePath []string
		urlPath  []string
		exp      []byte
		err      error
	}{
		{
			file:     c.NewCSS(),
			buf:      [][]byte{[]byte(`.rv{color:#333;}`)},
			str:      []string{`.hide{display:none;}`},
			filePath: []string{"f1.css"},
			urlPath:  []string{"http://www.css.com/f1.css"},
			exp:      []byte(`.rv{color:#333}.hide{display:none}.show{display:block}.red{color:red}`),
		},
		{
			file:     c.NewJS(),
			buf:      [][]byte{[]byte(`function rv(a){ alert(a);}`)},
			str:      []string{`rv("hi");`},
			filePath: []string{"f1.js"},
			urlPath:  []string{"https://www.js.com/f1.js"},
			exp: []byte(`function rv(a){alert(a);}rv("hi");function hey(){alert("hello word");}
window.onload=hey;document.location="/home.html";`),
		},
		{
			file:    c.NewCSS(),
			urlPath: []string{"http://www.css.com/fail.css"},
			err:     combine.ErrNotFound,
		},
	}
	w := &bytes.Buffer{}
	for i, tt := range dt {
		if err := tt.file.Add(tt.buf...); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tt.file.AddString(tt.str...); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tt.file.AddFile(tt.filePath...); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tt.file.AddURL(tt.urlPath...); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		w.Reset()
		if err := tt.file.Combine(w); err != nil {
			if !strings.Contains(err.Error(), tt.err.Error()) {
				t.Errorf("%d. unexpected error: \ngot=%s\nexp=%q", i, err, tt.err)
			}
		}
		if got := w.Bytes(); !bytes.Equal(got, tt.exp) {
			t.Errorf("%d. content mismatch: \ngot=%q\nexp=%q", i, got, tt.exp)
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
		switch r.URL.Path {
		case "/f1.css":
			_, _ = io.WriteString(w, `
/* an other comment */
.red{
	color:#f00;
}`)
		case "/f1.js":
			_, _ = io.WriteString(w, `
// just do it!
document.location = "/home.html";
`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	req := httptest.NewRequest("GET", url, nil)
	w := httptest.NewRecorder()
	urlHandler(w, req)
	return w.Result(), nil
}

func TestAsset_Tag(t *testing.T) {
	// Creates the registry
	c := combine.NewBox("", "")
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

func TestAsset_SrcTags(t *testing.T) {
	// Creates the registry
	c := combine.NewBox("./example/src", "")
	// Disables the build version to avoid variance.
	c.UseBuildVersion("")

	js := c.NewJS()
	if err := js.AddFile("f1.js", "f2.js"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	exp := `<script src="/f1.js"></script>
<script src="/f2.js"></script>`
	if out := js.SrcTags("/", "example/src"); out != exp {
		t.Errorf("mismatch content: got:%q exp:%q", out, exp)
	}
}
