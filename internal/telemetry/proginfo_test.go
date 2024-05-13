// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry_test

import (
	"fmt"
	"path"
	"runtime/debug"
	"testing"

	"golang.org/x/telemetry/internal/telemetry"
)

func TestProgramInfo_ProgramVersion(t *testing.T) {
	tests := []struct {
		path    string
		version string
		want    string
	}{
		{
			path:    "golang.org/x/tools/gopls",
			version: "(devel)",
			want:    "devel",
		},
		{
			path:    "golang.org/x/tools/gopls",
			version: "",
			want:    "",
		},
		{
			path:    "golang.org/x/tools/gopls",
			version: "v0.14.0-pre.1",
			want:    "v0.14.0-pre.1",
		},
		{
			path:    "golang.org/x/tools/gopls",
			version: "v0.0.0-20231207172801-3c8b0df0c3fd",
			want:    "devel",
		},
		{
			path:    "cmd/go",
			version: "",
			want:    "go1.23.0", // hard-coded below
		},
		{
			path:    "cmd/compile",
			version: "",
			want:    "go1.23.0", // hard-coded below
		},
	}
	type info struct {
		GoVers, ProgPkgPath, Prog, ProgVer string
	}
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("cannot use debug.ReadBuildInfo")
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s@%s", path.Base(tt.path), tt.version)
		t.Run(name, func(t *testing.T) {
			in := *buildInfo
			in.GoVersion = "go1.23.0"
			in.Path = tt.path
			in.Main.Version = tt.version
			_, _, got := telemetry.ProgramInfo(&in)
			if got != tt.want {
				t.Errorf("program version = %q, want %q", got, tt.want)
			}
		})
	}
}
