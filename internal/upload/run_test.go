// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"log"
	"testing"

	"golang.org/x/telemetry/internal/counter"
)

// this sends an empty valid report to the real server
func TestRun(t *testing.T) {
	// don't run this test on the builders
	// don't run this test with other tests
	t.Skip("for manual testing only")

	log.SetFlags(log.Lshortfile)
	finished := counter.Open()
	c := counter.New("testing")

	c.Inc()
	thisInstant = future(15)
	finished() // for Windows

	Run(nil)
	log.Printf("finished")
}
