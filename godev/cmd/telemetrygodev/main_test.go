// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	_ "embed"
	"testing"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/godev/internal/upload"
)

func TestValidate(t *testing.T) {
	cfg, err := upload.ReadConfig("testdata/config.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		report  *telemetry.Report
		wantErr bool
	}{
		{
			name:    "empty report",
			report:  &telemetry.Report{},
			wantErr: true,
		},
		{
			name: "valid report with no counters",
			report: &telemetry.Report{
				Week:     "2023-06-15",
				LastWeek: "",
				X:        0.1,
				Programs: []*telemetry.ProgramReport{},
				Config:   "v0.0.1-test",
			},
			wantErr: false,
		},
		{
			name: "valid report with counters",
			report: &telemetry.Report{
				Week:     "2023-06-15",
				LastWeek: "",
				X:        0.1,
				Programs: []*telemetry.ProgramReport{
					{
						Program:   "golang.org/x/tools/gopls",
						Version:   "v0.10.1",
						GoVersion: "go1.20.1",
						GOOS:      "linux",
						GOARCH:    "arm64",
						Counters: map[string]int64{
							"editor:vim": 100,
						},
					},
				},
				Config: "v0.0.1-test",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validate(tt.report, cfg); (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
