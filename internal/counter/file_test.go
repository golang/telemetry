// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"runtime/debug"
	"testing"
)

func TestProgramInfo_ProgramVersion(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "(devel)",
			in:   "(devel)",
			want: "devel",
		},
		{
			name: "empty version",
			in:   "",
			want: "",
		},
		{
			name: "prerelease",
			in:   "v0.14.0-pre.1",
			want: "v0.14.0-pre.1",
		},
		{
			name: "pseudoversion",
			in:   "v0.0.0-20231207172801-3c8b0df0c3fd",
			want: "devel",
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
		t.Run(tt.name, func(t *testing.T) {
			in := *buildInfo
			in.Main.Version = tt.in
			_, _, _, got := programInfo(&in)
			if got != tt.want {
				t.Errorf("program version = %q, want %q", got, tt.want)
			}
		})
	}
}
