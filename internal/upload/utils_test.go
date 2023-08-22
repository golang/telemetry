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
	"runtime"
	"testing"
	"time"

	"golang.org/x/telemetry"
	it "golang.org/x/telemetry/internal/telemetry"
)

func setup(t *testing.T) {
	if serverChan == nil {
		// 10 is more uploads than a test will see
		serverChan = make(chan msg, 10)
		go testServer(serverChan)
		// wait for the server to start
		addr := <-serverChan
		uploadURL = addr.path
		t.Logf("server started at %s", uploadURL)
	}
	logger = log.Default()
	logger.SetFlags(log.Lshortfile)
	dir := t.TempDir()
	it.LocalDir = dir + "/local"
	it.UploadDir = dir + "/upload"
	os.MkdirAll(it.LocalDir, 0777)
	os.MkdirAll(it.UploadDir, 0777)
	it.ModeFile = it.ModeFilePath(dir + "/mode")
	it.ModeFile.SetMode("on")
	it.SetMode(uploadURL)
}

func restore() {
	now = time.Now
}

func future(days int) func() time.Time {
	return func() time.Time {
		x := time.Duration(days)
		// make sure we're really x days in the future
		return time.Now().Add(x*24*time.Hour + 1*time.Second)
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

var serverChan chan msg

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

var testUploadConfig = &telemetry.UploadConfig{
	GOOS: []string{
		"android",
		"darwin",
		"dragonfly",
		"freebsd",
		"illumos",
		"js",
		"linux",
		"nacl",
		"netbsd",
		"openbsd",
		"plan9",
		"solaris",
		"windows",
	},
	GOARCH: []string{
		"386",
		"amd64",
		"amd64p32",
		"arm",
		"armbe",
		"arm64",
		"arm64be",
		"mips",
		"mipsle",
		"mips64",
		"mips64le",
		"mips64p32",
		"mips64p32le",
		"ppc64",
		"ppc64le",
		"riscv",
		"riscv64",
		"s390x",
		"sparc",
		"sparc64",
		"wasm",
	},
	GoVersion: []string{
		"go1.19",
		"go1.20",
		"go1.21",
		"go1.22",
	},
	Programs: []*telemetry.ProgramConfig{
		{
			Name: "debug.test",
			Counters: []telemetry.CounterConfig{
				{
					Name: "counter/main",
					Rate: 1.0,
				},
			},
		}, {
			Name: "upload.test",
			Counters: []telemetry.CounterConfig{
				{
					Name: "counter/main",
					Rate: 1.0,
				},
			},
		}, {
			Name: "upload.test-devel",
			Counters: []telemetry.CounterConfig{
				{
					Name: "counter/main",
					Rate: 1.0,
				},
			},
		},
		{
			Name: "test",
			Counters: []telemetry.CounterConfig{
				{
					Name: "counter/main",
					Rate: 1.0,
				},
			},
		},
	},
}
