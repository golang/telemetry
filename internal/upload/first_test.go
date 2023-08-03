// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"os"
	"testing"

	"golang.org/x/telemetry/internal/counter"
	it "golang.org/x/telemetry/internal/telemetry"
)

func TestZero(t *testing.T) {
	skipIfUnsupportedPlatform(t)
	setup(t)
	defer restore()
	now = future(0)
	finished := counter.Open()
	c := counter.New("testing")

	c.Inc()
	now = future(15)
	work := findWork(it.LocalDir, it.UploadDir)
	// expect one count file and nothing else
	if len(work.countfiles) != 1 {
		t.Errorf("expected one countfile, got %d", len(work.countfiles))
	}
	if len(work.readyfiles) != 0 {
		t.Errorf("expected no readyfiles, got %d", len(work.readyfiles))
	}
	if len(work.uploaded) != 0 {
		t.Errorf("expected no uploadedfiles, got %d", len(work.uploaded))
	}

	// Windows:
	// reports will not be able to remove the count file if it is still open
	// (in non-test situations it would have been rotated out and closed)
	finished()

	// generate reports
	uploadConfig = testUploadConfig
	if err := reports(work); err != nil {
		t.Fatal(err)
	}
	// expect a single report and nothing else
	got := findWork(it.LocalDir, it.UploadDir)
	if len(got.countfiles) != 0 {
		t.Errorf("expected no countfiles, got %d", len(got.countfiles))
	}
	if len(got.readyfiles) != 1 {
		// the uploadable report
		t.Errorf("expected one readyfile, got %d", len(got.readyfiles))
	}
	fi, err := os.ReadDir(it.LocalDir)
	if len(fi) != 2 || err != nil {
		// one local report and one uploadable report
		t.Errorf("expected two files in LocalDir, got %d, %v", len(fi), err)
	}
	if len(got.uploaded) != 0 {
		t.Errorf("expected no uploadedfiles, got %d", len(got.uploaded))
	}
	// check contents
}
