// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package configstore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/internal/proxy"
	"golang.org/x/telemetry/internal/testenv"
)

func TestDownload(t *testing.T) {
	testenv.NeedsGo(t)
	tmpdir := t.TempDir()
	defer cleanModuleCache(t, tmpdir)

	configVersion := "v0.1.0"
	in := telemetry.UploadConfig{
		GOOS:      []string{"darwin"},
		GOARCH:    []string{"amd64", "arm64"},
		GoVersion: []string{"1.20.3", "1.20.4"},
		Programs: []*telemetry.ProgramConfig{{
			Name:     "gopls",
			Versions: []string{"v0.11.0"},
			Counters: []telemetry.CounterConfig{{
				Name: "foobar",
				Rate: 2,
			}},
		}},
	}

	proxyURI, err := writeConfig(tmpdir, in, configVersion)
	if err != nil {
		t.Fatal("failed to prepare proxy:", err)
	}

	opts := testDownloadOption(proxyURI, tmpdir)

	testCases := []struct {
		version string
		want    telemetry.UploadConfig
	}{
		{version: configVersion, want: in},
		{version: "latest", want: in},
	}
	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			got, _, err := Download(tc.version, opts)
			if err != nil {
				t.Fatal("failed to download:", err)
			}

			want := tc.want
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Download(latest, _) = %v\nwant %v", stringify(got), stringify(want))
			}
		})
	}

	t.Run("invalidversion", func(t *testing.T) {
		got, ver, err := Download("nonexisting", opts)
		if err == nil {
			t.Fatalf("download succeeded unexpectedly: %v %+v", ver, got)
		}
		if !strings.Contains(err.Error(), "invalid version") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func stringify(x any) string {
	ret, err := json.MarshalIndent(x, "", " ")
	if err != nil {
		return fmt.Sprintf("json.Marshal failed - %v", err)
	}
	return string(ret)
}

// writeConfig adds cfg to the module proxy used for testing.
func writeConfig(dir string, cfg telemetry.UploadConfig, version string) (proxyURI string, _ error) {
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	dirPath := fmt.Sprintf("%v@%v/", configModulePath, version)
	files := map[string][]byte{
		dirPath + "go.mod":      []byte("module " + configModulePath + "\n\ngo 1.20\n"),
		dirPath + "config.json": encoded,
	}
	return proxy.WriteProxy(dir, files)
}

func testDownloadOption(proxyURI, tmpDir string) *DownloadOption {
	var env []string
	env = append(env, os.Environ()...)
	env = append(env,
		"GOPROXY="+proxyURI,  // Use the fake proxy.
		"GONOSUMDB=*",        // Skip verifying checksum against sum.golang.org.
		"GOMODCACHE="+tmpDir, // Don't pollute system module cache.
	)
	return &DownloadOption{
		Env: env,
	}
}

func cleanModuleCache(t *testing.T, tmpDir string) {
	t.Helper()
	cmd := exec.Command("go", "clean", "-modcache")
	cmd.Env = append(cmd.Environ(), "GOMODCACHE="+tmpDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go clean -modcache failed: %v\n%s", err, out)
	}
}
