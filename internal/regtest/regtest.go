// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.21

// Package regtest provides helpers for end-to-end testing
// involving counter and upload packages. This package requires go1.21 or newer.
package regtest

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"golang.org/x/telemetry/counter/countertest"
)

const (
	telemetryDirEnvVar = "_COUNTERTEST_RUN_TELEMETRY_DIR"
	entryPointEnvVar   = "_COUNTERTEST_ENTRYPOINT"
)

// Main is a test main function for use in TestMain, which runs one of the
// given programs when invoked as a separate process via RunProg.
//
// The return value of each program is the exit code of the process.
func Main(m *testing.M, programs map[string]func() int) {
	if d := os.Getenv(telemetryDirEnvVar); d != "" {
		countertest.Open(d)
	}
	if e, ok := os.LookupEnv(entryPointEnvVar); ok {
		if prog, ok := programs[e]; ok {
			os.Exit(prog())
		}
		fmt.Fprintf(os.Stderr, "unknown program %q", e)
		os.Exit(2)
	}
	flag.Parse()
	os.Exit(m.Run())
}

// RunProg runs the named program in a separate process with the specified
// telemetry directory, where prog is one of the programs passed to Main (which
// must be invoked by TestMain).
func RunProg(telemetryDir string, prog string) ([]byte, error) {
	testBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine the current process's executable name: %v", err)
	}

	// Spawn a subprocess to run the `prog`, by setting subprocessKeyEnvVar and telemetryDirEnvVar.
	cmd := exec.Command(testBin)
	cmd.Env = append(cmd.Env, telemetryDirEnvVar+"="+telemetryDir, entryPointEnvVar+"="+prog)
	return cmd.CombinedOutput()
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
