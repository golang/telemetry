// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"log"

	"golang.org/x/telemetry"
)

// Run generates and uploads reports
// TODO(pjw): decide what to do about error reporting throughout the package
func Run() {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("upload: %v", err)
		}
	}()
	todo := findWork(telemetry.LocalDir, telemetry.UploadDir)
	if err := reports(todo); err != nil {
		log.Printf("reports: %v", err)
	}
	for _, f := range todo.readyfiles {
		uploadReport(f)
	}
}
