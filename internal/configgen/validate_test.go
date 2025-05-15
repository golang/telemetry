// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"strings"
	"testing"

	"golang.org/x/telemetry/internal/chartconfig"
)

func TestLoadedChartsAreValid(t *testing.T) {
	// Test that we can actually load the chart config.
	charts, err := chartconfig.Load()
	if err != nil {
		t.Fatal("chartconfig.Load() failed:", err)
	}
	for i, chart := range charts {
		if err := ValidateChartConfig(chart); err != nil {
			t.Errorf("Chart %d is invalid: %v", i, err)
		}
	}

	if t.Failed() {
		// Skip the the rest of the test, it's redundant if
		// the chartconfig value isn't valid.
		return
	}

	// Test that all paddings are complete for the purposes
	// of being able to generate from the chartconfig value.
	for _, tc := range [...]struct {
		name     string
		paddings map[string]padding
	}{
		{"regularPaddings", regularPaddings},
		{"minimumPaddings", minimumPaddings},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("not running test that uses internet in short mode")
			}
			_, err := generate(charts, tc.paddings)
			if err != nil {
				t.Errorf("generate(charts, %s): %v", tc.name, err)
			}
		})
	}
}

func TestValidateOK(t *testing.T) {
	// A minimally valid chart config.
	const input = `
title: Editor Distribution
counter: gopls/editor:{emacs,vim,vscode,other}
type: partition
issue: https://go.dev/issue/12345
program: golang.org/x/tools/gopls
module: golang.org/x/tools/gopls
`
	records, err := chartconfig.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("Parse(%q) returned %d records, want exactly 1", input, len(records))
	}
	if err := ValidateChartConfig(records[0]); err != nil {
		t.Errorf("Validate(%q) = %v, want nil", input, err)
	}
}

func TestValidate(t *testing.T) {
	tests := map[string][]string{ // input -> want errors
		// validation of mandatory fields
		"description:bar": {"title", "program", "issue", "counter", "type"},

		// validation of semver intervals
		"version:1.2.3.4": {"semver"},

		// valid of stack configuration
		"depth:-1": {"non-negative", "stack"},
	}

	for input, wantErrs := range tests {
		records, err := chartconfig.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 1 {
			t.Fatalf("Parse(%q) returned %d records, want exactly 1", input, len(records))
		}
		err = ValidateChartConfig(records[0])
		if err == nil {
			t.Fatalf("Validate(%q) succeeded unexpectedly", input)
		}
		errs := err.Error()
		for _, want := range wantErrs {
			if !strings.Contains(errs, want) {
				t.Errorf("Validate(%q) = %v, want containing %q", input, err, want)
			}
		}
	}
}
