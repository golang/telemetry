// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	it "golang.org/x/telemetry/internal/telemetry"
)

// default for mode 'on'. Overridden in tests.
var uploadURL = "https://telemetry.go.dev/upload"

var dateRE = regexp.MustCompile(`(\d\d\d\d-\d\d-\d\d)[.]json$`)

func uploadReport(fname string) {
	// first make sure it is not in the future
	today := thisInstant.Format("2006-01-02")
	match := dateRE.FindStringSubmatch(fname)
	if match == nil || len(match) < 2 {
		logger.Printf("report name seemed to have no date %q", filepath.Base(fname))
	} else if match[1] > today {
		logger.Printf("report %q is later than today %s", filepath.Base(fname), today)
		return // report is in the future, which shouldn't happen
	}
	buf, err := os.ReadFile(fname)
	if err != nil {
		logger.Printf("%v reading %s", err, fname)
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
		logger.Printf("error on Post: %v %q for %q", err, server, fname)
		return false
	}
	if resp.StatusCode != 200 {
		logger.Printf("resp error on upload %q: %v for %q %q [%+v]", server, resp.Status, fname, fdate, resp)
		return false
	}
	// put a copy in the uploaded directory
	newname := filepath.Join(it.UploadDir, fdate+".json")
	if err := os.WriteFile(newname, buf, 0644); err == nil {
		os.Remove(fname) // if it exists
	}
	return true
}
