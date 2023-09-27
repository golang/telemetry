// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testenv contains helper functions for skipping tests
// based on which tools are present in the environment.
package testenv

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

// NeedsLocalhostNet skips t if networking does not work for ports opened
// with "localhost".
func NeedsLocalhostNet(t testing.TB) {
	switch runtime.GOOS {
	case "js", "wasip1":
		t.Skipf(`Listening on "localhost" fails on %s; see https://go.dev/issue/59718`, runtime.GOOS)
	}
}

var (
	hasGoOnce sync.Once
	hasGoErr  error
)

func hasGo() error {
	hasGoOnce.Do(func() {
		cmd := exec.Command("go", "env", "GOROOT")
		out, err := cmd.Output()
		if err != nil { // cannot run go.
			hasGoErr = fmt.Errorf("%v: %v", cmd, err)
			return
		}
		out = bytes.TrimSpace(out)
		if len(out) == 0 { // unusual, incomplete go installation.
			hasGoErr = fmt.Errorf("%v: no GOROOT - incomplete go installation", cmd)
		}
	})
	return hasGoErr
}

// NeedsGo skips t if the current system does not have 'go'  or
// cannot run them with exec.Command.
func NeedsGo(t testing.TB) {
	if err := hasGo(); err != nil {
		t.Skipf("skipping test: go is not available - %v", err)
	}
}

// SkipIfUnsupportedPlatform skips test if the current os/arch
// are not support.
func SkipIfUnsupportedPlatform(t testing.TB) {
	t.Helper()
	switch runtime.GOOS {
	case "openbsd", "js", "wasip1", "solaris", "android":
		// BUGS: #60614 - openbsd, #60967 - android , #60968 - solaris #60970 - solaris #60971 - wasip1)
		t.Skip("broken for openbsd etc")
	}
	if runtime.GOARCH == "386" {
		// BUGS: #60615 #60692 #60965 #60967
		t.Skip("broken for GOARCH 386")
	}
}

// MustHaveExec checks that the current system can start new processes
// using os.StartProcess or (more commonly) exec.Command.
// If not, MustHaveExec calls t.Skip with an explanation.
func MustHaveExec(t testing.TB) {
	switch runtime.GOOS {
	case "wasip1", "js", "ios":
		t.Skipf("skipping test: may not be able to exec subprocess on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}
