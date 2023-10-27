// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package regtest provides helpers for end-to-end testing
// involving counter and upload packages.
package regtest

import (
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

var (
	telemetryDirEnvVarValue = os.Getenv(telemetryDirEnvVar)
	entryPointEnvVarValue   = os.Getenv(entryPointEnvVar)
)

// Program is a value that can be used to identify a program in the test.
type Program string

// NewProgram returns a Program value that can be used to identify a program
// to run by RunProg. The program must be registered with NewProgram before
// the first call to RunProg in the test function.
//
// RunProg runs this binary in a separate process with special environment
// variables that specify the entry point. When this binary runs with the
// environment variables that match the specified name, NewProgram calls
// the given fn and exits with the return value. Note that all the code
// before NewProgram is executed in both the main process and the subprocess.
func NewProgram(t *testing.T, name string, fn func() int) Program {
	if telemetryDirEnvVarValue != "" && entryPointEnvVarValue == name {
		// We are running the separate process that was spawned by RunProg.
		fmt.Fprintf(os.Stderr, "running program %q\n", name)
		countertest.Open(telemetryDirEnvVarValue)
		os.Exit(fn())
	}

	testName, _, _ := strings.Cut(t.Name(), "/")
	registered, ok := registeredPrograms[testName]
	if !ok {
		registered = make(map[string]bool)
	}
	if registered[name] {
		t.Fatalf("program %q was already registered", name)
	}
	registered[name] = true
	return Program(name)
}

// registeredPrograms stores all registered program names to detect duplicate registrations.
var registeredPrograms = make(map[string]map[string]bool) // test name -> program name -> exist

// RunProg runs the program prog in a separate process with the specified
// telemetry directory. RunProg can be called multiple times in the same test,
// but all the programs must be registered with NewProgram before the first
// call to RunProg.
func RunProg(t *testing.T, telemetryDir string, prog Program) ([]byte, error) {
	if telemetryDirEnvVarValue != "" {
		fmt.Fprintf(os.Stderr, "unknown program %q\n %s %s", prog, telemetryDirEnvVarValue, entryPointEnvVarValue)
		os.Exit(2)
	}
	testName, _, _ := strings.Cut(t.Name(), "/")
	testBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine the current process's executable name: %v", err)
	}

	// Spawn a subprocess to run the 'prog' by setting telemetryDirEnvVar.
	cmd := exec.Command(testBin, "-test.run", fmt.Sprintf("^%s$", testName))
	cmd.Env = append(cmd.Env, telemetryDirEnvVar+"="+telemetryDir, entryPointEnvVar+"="+string(prog))
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
