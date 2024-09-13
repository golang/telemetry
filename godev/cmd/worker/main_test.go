// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/telemetry/internal/config"
	"golang.org/x/telemetry/internal/telemetry"
)

var exampleReports = []telemetry.Report{
	{
		Week:     "2999-01-01",
		LastWeek: "2998-01-01",
		X:        0.1,
		Programs: []*telemetry.ProgramReport{
			{
				Program:   "cmd/go",
				Version:   "go1.2.3",
				GoVersion: "go1.2.3",
				GOOS:      "darwin",
				GOARCH:    "arm64",
				Counters: map[string]int64{
					"main": 1,
				},
			},
			{
				Program:   "example.com/mod/pkg",
				Version:   "v2.3.4",
				GoVersion: "go1.2.3",
				GOOS:      "darwin",
				GOARCH:    "arm64",
				Counters: map[string]int64{
					"main":   1,
					"flag:a": 2,
					"flag:b": 3,
				},
				// TODO: add support for stacks
				Stacks: map[string]int64{
					"panic": 4,
				},
			},
			{
				Program:   "example.com/mod/pkg",
				Version:   "v2.3.4-pre.1",
				GoVersion: "go1.2.3",
				GOOS:      "darwin",
				GOARCH:    "arm64",
				Counters: map[string]int64{
					"flag:b": 3,
				},
				// TODO: add support for stacks
				Stacks: map[string]int64{
					"panic": 2,
				},
			},
		},
		Config: "v0.0.1",
	},
	{
		Week:     "2999-01-01",
		LastWeek: "2998-01-01",
		X:        0.2,
		Programs: []*telemetry.ProgramReport{
			{
				Program:   "example.com/mod/pkg",
				Version:   "v1.2.3",
				GoVersion: "go1.2.3",
				GOOS:      "darwin",
				GOARCH:    "arm64",
				Counters: map[string]int64{
					"main":   1,
					"flag:a": 2,
					"flag:b": 3,
				},
				// TODO: add support for stacks
				Stacks: map[string]int64{
					"panic": 4,
				},
			},
			{
				Program:   "example.com/mod/pkg",
				Version:   "v2.3.4",
				GoVersion: "go1.19.0",
				GOOS:      "darwin",
				GOARCH:    "arm64",
				Counters: map[string]int64{
					"main":   1,
					"flag:a": 2,
					"flag:b": 3,
				},
				// TODO: add support for stacks
				Stacks: map[string]int64{
					"panic": 4,
				},
			},
		},
		Config: "v0.0.1",
	},
	{
		Week:     "2999-01-01",
		LastWeek: "2998-01-01",
		X:        0.3,
		Programs: []*telemetry.ProgramReport{
			{
				Program:   "example.com/mod/pkg",
				Version:   "v1.2.3",
				GoVersion: "go1.2.3",
				GOOS:      "linux",
				GOARCH:    "amd64",
				Counters: map[string]int64{
					"main":   4,
					"flag:a": 5,
					"flag:b": 6,
					"flag:c": 1,
				},
				// TODO: add support for stacks
				Stacks: map[string]int64{
					"panic": 7,
				},
			},
		},
		Config: "v0.0.1",
	},
}

func TestGroup(t *testing.T) {
	type args struct {
		reports []telemetry.Report
	}
	tests := []struct {
		name string
		args args
		want data
	}{
		{
			name: "single report",
			args: args{
				[]telemetry.Report{
					{
						Week:     "2999-01-01",
						LastWeek: "2998-01-01",
						X:        0.123456789,
						Programs: []*telemetry.ProgramReport{
							{
								Program:   "example.com/mod/pkg",
								Version:   "v1.2.3",
								GoVersion: "go1.2.3",
								GOOS:      "darwin",
								GOARCH:    "arm64",
								Counters: map[string]int64{
									"main":   1,
									"flag:a": 2,
									"flag:b": 3,
								},
								// TODO: add support for stacks
								Stacks: map[string]int64{
									"panic": 4,
								},
							},
						},
						Config: "v0.0.1",
					},
				},
			},
			want: data{
				weekName("2999-01-01"): {
					programName("example.com/mod/pkg"): {
						graphName("Version"): {
							bucketName("v1.2.3"): {
								reportID(0.1234567890): 1,
							},
						},
						graphName("GOOS"): {
							bucketName("darwin"): {
								reportID(0.1234567890): 1,
							},
						},
						graphName("GOARCH"): {
							bucketName("arm64"): {
								reportID(0.1234567890): 1,
							},
						},
						graphName("GoVersion"): {
							bucketName("go1.2.3"): {
								reportID(0.1234567890): 1,
							},
						},
						graphName("main"): {
							bucketName("main"): {
								reportID(0.1234567890): 1,
							},
						},
						graphName("flag"): {
							bucketName("a"): {
								reportID(0.1234567890): 2,
							},
							bucketName("b"): {
								reportID(0.1234567890): 3,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := group(tt.args.reports)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("nest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPartition(t *testing.T) {
	exampleData := group(exampleReports)
	type args struct {
		program programName
		name    graphName
		buckets []bucketName
	}
	tests := []struct {
		name string
		data data
		args args
		want *chart
	}{
		{
			name: "major.minor.patch version counter",
			data: exampleData,
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []bucketName{"v1.2.3", "v2.3.4"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:Version",
				Name: "Version",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "v1.2",
						Value: 2,
					},
					{
						Week:  "2999-01-01",
						Key:   "v2.3",
						Value: 2, // TODO(rfindley): why isn't this '2'? There are two reports in the data.
					},
				},
			},
		},
		{
			name: "major.minor version counter should have same result as major.minor.patch",
			data: exampleData,
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []bucketName{"v1.2.3", "v2.3.4"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:Version",
				Name: "Version",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "v1.2",
						Value: 2,
					},
					{
						Week:  "2999-01-01",
						Key:   "v2.3",
						Value: 2,
					},
				},
			},
		},
		{
			name: "duplicated counter should be ignored",
			data: exampleData,
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []bucketName{"v1.2.3", "v2.3.4", "v1.2.3"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:Version",
				Name: "Version",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "v1.2",
						Value: 2,
					},
					{
						Week:  "2999-01-01",
						Key:   "v2.3",
						Value: 2,
					},
				},
			},
		},
		{
			name: "goos counter",
			data: exampleData,
			args: args{
				program: "example.com/mod/pkg",
				name:    "GOOS",
				buckets: []bucketName{"darwin", "linux"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:GOOS",
				Name: "GOOS",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "darwin",
						Value: 2,
					},
					{
						Week:  "2999-01-01",
						Key:   "linux",
						Value: 1,
					},
				},
			},
		},
		{
			name: "three days, multiple versions",
			data: data{
				"2999-01-01": {"example.com/mod/pkg": {"Version": {
					"v1.2.3": {0.1: 2},
					"v2.3.4": {0.1: 3},
				},
				}},
				"2999-01-04": {"example.com/mod/pkg": {"Version": {
					"v1.2.3": {0.3: 2},
					"v2.3.4": {0.4: 5},
				},
				}},
				"2999-01-05": {"example.com/mod/pkg": {"Version": {
					"v2.3.4": {0.5: 6},
				}}},
			},
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []bucketName{"v1.2.3", "v2.3.4"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:Version",
				Name: "Version",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-05",
						Key:   "v1.2",
						Value: 2,
					},
					{
						Week:  "2999-01-05",
						Key:   "v2.3",
						Value: 3,
					},
				},
			},
		},
		{
			name: "three days, multiple GOOS",
			data: data{
				"2999-01-01": {"example.com/mod/pkg": {"GOOS": {
					"darwin": {0.1: 2, 0.2: 2, 0.3: 2},
					"linux":  {0.1: 2, 0.2: 2},
				},
				}},
				"2999-01-02": {"example.com/mod/pkg": {"GOOS": {
					"darwin": {0.4: 2, 0.5: 2},
					"linux":  {0.6: 5},
				},
				}},
				"2999-01-03": {"example.com/mod/pkg": {"GOOS": {
					"darwin": {0.6: 3},
				},
				}},
			},
			args: args{
				program: "example.com/mod/pkg",
				name:    "GOOS",
				buckets: []bucketName{"darwin", "linux"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:GOOS",
				Name: "GOOS",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-03",
						Key:   "darwin",
						Value: 6,
					},
					{
						Week:  "2999-01-03",
						Key:   "linux",
						Value: 3,
					},
				},
			},
		},
		{
			name: "two days data, missing GOOS in first day",
			data: data{
				"2999-01-01": {"example.com/mod/pkg": {"Version": {
					"v1.2": {0.1: 2},
				},
				}},
				"2999-01-02": {"example.com/mod/pkg": {"GOOS": {
					"darwin": {0.3: 2},
					"linux":  {0.3: 2},
				},
				}},
			},
			args: args{
				program: "example.com/mod/pkg",
				name:    "GOOS",
				buckets: []bucketName{"darwin", "linux"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:GOOS",
				Name: "GOOS",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-02",
						Key:   "darwin",
						Value: 1,
					},
					{
						Week:  "2999-01-02",
						Key:   "linux",
						Value: 1,
					},
				},
			},
		},
		{
			name: "three days, missing version data all days",
			data: data{
				"2999-01-01": {"example.com/mod/pkg": {"GOOS": {
					"GOOS":        {0.1: 2},
					"GOOS:darwin": {0.1: 2},
				},
				}},
				"2999-01-02": {"example.com/mod/pkg": {"GOOS": {
					"GOOS":       {0.6: 5},
					"GOOS:linux": {0.6: 5},
				},
				}},
				"2999-01-03": {"example.com/mod/pkg": {"GOOS": {
					"GOOS":        {0.6: 3},
					"GOOS:darwin": {0.6: 3},
				},
				}},
			},
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []bucketName{"v1.2.3", "v2.3.4"},
			},
			want: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.data.partition(tc.args.program, tc.args.name, tc.args.buckets, nil)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("partition() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCharts(t *testing.T) {
	exampleData := group(exampleReports)
	cfg := &config.Config{
		UploadConfig: &telemetry.UploadConfig{
			GOOS:       []string{"darwin"},
			GOARCH:     []string{"amd64"},
			GoVersion:  []string{"go1.2.3", "go1.19.0"},
			SampleRate: 1,
			Programs: []*telemetry.ProgramConfig{
				{
					Name:     "cmd/go",
					Versions: []string{"go1.2.3"},
					Counters: []telemetry.CounterConfig{{
						Name: "main",
					}},
				},
				{
					Name:     "cmd/compiler",
					Versions: []string{"go1.2.3"},
					Counters: []telemetry.CounterConfig{{
						Name: "count1",
					}},
				},
				{
					Name: "example.com/mod/pkg",
					// Exercise semver sorting. Notably v1.2.3 has data but is not
					// present.
					//
					// TODO(rfindley): in a follow-up CL, remove the MajMin collapsing of
					// Versions. It's actually really interesting to see detailed version
					// information.
					Versions: []string{"v2.3.4", "v2.3.4-pre.1", "v0.15.0"},
					Counters: []telemetry.CounterConfig{
						{Name: "count2"},
						{Name: "flag:{a,b,c}"},
					},
				},
			},
		},
	}
	want := &chartdata{
		DateRange: [2]string{"2999-01-01", "2999-01-01"},
		Programs: []*program{
			{
				ID:   "charts:cmd/go",
				Name: "cmd/go",
				Charts: []*chart{
					{
						ID:   "charts:cmd/go:GOOS",
						Name: "GOOS",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "darwin", Value: 1},
						},
					},
					{
						ID:   "charts:cmd/go:GoVersion",
						Name: "GoVersion",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "go1.2", Value: 1},
							{Week: "2999-01-01", Key: "go1.19"},
						},
					},
					{
						ID:   "charts:cmd/go:main",
						Name: "main",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "main", Value: 1},
						},
					},
				},
			},
			{
				ID:   "charts:cmd/compiler",
				Name: "cmd/compiler",
			},
			{
				ID:   "charts:example.com/mod/pkg",
				Name: "example.com/mod/pkg",
				Charts: []*chart{
					{
						ID:   "charts:example.com/mod/pkg:Version",
						Name: "Version",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "v0.15", Value: 0},
							{Week: "2999-01-01", Key: "v2.3", Value: 2},
						},
					},
					{
						ID:   "charts:example.com/mod/pkg:GOOS",
						Name: "GOOS",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "darwin", Value: 2},
						},
					},
					{
						ID:   "charts:example.com/mod/pkg:GOARCH",
						Name: "GOARCH",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "amd64", Value: 1},
						},
					},
					{
						ID:   "charts:example.com/mod/pkg:GoVersion",
						Name: "GoVersion",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "go1.2", Value: 3},
							{Week: "2999-01-01", Key: "go1.19", Value: 1},
						},
					},
					{
						ID:   "charts:example.com/mod/pkg:flag",
						Name: "flag",
						Type: "partition",
						Data: []*datum{
							{Week: "2999-01-01", Key: "a", Value: 3},
							{Week: "2999-01-01", Key: "b", Value: 3},
							{Week: "2999-01-01", Key: "c", Value: 1},
						},
					},
				},
			},
		},
		NumReports: 1,
	}
	got := charts(cfg, "2999-01-01", "2999-01-01", exampleData, []float64{0.12345})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("charts = %+v\n, (-want +got): %v", got, diff)
	}
}

func TestNormalizeCounterName(t *testing.T) {
	testcases := []struct {
		name   string
		chart  graphName
		bucket bucketName
		want   bucketName
	}{
		{
			name:   "strip patch version for Version",
			chart:  "Version",
			bucket: "v0.15.3",
			want:   "v0.15",
		},
		{
			name:   "strip patch go version for Version",
			chart:  "Version",
			bucket: "go1.12.3",
			want:   "go1.12",
		},
		{
			name:   "concatenate devel for Version",
			chart:  "Version",
			bucket: "devel",
			want:   "devel",
		},
		{
			name:   "concatenate for GOOS",
			chart:  "GOOS",
			bucket: "darwin",
			want:   "darwin",
		},
		{
			name:   "concatenate for GOARCH",
			chart:  "GOARCH",
			bucket: "amd64",
			want:   "amd64",
		},
		{
			name:   "strip patch version for GoVersion",
			chart:  "GoVersion",
			bucket: "go1.12.3",
			want:   "go1.12",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCounterName(tc.chart, tc.bucket)
			if tc.want != got {
				t.Errorf("normalizeCounterName(%q, %q) = %q, want %q", tc.chart, tc.bucket, got, tc.want)
			}
		})
	}
}

func TestWriteCount(t *testing.T) {
	type keyValue struct {
		week    weekName
		program programName
		chart   graphName
		bucket  bucketName
		x       reportID
		value   int64
	}
	testcases := []struct {
		name   string
		inputs []keyValue
		want   []keyValue
	}{
		{
			name: "program version counter should have value",
			inputs: []keyValue{
				{"2987-07-01", "golang.org/x/tools/gopls", "Version", "v0.15.3", 0.00009, 1},
			},
			want: []keyValue{
				{"2987-07-01", "golang.org/x/tools/gopls", "Version", "v0.15.3", 0.00009, 1},
			},
		},
		{
			name: "only one count with same prefix and counter",
			inputs: []keyValue{
				{"2987-06-30", "cmd/go", "go/invocations", "go/invocations", 0.86995, 84},
			},
			want: []keyValue{
				{"2987-06-30", "cmd/go", "go/invocations", "go/invocations", 0.86995, 84},
			},
		},
		{
			name: "overwrite values when calling multiple times",
			inputs: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 1},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 2},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 3},
			},
			want: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 3},
			},
		},
		{
			name: "multiple counters",
			inputs: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 2},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "linux", 0.86018, 4},
			},
			want: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 2},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "linux", 0.86018, 4},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := make(data)
			for _, input := range tc.inputs {
				d.writeCount(input.week, input.program, input.chart, input.bucket, input.x, input.value)
			}

			for _, want := range tc.want {
				got := d[want.week][want.program][want.chart][want.bucket][want.x]
				if want.value != got {
					t.Errorf("d[%q][%q][%q][%q][%v] = %v, want %v", want.week, want.program, want.chart, want.bucket, want.x, got, want.value)
				}
			}
		})
	}
}

func TestParseDateRange(t *testing.T) {
	testcases := []struct {
		name      string
		url       string
		wantStart time.Time
		wantEnd   time.Time
		wantErr   bool
	}{
		{
			name:      "regular key start & end input",
			url:       "http://localhost:8082/chart/?start=2024-06-10&end=2024-06-17",
			wantStart: time.Date(2024, 06, 10, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 06, 17, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "regular key date input",
			url:       "http://localhost:8082/chart/?date=2024-06-11",
			wantStart: time.Date(2024, 06, 11, 0, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2024, 06, 11, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "malformatted value for start",
			url:     "http://localhost:8082/chart/?start=2024-066-01&end=2024-06-17",
			wantErr: true,
		},
		{
			name:    "malformatted value for start",
			url:     "http://localhost:8082/chart/?start=2024-06-10&end=2024-06-179",
			wantErr: true,
		},
		{
			name:    "end is earlier than start",
			url:     "http://localhost:8082/chart/?start=2024-06-17&end=2024-06-10",
			wantErr: true,
		},
		{
			name:    "have only start but missing end",
			url:     "http://localhost:8082/chart/?start=2024-06-01",
			wantErr: true,
		},
		{
			name:    "key date and start used together",
			url:     "http://localhost:8082/chart/?start=2024-06-17&date=2024-06-19",
			wantErr: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := url.Parse(tc.url)
			if err != nil {
				t.Fatalf("failed to parse url %q: %v", url, err)
			}

			gotStart, gotEnd, err := parseDateRange(url)
			if tc.wantErr && err == nil {
				t.Errorf("parseDateRange %v should return error but return nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("parseDateRange %v should return nil but return error: %v", tc.url, err)
			}

			if !tc.wantErr {
				if !gotStart.Equal(tc.wantStart) || !gotEnd.Equal(tc.wantEnd) {
					t.Errorf("parseDateRange(%s) = (%s, %s), want (%s, %s)", tc.url, gotStart.Format(telemetry.DateOnly), gotEnd.Format(telemetry.DateOnly), tc.wantStart.Format(telemetry.DateOnly), tc.wantEnd.Format(telemetry.DateOnly))
				}
			}
		})
	}
}
