// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package telemetry manages the telemetry mode file.
package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The followings are the process' default Settings.
// The values are subdirectories and a file under
// os.UserConfigDir()/go/telemetry.
// For convenience, each field is made to global
// and they are not supposed to be changed.
var (
	// Default directory containing count files and local reports (not yet uploaded)
	LocalDir string
	// Default directory containing uploaded reports.
	UploadDir string
	// Default file path that holds the telemetry mode info.
	ModeFile ModeFilePath
)

// ModeFilePath is the telemetry mode file path with methods to manipulate the file contents.
type ModeFilePath string

func init() {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	gotelemetrydir := filepath.Join(cfgDir, "go", "telemetry")
	LocalDir = filepath.Join(gotelemetrydir, "local")
	UploadDir = filepath.Join(gotelemetrydir, "upload")
	ModeFile = ModeFilePath(filepath.Join(gotelemetrydir, "mode"))
}

// SetMode updates the telemetry mode with the given mode.
// Acceptable values for mode are "on" or "off".
func SetMode(mode string) error {
	return ModeFile.SetMode(mode)
}

func (m ModeFilePath) SetMode(mode string) error {
	mode = strings.TrimSpace(mode)
	switch mode {
	case "on", "off":
	default:
		return fmt.Errorf("invalid telemetry mode: %q", mode)
	}
	fname := string(m)
	if fname == "" {
		return fmt.Errorf("cannot determine telemetry mode file name")
	}
	if err := os.MkdirAll(filepath.Dir(fname), 0755); err != nil {
		return fmt.Errorf("cannot create a telemetry mode file: %w", err)
	}
	data := []byte(mode)
	return os.WriteFile(fname, data, 0666)
}

// Mode returns the current telemetry mode.
func Mode() string {
	return ModeFile.Mode()
}

func (m ModeFilePath) Mode() string {
	fname := string(m)
	if fname == "" {
		return "off" // it's likely LocalDir/UploadDir are empty too. Turn off telemetry.
	}
	data, err := os.ReadFile(fname)
	if err != nil {
		return "off" // default
	}
	mode := string(data)
	mode = strings.TrimSpace(mode)
	return mode
}
