// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry_test

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/counter/countertest"
	"golang.org/x/telemetry/internal/configtest"
	"golang.org/x/telemetry/internal/regtest"
	it "golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

// These environment variables are used to coordinate the fork+exec subprocess
// started by TestStart.
const (
	runStartEnv     = "X_TELEMETRY_TEST_START"
	telemetryDirEnv = "X_TELEMETRY_TEST_START_TELEMETRY_DIR"
	uploadURLEnv    = "X_TELEMETRY_TEST_START_UPLOAD_URL"
)

func TestMain(m *testing.M) {
	// TestStart can't use internal/regtest, because Start itself also uses
	// fork+exec to start a subprocess, which does not interact well with the
	// fork+exec trick used by regtest.RunProg.
	if os.Getenv(runStartEnv) != "" {
		os.Exit(runStart())
	}
	os.Exit(m.Run())
}

func TestStart(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	testenv.MustHaveExec(t)

	// TODO(golang/go#67211): enable this test at go tip once the bug in Start
	// delegation is fixed.
	if testenv.Go1Point() >= 23 {
		t.Skip("skipping due to golang/go#67211: Start fails with the current x/telemetry vendored into the Go command")
	}

	// This test sets up a telemetry environment, and then runs a test program
	// that increments a counter, and uploads via telemetry.Start.

	t.Setenv(telemetryDirEnv, t.TempDir())

	uploaded := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploaded = true
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("error reading body: %v", err)
		} else {
			if body := string(body); !strings.Contains(body, "teststart/counter") {
				t.Errorf("upload does not contain \"teststart/counter\":\n%s", body)
			}
		}
	}))
	t.Setenv(uploadURLEnv, server.URL)

	uc := regtest.CreateTestUploadConfig(t, []string{"teststart/counter"}, nil)
	env := configtest.LocalProxyEnv(t, uc, "v1.2.3")
	for _, e := range env {
		kv := strings.SplitN(e, "=", 2)
		t.Setenv(kv[0], kv[1])
	}

	// Run the runStart function below, via a fork+exec trick.
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "** TestStart **")      // this unused arg is just for ps(1)
	cmd.Env = append(os.Environ(), runStartEnv+"=1") // see TestMain
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("program failed unexpectedly (%v)\n%s", err, out)
	}

	if !uploaded {
		t.Fatalf("no upload occurred on %v", os.Getpid())
	}
}

func runStart() int {
	mustGetEnv := func(envvar string) string {
		v := os.Getenv(envvar)
		if v == "" {
			log.Fatalf("missing required environment var %q", envvar)
		}
		return v
	}

	countertest.Open(mustGetEnv(telemetryDirEnv))
	counter.Inc("teststart/counter")
	if err := it.Default.SetModeAsOf("on", time.Now().Add(-8*24*time.Hour)); err != nil {
		log.Fatalf("setting mode: %v", err)
	}

	res := telemetry.Start(telemetry.Config{
		// No need to set TelemetryDir since the Default dir is already set by countertest.Open.
		Upload:          true,
		UploadURL:       mustGetEnv(uploadURLEnv),
		UploadStartTime: time.Now().Add(8 * 24 * time.Hour),
	})
	res.Wait()
	return 0
}
