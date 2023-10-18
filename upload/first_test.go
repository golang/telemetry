// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"log"
	"net/http"
	"strings"
	"testing"
)

// make sure we can talk to the test server
// In practice this test runs last, so is somewhat superfluous,
// but it also checks that uploads and reads from the channel are matched
func TestSimpleServer(t *testing.T) {
	setup(t, "2023-01-01")
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
