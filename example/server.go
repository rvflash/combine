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

type homeHandler struct {
	static *combine.Box
}

func (p *homeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Just a sample of combined asset.
	css := p.static.NewCSS()
	_ = css.AddURL("https://raw.githubusercontent.com/twbs/bootstrap/v4-dev/dist/css/bootstrap-reboot.css")
	_ = css.AddString(".blue{ color: #4286f4; }")

	w.WriteHeader(200)
	fmt.Fprintf(w, `<!doctype HTML>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1, shrink-to-fit=no">
%s
<title>Combine sample</title>
<div>
	<h1 class="blue">Hello word</h1>
</div>
`, css.Tag("/min/"))
}

func main() {

	// Creates the box.
	static := combine.NewBox("./src", "./combine")
	defer func() { _ = static.Close() }()

	// Launches the HTTP server.
	http.Handle("/", &homeHandler{static})
	http.Handle("/min/", http.FileServer(static))
	if err := http.ListenAndServe(":6060", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
