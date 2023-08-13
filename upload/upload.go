// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"log"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/internal/upload"
)

// Run generates and uploads reports, as allowed by the mode file.
// A nil Control is legal and uses the latest version of the module
// golang.org/x/telemetry/config and the derault upload URL
// (presently https://telemetry.go.dev/upload).
func Run(c *telemetry.Control) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("upload recover: %v", err)
		}
	}()
	upload.Run(c)
}
