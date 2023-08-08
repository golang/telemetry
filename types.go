// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry

import (
	"io"

	"golang.org/x/telemetry/internal/telemetry"
)

// Common types and directories used by multiple packages.

// An UploadConfig controls what data is uploaded.
type UploadConfig struct {
	GOOS      []string
	GOARCH    []string
	GoVersion []string // how does this get changed with new releases?
	Programs  []*ProgramConfig
	Version   string `json:",omitempty" ` // version of this config. TODO(rfindley): Is this needed?
	// TODO(rfindley): add a 'FormatVersion' field, to allow evolution of the config format?
}

type ProgramConfig struct {
	// the counter names may have to be
	// repeated for each program. (e.g., if the counters are in a package
	// that is used in more than one program.)
	Name     string
	Versions []string        // versions present in a counterconfig
	Counters []CounterConfig `json:",omitempty"`
	Stacks   []CounterConfig `json:",omitempty"`
}

type CounterConfig struct {
	Name  string
	Rate  float64 // If X < Rate, report this counter
	Depth int     `json:",omitempty"` // for stack counters
}

// A Report is what's uploaded (or saved locally)
type Report struct {
	Week     string  // first day this report covers (YYYY-MM-DD)
	LastWeek string  // Week field from latest previous report uploaded
	X        float64 // A random proability used to determine which counters are uploaded
	Programs []*ProgramReport
	Config   string // version of UploadConfig used
}

type ProgramReport struct {
	Program   string
	Version   string
	GoVersion string
	GOOS      string
	GOARCH    string
	Counters  map[string]int64
	Stacks    map[string]int64
}

// A Control allows the user to override various default
// reporting and uploading choices.
// Future versions may also allow the user to set the upload URL.
type Control struct {
	// UploadConfig provides the telemetry UploadConfig used to
	// decide which counters get uploaded. nil is legal, and
	// means the code will use the latest version of the module
	// golang.org/x/telemetry/config.
	UploadConfig func() *UploadConfig
	// Logging provides a io.Writer for error messages during uploading
	// nil is legal and no log messages get generated
	Logging io.Writer
}

var (
	// directory containing count files and local (not to be uploaded) reports
	LocalDir = telemetry.LocalDir
	// directory containing uploaded reports
	UploadDir = telemetry.UploadDir
	// whether telemetry is enabled
	Enabled bool = telemetry.Mode() != "off" // TODO(hyangah): consider to delete.
)
