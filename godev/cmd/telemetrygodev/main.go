// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Telemetrygodev serves the telemetry.go.dev website.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"golang.org/x/telemetry/godev"
	"golang.org/x/telemetry/godev/internal/content"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/unionfs"
)

var (
	addr = flag.String("addr", ":8080", "server listens on the given TCP network address")
	dev  = flag.Bool("dev", false, "load static content and templates from the filesystem")
)

func main() {
	flag.Parse()
	s := content.Server(fsys(*dev))
	mw := middleware.Default
	fmt.Printf("server listening at http://%s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mw(s)))
}

func fsys(fromOS bool) fs.FS {
	var f fs.FS = godev.FS
	if fromOS {
		f = os.DirFS(".")
	}
	f, err := unionfs.Sub(f, "content/telemetrygodev", "content/shared")
	if err != nil {
		log.Fatal(err)
	}
	return f
}
