// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package telemetry manages the telemetry mode file.
package telemetry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var userConfigDir = os.UserConfigDir

// filename returns the default telemetry mode file name.
func filename() (string, error) {
	cfgDir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	// TODO(hyangah): should we consider GOTELEMETRYDIR?

	return filepath.Join(cfgDir, "go", "telemetry", "mode"), nil
}

// SetMode updates the telemetry mode with the given mode.
// Acceptable values for mode are "on", "off", "local", or https:// urls.
func SetMode(mode string) error {
	switch mode {
	case "on", "off", "local":
	default:
		if !strings.HasPrefix(mode, "https://") {
			return errors.New("invalid telemetry mode value")
		}
	}
	fname, err := filename()
	if err != nil {
		return fmt.Errorf("cannot create a telemetry mode file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		return fmt.Errorf("cannot create a telemetry mode file: %w", err)
	}
	data := []byte(mode)
	return os.WriteFile(fname, data, 0666)
}

// LookupMode returns the current telemetry mode.
func LookupMode() string {
	fname, err := filename()
	if err != nil {
		return "local" // default
	}
	data, err := os.ReadFile(fname)
	if err != nil {
		return "local" // default
	}
	mode := string(data)
	switch mode {
	case "on", "off", "local":
	default:
		if !strings.HasPrefix(mode, "https://") {
			return "local"
		}
	}
	return mode
}
