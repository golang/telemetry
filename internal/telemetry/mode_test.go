// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package telemetrymode manages the telemetry mode file.
package telemetry

import (
	"os"
	"testing"
)

func TestTelemetryModeWithNoModeConfig(t *testing.T) {
	defer func() { userConfigDir = os.UserConfigDir }()

	tmp := t.TempDir()
	userConfigDir = func() (string, error) { return tmp, nil }

	got := LookupMode()
	if got != "local" {
		t.Fatalf("LookupMode() = %q, want local", got)
	}
}

func TestTelemetryMode(t *testing.T) {
	defer func() { userConfigDir = os.UserConfigDir }()

	tmp := t.TempDir()
	userConfigDir = func() (string, error) { return tmp, nil }

	tests := []struct {
		in      string
		wantErr bool // want error when setting.
	}{
		{"on", false},
		{"off", false},
		{"local", false},
		{"https://mytelemetry.com", false},
		{"http://insecure.com", true},
		{"bogus", true},
		{"", true},
	}
	for _, tt := range tests {
		t.Run("mode="+tt.in, func(t *testing.T) {
			setErr := SetMode(tt.in)
			if (setErr != nil) != tt.wantErr {
				t.Fatalf("Set() error = %v, wantErr %v", setErr, tt.wantErr)
			}
			if setErr != nil {
				return
			}
			if got := LookupMode(); got != tt.in {
				t.Errorf("LookupMode() = %q, want %q", got, tt.in)
			}
		})
	}
}
