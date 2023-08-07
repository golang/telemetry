// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/telemetry/internal/counter"
	it "golang.org/x/telemetry/internal/telemetry"
)

// make sure we can talk to the test server
func TestSimpleServer(t *testing.T) {
	setup(t)
	defer restore()
	url := uploadURL
	resp, err := http.Post(url+"/foo", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("%#v", resp.StatusCode)
	}
	got := <-serverChan
	if got != (msg{"/foo", 5}) {
		t.Errorf("got %v", got)
	}
}
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
	// check contents. The semantic difference is "testing:1" in the
	// local file, but the json has some extra commas.
	var localFile, uploadFile []byte
	for _, f := range fi {
		fname := filepath.Join(it.LocalDir, f.Name())
		buf, err := os.ReadFile(fname)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(f.Name(), "local") {
			localFile = buf
		} else {
			uploadFile = buf
		}
	}
	want := regexp.MustCompile("(?s:,. *\"testing\": 1)")
	found := want.FindSubmatchIndex(localFile)
	if len(found) != 2 {
		t.Fatalf("expected to find %q in %q", want, localFile)
	}
	if string(uploadFile) != string(localFile[:found[0]])+string(localFile[found[1]:]) {
		t.Fatalf("got\n%q expected\n%q", uploadFile,
			string(localFile[:found[0]])+string(localFile[found[1]:]))
	}
	// and try uploading to the test
	uploadReport(got.readyfiles[0])
	x := <-serverChan
	if x.length != len(uploadFile) {
		t.Errorf("%v %d", x, len(uploadFile))
	}
}
