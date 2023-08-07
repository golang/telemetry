// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	it "golang.org/x/telemetry/internal/telemetry"
)

//var uploadURL = "https://exp-telemetry-l2wtsklj5q-uc.a.run.app/upload"

// default for mode 'on'
var uploadURL = "https://telemetry.go.dev/upload"

func uploadReport(fname string) {
	buf, err := os.ReadFile(fname)
	if err != nil {
		log.Printf("%v reading %s", err, fname)
		return
	}
	if uploadReportContents(fname, buf) {
		// anything left to do?
	}
}

// try to upload the report, 'true' if successful
func uploadReportContents(fname string, buf []byte) bool {
	b := bytes.NewReader(buf)
	fdate := strings.TrimSuffix(filepath.Base(fname), ".json")
	fdate = fdate[len(fdate)-len("2006-01-02"):]
	server := uploadURL + "/" + fdate
	var client *http.Client
	// this is temporary until certificates propagate (we hope)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client = &http.Client{}

	resp, err := client.Post(server, "application/json", b)
	if err != nil {
		log.Printf("error on Post: %v %q for %q", err, server, fname)
		return false
	}
	if resp.StatusCode != 200 {
		log.Printf("resp error on upload %q: %v for %q %q", server, resp.Status, fname, fdate)
		return false
	}
	// put a copy in the uploaded directory
	newname := filepath.Join(it.UploadDir, fdate+".json")
	if err := os.WriteFile(newname, buf, 0644); err == nil {
		os.Remove(fname) // if it exists
	}
	return true
}
