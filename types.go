// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry

import (
	"os"
	"path/filepath"

	"golang.org/x/telemetry/internal/telemetry"
)

// Common types and directories used by multple packages.

// An UploadConfig controls what data is uploaded.
type UploadConfig struct {
	GOOS      []string
	GOARCH    []string
	GoVersion []string // how does this get changed with new releases?
	Programs  []*ProgramConfig
	Version   string // version of this config. Is this needed?
}

type ProgramConfig struct {
	// the counter names may have to be
	// repeated for each program. (e.g., if the counters are in a package
	// that is used in more than one program.)
	Name     string
	Versions []string // where do these come from
	Counters []CounterConfig
	Stacks   []CounterConfig
}

type CounterConfig struct {
	Name  string
	Rate  float64 // If X < Rate, report this counter
	Depth int     // for stack counters
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

var (
	// directory containing count files and local (not to be uploaded) reports
	LocalDir string
	// directory containing uploaded reports
	UploadDir string
	// whether telemetry is enabled
	Enabled bool
)

// init() sets LocalDir and UploadDir. Users must not change these.
// If the directories cannot be found or set, telemetry is disabled.
func init() {
	mode := telemetry.LookupMode()
	if mode == "off" {
		return
	}

	env, err := os.UserConfigDir()
	if err != nil {
		return
	}
	env = filepath.Join(env, "go", "telemetry")

	l := filepath.Join(env, "local")
	u := filepath.Join(env, "upload")
	if err := os.MkdirAll(l, 0755); err != nil {
		return
	}
	if err := os.MkdirAll(u, 0755); err != nil {
		return
	}
	LocalDir = l
	UploadDir = u
	Enabled = true
}
