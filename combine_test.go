// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package combine_test

import (
	"fmt"

	"github.com/rvflash/combine"
)

var reg = combine.New()

func ExampleData_NewJS() {
	//  dir, err := os.Getwd()
	js := reg.NewJS()
	err := js.AddFile("/var/www/static/js/home.js")
	if err != nil {
		fmt.Println(err)
	}
	_ = js.AddString("if (false) { document.location = rv; }")
	_ = js.Add([]byte("var rv = 12"))
	fmt.Println(js.Tag(""))
	// Output: 2166136261.0.907789902.1115489359.2073300965
}
