// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
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
	env, err := os.UserConfigDir()
	if err != nil {
		log.Print(err)
		return
	}
	env = filepath.Join(env, "go", "telemetry")

	l := filepath.Join(env, "local")
	u := filepath.Join(env, "upload")
	if err := os.MkdirAll(l, 0755); err != nil {
		log.Printf("no local directory %v", err)
		return
	}
	if err := os.MkdirAll(u, 0755); err != nil {
		log.Printf("no upload directory %v", err)
		return
	}
	LocalDir = l
	UploadDir = u

	env = readVar("GOTELEMETRY")
	Enabled = env != "off"
}

func readVar(name string) string {
	env := os.Getenv(name)
	if env == "" {
		env = readGoEnv(name + "=")
	}
	return env
}

func readGoEnv(key string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "go/env"))
	if err != nil {
		return ""
	}
	for data != nil {
		line, rest, _ := bytes.Cut(data, []byte("\n"))
		data = rest
		if bytes.HasPrefix(line, []byte(key)) {
			return string(line[len(key):])
		}
	}
	return ""
}
