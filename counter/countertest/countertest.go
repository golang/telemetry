// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.21

// require go1.21 due to the use of testing.Testing

// countertest provides testing utilities for counters.
// This package cannot be used except for testing.
package countertest

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/telemetry"
)

func init() {
	if !testing.Testing() {
		panic("use of this package is disallowed in non-testing code")
	}
}

var (
	openedMu sync.Mutex
	opened   bool
)

func isOpen() bool {
	openedMu.Lock()
	defer openedMu.Unlock()
	return opened
}

// Open enables telemetry data writing to disk.
// This is supposed to be called once during the program execution
// (i.e. typically in TestMain), and must not be used with
// golang.org/x/telemetry/counter.Open.
func Open(telemetryDir string) {
	openedMu.Lock()
	defer openedMu.Unlock()
	if opened {
		panic("Open was called more than once")
	}
	telemetry.ModeFile = telemetry.ModeFilePath(filepath.Join(telemetryDir, "mode"))
	telemetry.LocalDir = filepath.Join(telemetryDir, "local")
	telemetry.UploadDir = filepath.Join(telemetryDir, "upload")
	err1 := os.MkdirAll(telemetry.LocalDir, 0755)
	err2 := os.MkdirAll(telemetry.UploadDir, 0755)
	if err1 != nil || err2 != nil {
		panic(fmt.Sprintf("failed to create telemetry dirs: %v, %v", err1, err2))
	}

	counter.Open()
	opened = true
}

// ReadCounter reads the given counter.
func ReadCounter(c *counter.Counter) (count uint64, _ error) {
	if !isOpen() {
		return 0, fmt.Errorf("unmet requirement - Open must be called")
	}
	return counter.Read(c)
}

// ReadStackCounter reads the given StackCounter.
func ReadStackCounter(c *counter.StackCounter) (stackCounts map[string]uint64, _ error) {
	if !isOpen() {
		return nil, fmt.Errorf("unmet requirement - Open must be called")
	}
	return counter.ReadStack(c)
}
