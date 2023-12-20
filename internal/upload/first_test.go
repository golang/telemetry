// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// make sure we can talk to the test server
// In practice this test runs last, so is somewhat superfluous,
// but it also checks that uploads and reads from the channel are matched
func TestSimpleServer(t *testing.T) {
	srv, uploaded := createTestUploadServer(t)
	defer srv.Close()

	url := srv.URL
	resp, err := http.Post(url, "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%#v", resp.StatusCode)
	}
	got := uploaded()
	want := [][]byte{[]byte("hello")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %s, want %s", got, want)
	}
}
