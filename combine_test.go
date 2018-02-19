// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rvflash/combine"
)

func ExampleBox_Open() {
	box := combine.NewBox("./example/src", "")
	defer func() { _ = box.Close() }()
	/// ...
	http.Handle("/", http.FileServer(box))
	http.ListenAndServe(":8080", nil)
}

func ExampleData_NewJS() {
	// Creates the registry
	c := combine.NewBox("./example/src", "")
	// Disables the build version to avoid variance.
	c.UseBuildVersion("")

	// Creates a new JS asset to combine various kind of media.
	js := c.NewJS()
	// Add files ...
	if err := js.AddFile("f1.js", "f2.js"); err != nil {
		fmt.Println("js: ", err)
		return
	}
	// Or one or more URL ...
	if err := js.AddURL("https://rv.com/f1.js"); err != nil {
		fmt.Println("js: ", err)
		return
	}
	// Or raw code as string ...
	if err := js.AddString(`alert("hey");`); err != nil {
		fmt.Println("js: ", err)
		return
	}
	// Or bytes
	if err := js.Add([]byte("var rv = 12")); err != nil {
		fmt.Println("js: ", err)
		return
	}
	// Gets the generate HTML5 tag to get a static version of this bulk.
	fmt.Println(js.Tag("/"))

	// Output: <script src="/883963153.0.1831620815.718850705.1931138922.3355474073.js"></script>
}

func TestNew(t *testing.T) {
	// Creates the registry
	c := combine.NewBox("./example/src", "./example/combine")
	defer func() { _ = c.Close() }()
	// Creates a HTTP test server.
	ts := httptest.NewServer(http.FileServer(c))
	defer ts.Close()

	js := c.NewJS()
	if err := js.AddString("var a = 56;"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var dt = []struct {
		path,
		body string
		statusCode int
	}{
		{body: "404 page not found\n", statusCode: 404},
		{path: "/2925958264.0.js", body: "var a=56;", statusCode: 200},
		{path: "/2925958264.0.js", body: "var a=56;", statusCode: 200},
	}
	for i, tt := range dt {
		resp, err := http.Get(ts.URL + tt.path)
		if err != nil {
			return
		}
		if resp.StatusCode != tt.statusCode {
			t.Errorf("%d . unexpected status code: got:%d exp:%d", i, resp.StatusCode, tt.statusCode)
		}
		out, _ := ioutil.ReadAll(resp.Body)
		if string(out) != tt.body {
			t.Errorf("%d . unexpected content: got:%q exp:%q", i, out, tt.body)
		}
		_ = resp.Body.Close()
	}

}
