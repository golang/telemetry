// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"testing"

	"golang.org/x/telemetry"
)

func TestGenerate(t *testing.T) {
	defer func(vers map[string][]string) {
		versionsForTesting = vers
	}(versionsForTesting)
	versionsForTesting = map[string][]string{
		"golang.org/toolchain":     {"v0.0.1-go1.21.0.linux-arm", "v0.0.1-go1.20.linux-arm"},
		"golang.org/x/tools/gopls": {"v0.13.0", "v0.14.0", "v0.15.0"},
	}
	const gcfg = `
title: Editor Distribution
counter: gopls/editor:{emacs,vim,vscode,other}
description: measure editor distribution for gopls users.
type: partition
issue: https://go.dev/issue/61038
program: golang.org/x/tools/gopls
version: v0.14.0
`
	got, err := generate([]byte(gcfg))
	if err != nil {
		t.Fatal(err)
	}
	want := telemetry.UploadConfig{
		GOOS:      goos(),
		GOARCH:    goarch(),
		GoVersion: []string{"go1.20", "go1.21.0"},
		Programs: []*telemetry.ProgramConfig{{
			Name:     "golang.org/x/tools/gopls",
			Versions: []string{"v0.14.0", "v0.15.0"},
			Counters: []telemetry.CounterConfig{{
				Name: "gopls/editor:{emacs,vim,vscode,other}",
				Rate: 0.1,
			}},
		}},
	}
	if !reflect.DeepEqual(*got, want) {
		if len(got.Programs) != len(want.Programs) {
			t.Errorf("generate(): got %d programs, want %d", len(got.Programs), len(want.Programs))
		} else {
			for i, gotp := range got.Programs {
				want := *want.Programs[i]
				if !reflect.DeepEqual(*gotp, want) {
					t.Errorf("generate() program #%d =\n%+v\nwant:\n%+v", i, *gotp, want)

				}
			}
		}
		t.Errorf("generate() =\n%+v\nwant:\n%+v", *got, want)
	}
}
