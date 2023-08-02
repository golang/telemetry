// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"log"
	"os"
	"testing"
	"time"

	xt "golang.org/x/telemetry"
	"golang.org/x/telemetry/internal/counter"
	it "golang.org/x/telemetry/internal/telemetry"
)

func setup(t *testing.T) {
	log.SetFlags(log.Lshortfile)
	dir := t.TempDir()
	it.LocalDir = dir + "/local"
	it.UploadDir = dir + "/upload"
	os.MkdirAll(it.LocalDir, 0777)
	os.MkdirAll(it.UploadDir, 0777)
	xt.LocalDir = it.LocalDir
	xt.UploadDir = it.UploadDir
	it.ModeFile = it.ModeFilePath(dir + "/mode")
	uploadURL := "http://localhost:3131"
	it.ModeFile.SetMode(uploadURL)
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

func TestZero(t *testing.T) {
	setup(t)
	defer restore()
	now = future(0)
	c := counter.New("testing")
	defer c.AllDone()
	c.Inc()
	counter.Open() // needed for tests
	now = future(15)
	work := findWork(it.LocalDir, it.UploadDir)
	// expect one count file and nothing else
	if len(work.countfiles) != 1 {
		t.Errorf("expected one countfile, got %d", len(work.countfiles))
	}
	if len(work.readyfiles) != 0 {
		t.Errorf("expected no readufiles, got %d", len(work.readyfiles))
	}
	if len(work.uploaded) != 0 {
		t.Errorf("expected no uploadedfiles, got %d", len(work.uploaded))
	}
}
