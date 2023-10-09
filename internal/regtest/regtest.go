// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.21

// Package regtest provides helpers for end-to-end testing
// involving counter and upload packages. This package requires go1.21 or newer.
package regtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"

	"golang.org/x/telemetry/counter/countertest"
)

const telemetryDirEnvVar = "_COUNTERTEST_RUN_TELEMETRY_DIR"

var (
	runProgMu  sync.Mutex
	hasRunProg = map[string]bool{} // test name -> RunProg already called for this test.
)

func canRunProg(name string) bool {
	runProgMu.Lock()
	defer runProgMu.Unlock()
	if hasRunProg[name] {
		return false
	}
	hasRunProg[name] = true
	return true
}

// RunProg runs prog in a separate process with the specified telemetry directory.
// The return value of prog is the exit code of the process.
// RunProg can be called at most once per testing.T.
// If an integration test needs to run multiple programs, use subtests.
func RunProg(t *testing.T, telemetryDir string, prog func() int) ([]byte, error) {
	testName := t.Name()
	if !canRunProg(testName) {
		t.Fatalf("RunProg was called more than once in test %v. Use subtests if a test needs it more than once", testName)
	}
	if telemetryDir := os.Getenv(telemetryDirEnvVar); telemetryDir != "" {
		// run the prog.
		countertest.Open(telemetryDir)
		os.Exit(prog())
	}

	t.Helper()

	testBin, err := os.Executable()
	if err != nil {
		t.Fatalf("cannot determine the current process's executable name: %v", err)
	}

	// Spawn a subprocess to run the `prog`, by setting subprocessKeyEnvVar and telemetryDirEnvVar.
	cmd := exec.Command(testBin, "-test.run", testName)
	cmd.Env = append(cmd.Env, telemetryDirEnvVar+"="+telemetryDir)
	return cmd.CombinedOutput()
}

// InSubprocess returns whether the current process is a subprocess forked by RunProg.
func InSubprocess() bool {
	return os.Getenv(telemetryDirEnvVar) != ""
}

// ProgInfo returns the go version, program name and version info the process would record in its counter file.
func ProgInfo(t *testing.T) (goVersion, progVersion, progName string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("cannot read build info - it's likely this setup is unsupported by the counter package")
	}
	goVers := info.GoVersion
	if strings.Contains(goVers, "devel") || strings.Contains(goVers, "-") {
		goVers = "devel"
	}
	progPkgPath := info.Path
	if progPkgPath == "" {
		progPkgPath = strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	}
	progVers := info.Main.Version
	if strings.Contains(progVers, "devel") || strings.Contains(progVers, "-") {
		progVers = "devel"
	}
	return goVers, progVers, progPkgPath
}
