// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	_ "embed"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/telemetry/godev/internal/config"
	tconfig "golang.org/x/telemetry/internal/config"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

func TestMain(m *testing.M) {
	if !canRunGoDevModuleTests() {
		return
	}
	os.Exit(m.Run())
}

// First class ports, https://go.dev/wiki/PortingPolicy, excluding 386 and arm.
var onSupportedPlatform = map[string]bool{
	"darwin/amd64":  true,
	"darwin/arm64":  true,
	"linux/amd64":   true,
	"linux/arm64":   true,
	"windows/amd64": true,
}

// canRunGoDevModuleTests returns whether the current test environment
// is suitable for golang.org/x/telemetry/godev module tests.
func canRunGoDevModuleTests() bool {
	// Even though telemetry.go.dev runs on linux,
	// we should be still able to debug and test telemetry.go.dev on
	// contributors' machines locally.
	if goosarch := runtime.GOOS + "/" + runtime.GOARCH; !onSupportedPlatform[goosarch] {
		return false
	}
	// Must be able to run 'go'.
	if err := testenv.HasGo(); err != nil {
		return false
	}

	// Our tests must run from the repository source, not from module cache.
	// Check golang.org/x/telemetry directory is accessible and has go.mod and config/config.json.
	output, err := exec.Command("go", "list", "-f", "{{.Dir}}", "golang.org/x/telemetry").Output()
	if err != nil {
		return false
	}
	xTelemetryDir := string(bytes.TrimSpace(output))
	if xTelemetryDir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(xTelemetryDir, "go.mod")); err != nil {
		return false
	}
	// config/config.json is in the golang.org/x/telemetry/config module, so
	// this doesn't hold from e.g. GOMODCACHE.
	if _, err := os.Stat(filepath.Join(xTelemetryDir, "config", "config.json")); err != nil {
		return false
	}

	return true
}

// If telemetry_url is configured, TestPaths may be used as a basic push test.
var telemetryURL = flag.String("telemetry_url", "", "url of the telemetry instance to test")

func TestPaths(t *testing.T) {
	rootURL := *telemetryURL
	if rootURL == "" {
		ctx := context.Background()
		cfg := config.NewConfig()
		cfg.LocalStorage = t.TempDir()
		// NewConfig assumes that the command is run from the repo root, but tests
		// run from their test directory. We should fix this, but for now just
		// fix up the config path.
		// TODO(rfindley): fix this.
		cfg.UploadConfig = filepath.Join("..", "..", "..", "config", "config.json")
		handler := newHandler(ctx, cfg)
		ts := httptest.NewServer(handler)
		defer ts.Close()
		rootURL = ts.URL
	}

	tests := []struct {
		method    string
		path      string
		body      string
		code      int
		fragments []string
	}{
		{"GET", "/", "", 200, []string{"Overview"}},
		{
			"POST",
			"/upload/2023-01-01/123.json",
			`{"Week":"2023-01-01","LastWeek":"2022-12-25","X":0.123,"Programs":null,"Config":"v0.0.0-20230822160736-17171dbf1d76"}`,
			200,
			nil, // the body returned by /upload doesn't matter
		},
	}

	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			url := strings.TrimRight(rootURL, "/") + test.path
			r := strings.NewReader(test.body)
			req, err := http.NewRequest(test.method, url, r)
			if err != nil {
				t.Fatalf("NewRequest failed: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != test.code {
				t.Errorf("status code = %d, want %d", resp.StatusCode, test.code)
			}

			content, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("reading body: %v", err)
			}
			for _, fragment := range test.fragments {
				if !bytes.Contains(content, []byte(fragment)) {
					t.Errorf("missing fragment %q", fragment)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cfg, err := tconfig.ReadConfig("testdata/config.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		report  *telemetry.Report
		wantErr bool
	}{
		{
			name:    "empty report",
			report:  &telemetry.Report{},
			wantErr: true,
		},
		{
			name: "valid report with no counters",
			report: &telemetry.Report{
				Week:     "2023-06-15",
				LastWeek: "",
				X:        0.1,
				Programs: []*telemetry.ProgramReport{},
				Config:   "v0.0.1-test",
			},
			wantErr: false,
		},
		{
			name: "valid report with counters",
			report: &telemetry.Report{
				Week:     "2023-06-15",
				LastWeek: "",
				X:        0.1,
				Programs: []*telemetry.ProgramReport{
					{
						Program:   "golang.org/x/tools/gopls",
						Version:   "v0.10.1",
						GoVersion: "go1.20.1",
						GOOS:      "linux",
						GOARCH:    "arm64",
						Counters: map[string]int64{
							"editor:vim": 100,
						},
					},
				},
				Config: "v0.0.1-test",
			},
		},
		{
			name: "valid report with a stack counter",
			report: &telemetry.Report{
				Week:     "2023-06-15",
				LastWeek: "",
				X:        1.0,
				Programs: []*telemetry.ProgramReport{
					{
						Program:   "golang.org/x/tools/gopls",
						Version:   "v0.10.1",
						GoVersion: "go1.20.1",
						GOOS:      "linux",
						GOARCH:    "arm64",
						Stacks: map[string]int64{
							"gopls/bug\ngolang.org/x/tools/gopls/internal/bug.report:35\ngolang.org/x/tools/gopls/internal/bug.Errorf:2\ngolang.org/x/tools/gopls/internal/lsp.(*Server).SignatureHelp:1\nruntime.goexit:0": 1,
						},
					},
				},
				Config: "v0.0.1-test",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validate(tt.report, cfg); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
