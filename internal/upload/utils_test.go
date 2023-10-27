// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/telemetry"
	it "golang.org/x/telemetry/internal/telemetry"
)

// setup overwrites the system default telemetry configuration
// including the global variables defined in internal/telemetry.
// It also starts a test upload server.
func setup(t *testing.T, asof string) {
	asofTime, err := time.Parse("2006-01-02", asof)
	if err != nil {
		t.Fatalf("parsing asof time %q: %v", asof, err)
	}
	if serverChan == nil {
		// 10 is more uploads than a test will see
		serverChan = make(chan msg, 10)
		go testServer(serverChan)
		// wait for the server to start
		addr := <-serverChan
		serverURL = addr.path
		t.Logf("server started at %s", serverURL)

		logger = log.Default()
		logger.SetFlags(log.Lshortfile)
		dir := t.TempDir()
		it.LocalDir = dir + "/local"
		it.UploadDir = dir + "/upload"
		os.MkdirAll(it.LocalDir, 0777)
		os.MkdirAll(it.UploadDir, 0777)
		it.ModeFile = it.ModeFilePath(dir + "/mode")
		it.ModeFile.SetModeAsOf("on", asofTime)
		// set weekends?
	}
	// make sure they exist, in case the test cleanup removed them
	// but Open() still can't be called twice
	os.MkdirAll(it.LocalDir, 0777)
	os.MkdirAll(it.UploadDir, 0777)
}

func cleanDir(t *testing.T, test *Test, dir string) {
	fis, err := os.ReadDir(dir)
	if err != nil {
		msg := "nil test"
		if test != nil {
			msg = test.name
		}
		t.Errorf("couldn't clean dir for test %s (%v), %s", msg, err, dir)
	}
	for _, f := range fis {
		fname := filepath.Join(dir, f.Name())
		if err := os.Remove(fname); err != nil {
			t.Logf("%v removing %s", err, fname)
		}
	}
}

// copied from internal/counter/counter_test.go
func skipIfUnsupportedPlatform(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "openbsd", "js", "wasip1", "solaris", "android":
		// BUGS: #60614 - openbsd, #60967 - android , #60968 - solaris #60970 - solaris #60971 - wasip1)
		t.Skip("broken for openbsd etc")
	}
	if runtime.GOARCH == "386" {
		// BUGS: #60615 #60692 #60965 #60967
		t.Skip("broken for GOARCH 386")
	}
}

type msg struct {
	path   string
	length int
}

var (
	serverChan chan msg
	serverURL  string
)

// a test server. it is started once
func testServer(started chan msg) {
	log.SetFlags(log.Lshortfile)
	http.HandleFunc("/", handlerFunc)

	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	addr := ln.Addr().String()
	addr = "http://" + addr
	started <- msg{path: addr, length: len(addr)}
	log.Fatal(http.Serve(ln, nil))
}

func handlerFunc(w http.ResponseWriter, r *http.Request) {
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		log.Print(err)
		// set some sensible error code TODO(pjw): not teapot
		http.Error(w, "read failed", http.StatusTeapot)
	}
	serverChan <- msg{path: r.URL.Path, length: len(buf)}
}

func NewTestUploader(t *testing.T, cfg *telemetry.UploadConfig) *Uploader {
	u := NewUploader(cfg)
	if serverURL == "" {
		t.Fatal("testServer is not running yet")
	}
	u.UploadServerURL = serverURL
	return u
}
