// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package telemetrymode manages the telemetry mode file.
package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTelemetryDefault(t *testing.T) {
	defaultDirMissing := false
	if _, err := os.UserConfigDir(); err != nil {
		defaultDirMissing = true
	}
	if defaultDirMissing {
		if LocalDir != "" || UploadDir != "" || ModeFile != "" {
			t.Errorf("DefaultSetting: (%q, %q, %q), want empty LocalDir/UploadDir/ModeFile", LocalDir, UploadDir, ModeFile)
		}
	} else {
		if LocalDir == "" || UploadDir == "" || ModeFile == "" {
			t.Errorf("DefaultSetting: (%q, %q, %q), want non-empty LocalDir/UploadDir/ModeFile", LocalDir, UploadDir, ModeFile)
		}
	}
}
func TestTelemetryModeWithNoModeConfig(t *testing.T) {
	tmp := t.TempDir()
	tests := []struct {
		modefile ModeFilePath
		want     string
	}{
		{ModeFilePath(filepath.Join(tmp, "mode")), "off"},
		{"", "off"},
	}
	for _, tt := range tests {
		if got := tt.modefile.Mode(); got != tt.want {
			t.Logf("Mode file: %q", tt.modefile)
			t.Errorf("Mode() = %v, want %v", got, tt.want)
		}
	}
}

func TestTelemetryMode(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool // want error when setting.
	}{
		{"on", false},
		{"off", false},
		{"local", true}, // golang/go#63143: local mode is no longer supported
		{"https://mytelemetry.com", true},
		{"http://insecure.com", true},
		{"bogus", true},
		{"", true},
	}
	tmp := t.TempDir()
	for i, tt := range tests {
		t.Run("mode="+tt.in, func(t *testing.T) {
			modefile := ModeFilePath(filepath.Join(tmp, fmt.Sprintf("modefile%d", i)))
			setErr := modefile.SetMode(tt.in)
			if (setErr != nil) != tt.wantErr {
				t.Fatalf("Set() error = %v, wantErr %v", setErr, tt.wantErr)
			}
			if setErr != nil {
				return
			}
			if got := modefile.Mode(); got != tt.in {
				t.Errorf("LookupMode() = %q, want %q", got, tt.in)
			}
		})
	}
}
