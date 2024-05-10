// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload_test

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/telemetry/internal/configtest"
	"golang.org/x/telemetry/internal/regtest"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
	"golang.org/x/telemetry/internal/upload"
)

// createUploader sets up an upload environment for the provided test, with a
// fake proxy allowing the given counters, and a fake upload server.
//
// The returned Uploader is ready to upload the given directory.
// The second return is a function to fetch all uploaded reports.
//
// For convenience, createUploader also sets the mode in telemetryDir to "on",
// back-dated to a time in the past. Callers that want to run the upload with a
// different mode can reset as necessary.
//
// All associated resources are cleaned up with t.Clean.
func createUploader(t *testing.T, telemetryDir string, counters, stackCounters []string) (*upload.Uploader, func() [][]byte) {
	t.Helper()

	if err := telemetry.NewDir(telemetryDir).SetModeAsOf("on", time.Now().Add(-365*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	srv, uploaded := upload.CreateTestUploadServer(t)
	uc := upload.CreateTestUploadConfig(t, counters, stackCounters)
	env := configtest.LocalProxyEnv(t, uc, "v1.2.3")

	uploader, err := upload.NewUploader(upload.RunConfig{
		TelemetryDir: telemetryDir,
		UploadURL:    srv.URL,
		LogWriter:    testWriter{t},
		Env:          env,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { uploader.Close() })
	return uploader, uploaded
}

// testWriter is an io.Writer wrapping t.Log.
type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(p []byte) (n int, err error) {
	w.t.Log(strings.TrimSuffix(string(p), "\n")) // trim newlines added by logging
	return len(p), nil
}

func TestUploader_MultipleUploads(t *testing.T) {
	// This test checks that Uploader.Run produces multiple reports when counters
	// span more than a week.

	testenv.SkipIfUnsupportedPlatform(t)

	// This program is run at two different dates.
	prog := regtest.NewIncProgram(t, "prog", "counter1")

	// Create two counter files to upload, at least a week apart.
	telemetryDir := t.TempDir()
	asof1 := time.Now().Add(-15 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	asof2 := time.Now().Add(-8 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	uploader, getUploads := createUploader(t, telemetryDir, []string{"counter1", "counter2"}, nil)
	if err := uploader.Run(); err != nil {
		t.Fatal(err)
	}

	uploads := getUploads()
	if got, want := len(uploads), 2; got != want {
		t.Fatalf("got %d uploads, want %d", got, want)
	}
	for _, upload := range uploads {
		report := string(upload)
		if !strings.Contains(report, "counter1") {
			t.Errorf("Didn't get an upload for counter1. Report:\n%s", report)
		}
	}
}

func TestUploader_EmptyUpload(t *testing.T) {
	// This test verifies that an empty counter file does not cause uploads of
	// another week's reports to fail.

	testenv.SkipIfUnsupportedPlatform(t)

	// prog1 runs in week 1, and increments no counter.
	prog1 := regtest.NewIncProgram(t, "prog1")
	// prog2 runs in week 2.
	prog2 := regtest.NewIncProgram(t, "prog2", "week2")

	telemetryDir := t.TempDir()

	// Create two counter files to upload, at least a week apart.
	// Week 1 has no counters, which in the past caused the both uploads to fail.
	asof1 := time.Now().Add(-15 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog1); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	asof2 := time.Now().Add(-8 * 24 * time.Hour)
	if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog2); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}

	uploader, getUploads := createUploader(t, telemetryDir, []string{"week1", "week2"}, nil)
	if err := uploader.Run(); err != nil {
		t.Fatal(err)
	}

	// Check that we got one upload, for week 2.
	uploads := getUploads()
	if got, want := len(uploads), 1; got != want {
		t.Fatalf("got %d uploads, want %d", got, want)
	}
	for _, upload := range uploads {
		report := string(upload)
		if !strings.Contains(report, "week2") {
			t.Errorf("Didn't get an upload for week2. Report:\n%s", report)
		}
	}
}

func TestUploader_ModeHandling(t *testing.T) {
	// This test verifies that the uploader honors the telemetry mode, as well as
	// its asof date.

	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewIncProgram(t, "prog1", "counter")

	tests := []struct {
		mode        string
		wantUploads int
	}{
		{"off", 0},
		{"local", 0},
		{"on", 1}, // only the second week is uploadable
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			telemetryDir := t.TempDir()
			// Create two counter files to upload, at least a week apart.
			now := time.Now()
			asof1 := now.Add(-15 * 24 * time.Hour)
			if out, err := regtest.RunProgAsOf(t, telemetryDir, asof1, prog); err != nil {
				t.Fatalf("failed to run program: %s", out)
			}
			asof2 := now.Add(-8 * 24 * time.Hour)
			if out, err := regtest.RunProgAsOf(t, telemetryDir, asof2, prog); err != nil {
				t.Fatalf("failed to run program: %s", out)
			}

			uploader, getUploads := createUploader(t, telemetryDir, []string{"counter"}, nil)

			// Enable telemetry as of 10 days ago. This should prevent the first week
			// from being uploaded, but not the second.
			if err := telemetry.NewDir(telemetryDir).SetModeAsOf(test.mode, now.Add(-10*24*time.Hour)); err != nil {
				t.Fatal(err)
			}

			if err := uploader.Run(); err != nil {
				t.Fatal(err)
			}

			uploads := getUploads()
			if gotUploads := len(uploads); gotUploads != test.wantUploads {
				t.Fatalf("got %d uploads, want %d", gotUploads, test.wantUploads)
			}
		})
	}
}
