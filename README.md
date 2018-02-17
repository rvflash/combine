# Combine

[![GoDoc](https://godoc.org/github.com/rvflash/combine?status.svg)](https://godoc.org/github.com/rvflash/combine)
[![Build Status](https://img.shields.io/travis/rvflash/combine.svg)](https://travis-ci.org/rvflash/combine)
[![Code Coverage](https://img.shields.io/codecov/c/github/rvflash/combine.svg)](http://codecov.io/github/rvflash/combine?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/rvflash/combine)](https://goreportcard.com/report/github.com/rvflash/combine)


Package combine provides interface to create assets with multiple source of contents (string, file path or URL).
It combines them after minify it on the fly and also provides methods to launch a file server to serve them.


### Installation

```bash
$ go get github.com/rvflash/combine
```

### Usage


```go
import "github.com/rvflash/combine"
// ...
box := combine.NewBox("./example/src", "")
// ...
http.Handle("/", http.FileServer(box))
http.ListenAndServe(":8080", nil)
```