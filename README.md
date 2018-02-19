# Combine

[![GoDoc](https://godoc.org/github.com/rvflash/combine?status.svg)](https://godoc.org/github.com/rvflash/combine)
[![Build Status](https://img.shields.io/travis/rvflash/combine.svg)](https://travis-ci.org/rvflash/combine)
[![Code Coverage](https://img.shields.io/codecov/c/github/rvflash/combine.svg)](http://codecov.io/github/rvflash/combine?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/rvflash/combine)](https://goreportcard.com/report/github.com/rvflash/combine)


Package combine provides interface to create assets with multiple source of contents (bytes, string, file path or URL).
It combines and minifies them on the fly on the first demand on the dedicated file server.


### Installation

```bash
$ go get github.com/rvflash/combine
```

### Usage

See example for real usage. Errors are ignored for the demo.

```go
import "github.com/rvflash/combine"
// ...
// Creates a box with the path to the local file resources
// and the path to store combined / minified assets.
box := combine.NewBox("./src", "./combine")
// Deletes all files cache on exit. 
defer func() { _ = box.Close() }()
// ...
// Creates a asset.
css := static.NewCSS()
_ = css.AddURL("https://raw.githubusercontent.com/twbs/bootstrap/v4-dev/dist/css/bootstrap-reboot.css")
_ = css.AddString(".blue{ color: #4286f4; }")
_ = css.AddFile("local/file/is_src_dir.css")
// Uses it in a HTML template by retrieving its path or tag.
// By default, a build version will also added.
tag := css.Tag("/static/")
// ...
// Serves combined and minifed resousrces
http.Handle("/static/", http.FileServer(box))
http.ListenAndServe(":8080", nil)
```