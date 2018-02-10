// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine_test

import (
	"fmt"

	"github.com/rvflash/combine"
)

func ExampleData_NewJS() {
	// Creates the registry
	c := combine.New("./example/src")
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
