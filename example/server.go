// Copyright (c) 2018 Hervé Gouchet. All rights reserved.
// Use of this source code is governed by the MIT License
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/rvflash/combine"
)

var static = combine.NewBox("")

func landing(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello world")
}

func main() {
	http.Handle("/static/", static.FileServer(""))
	http.HandleFunc("/", landing)
	err := http.ListenAndServe(":6060", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
