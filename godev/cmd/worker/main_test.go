// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/telemetry/internal/config"
	"golang.org/x/telemetry/internal/telemetry"
)

func Test_nest(t *testing.T) {
	type args struct {
		reports []*telemetry.Report
	}
	tests := []struct {
		name string
		args args
		want data
	}{
		{
			"single report",
			args{
				[]*telemetry.Report{
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
			data{
				weekKey{"2999-01-01"}: {
					programKey{"example.com/mod/pkg"}: {
						graphKey{"Version"}: {
							counterKey{"Version"}: {
								xKey{0.1234567890}: 1,
							},
							counterKey{"Version:v1.2"}: {
								xKey{0.1234567890}: 1,
							},
						},
						graphKey{"GOOS"}: {
							counterKey{"GOOS"}: {
								xKey{0.1234567890}: 1,
							},
							counterKey{"GOOS:darwin"}: {
								xKey{0.1234567890}: 1,
							},
						},
						graphKey{"GOARCH"}: {
							counterKey{"GOARCH"}: {
								xKey{0.1234567890}: 1,
							},
							counterKey{"GOARCH:arm64"}: {
								xKey{0.1234567890}: 1,
							},
						},
						graphKey{"GoVersion"}: {
							counterKey{"GoVersion"}: {
								xKey{0.1234567890}: 1,
							},
							counterKey{"GoVersion:go1.2"}: {
								xKey{0.1234567890}: 1,
							},
						},
						graphKey{"main"}: {
							counterKey{"main"}: {
								xKey{0.1234567890}: 1,
							},
						},
						graphKey{"flag"}: {
							counterKey{"flag"}: {
								xKey{0.1234567890}: 5,
							},
							counterKey{"flag:a"}: {
								xKey{0.1234567890}: 2,
							},
							counterKey{"flag:b"}: {
								xKey{0.1234567890}: 3,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nest(tt.args.reports)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("pivot() = %v, want %v", got, tt.want)
			}
		})
	}
}

var reports = []*telemetry.Report{
	{
		Week:     "2999-01-01",
		LastWeek: "2998-01-01",
		X:        0.123456789,
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
		},
		Config: "v0.0.1",
	},
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
		},
		Config: "v0.0.1",
	},
	{
		Week:     "2999-01-01",
		LastWeek: "2998-01-01",
		X:        0.987654321,
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

func Test_histogram(t *testing.T) {
	dat := nest(reports)
	type args struct {
		program string
		name    string
		buckets []string
		xs      []float64
	}
	tests := []struct {
		name string
		args args
		want *chart
	}{
		{
			"flag histogram",
			args{
				"example.com/mod/pkg",
				"flag:{a,b,c}",
				[]string{"flag:a", "flag:b", "flag:c"},
				[]float64{0.123456789, 0.987654321},
			},
			&chart{
				ID:   "charts:example.com/mod/pkg:flag:{a,b,c}",
				Name: "flag:{a,b,c}",
				Type: "histogram",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "a",
						Value: 0.4,
					},
					{
						Week:  "2999-01-01",
						Key:   "a",
						Value: 0.4166666666666667,
					},
					{
						Week:  "2999-01-01",
						Key:   "b",
						Value: 0.6,
					},
					{
						Week:  "2999-01-01",
						Key:   "b",
						Value: 0.5,
					},
					{
						Week:  "2999-01-01",
						Key:   "c",
						Value: 0,
					},
					{
						Week:  "2999-01-01",
						Key:   "c",
						Value: 0.08333333333333333,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := histogram(dat, tt.args.program, tt.args.name, tt.args.buckets, tt.args.xs); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("histogram() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_partition(t *testing.T) {
	dat := nest(reports)
	type args struct {
		program string
		name    string
		buckets []string
	}
	tests := []struct {
		name string
		args args
		want *chart
	}{
		{
			name: "major.minor.patch version counter",
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []string{"v1.2.3", "v2.3.4"},
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
						Value: 1,
					},
				},
			},
		},
		{
			name: "major.minor version counter should have same result as major.minor.patch",
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []string{"v1.2", "v2.3"},
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
						Value: 1,
					},
				},
			},
		},
		{
			name: "duplicated counter should be ignored",
			args: args{
				program: "example.com/mod/pkg",
				name:    "Version",
				buckets: []string{"v1.2.3", "v2.3.4", "v1.2.3"},
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
						Value: 1,
					},
				},
			},
		},
		{
			name: "goos counter",
			args: args{
				program: "example.com/mod/pkg",
				name:    "GOOS",
				buckets: []string{"darwin", "linux"},
			},
			want: &chart{
				ID:   "charts:example.com/mod/pkg:GOOS",
				Name: "GOOS",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "darwin",
						Value: 1,
					},
					{
						Week:  "2999-01-01",
						Key:   "linux",
						Value: 1,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dat.partition(tt.args.program, tt.args.name, tt.args.buckets); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("partition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_charts(t *testing.T) {
	dat := nest(reports)
	cfg := &config.Config{
		UploadConfig: &telemetry.UploadConfig{
			GOOS:       []string{"darwin"},
			GOARCH:     []string{"amd64"},
			GoVersion:  []string{"go1.2.3"},
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
					Name:     "example.com/mod/pkg",
					Versions: []string{"v0.15.0"},
					Counters: []telemetry.CounterConfig{{
						Name: "count2",
					}},
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
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "darwin",
							Value: 1,
						}},
					},
					{
						ID:   "charts:cmd/go:GOARCH",
						Name: "GOARCH",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "amd64",
							Value: 0,
						}},
					},
					{
						ID:   "charts:cmd/go:GoVersion",
						Name: "GoVersion",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "go1.2",
							Value: 1,
						}},
					},
					{
						ID:   "charts:cmd/go:main",
						Name: "main",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "main",
							Value: 1,
						}},
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
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "v0.15",
							Value: 0,
						}},
					},
					{
						ID:   "charts:example.com/mod/pkg:GOOS",
						Name: "GOOS",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "darwin",
							Value: 1,
						}},
					},
					{
						ID:   "charts:example.com/mod/pkg:GOARCH",
						Name: "GOARCH",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "amd64",
							Value: 1,
						}},
					},
					{
						ID:   "charts:example.com/mod/pkg:GoVersion",
						Name: "GoVersion",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "go1.2",
							Value: 2,
						}},
					},
				},
			},
		},
		NumReports: 1,
	}
	got := charts(cfg, "2999-01-01", dat, []float64{0.12345})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("charts = %+v\n, (-want +got): %v", got, diff)
	}
}

func TestNormalizeCounterName(t *testing.T) {
	testcases := []struct {
		name    string
		prefix  string
		counter string
		want    string
	}{
		{
			name:    "strip patch version for Version",
			prefix:  "Version",
			counter: "v0.15.3",
			want:    "Version:v0.15",
		},
		{
			name:    "strip patch go version for Version",
			prefix:  "Version",
			counter: "go1.12.3",
			want:    "Version:go1.12",
		},
		{
			name:    "concatenate devel for Version",
			prefix:  "Version",
			counter: "devel",
			want:    "Version:devel",
		},
		{
			name:    "concatenate for GOOS",
			prefix:  "GOOS",
			counter: "darwin",
			want:    "GOOS:darwin",
		},
		{
			name:    "concatenate for GOARCH",
			prefix:  "GOARCH",
			counter: "amd64",
			want:    "GOARCH:amd64",
		},
		{
			name:    "strip patch version for GoVersion",
			prefix:  "GoVersion",
			counter: "go1.12.3",
			want:    "GoVersion:go1.12",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeCounterName(tc.prefix, tc.counter)
			if tc.want != got {
				t.Errorf("normalizeCounterName(%q, %q) = %q, want %q", tc.prefix, tc.counter, got, tc.want)
			}
		})
	}
}

func TestWriteCount(t *testing.T) {
	type keyValue struct {
		week, program, prefix, counter string
		x                              float64
		value                          int64
	}
	testcases := []struct {
		name   string
		inputs []keyValue
		wants  []keyValue
	}{
		{
			name: "program version counter should have value",
			inputs: []keyValue{
				{"2987-07-01", "golang.org/x/tools/gopls", "Version", "v0.15.3", 0.00009, 1},
			},
			wants: []keyValue{
				{"2987-07-01", "golang.org/x/tools/gopls", "Version", "Version:v0.15", 0.00009, 1},
				{"2987-07-01", "golang.org/x/tools/gopls", "Version", "Version", 0.00009, 1},
			},
		},
		{
			name: "only one count with same prefix and counter",
			inputs: []keyValue{
				{"2987-06-30", "cmd/go", "go/invocations", "go/invocations", 0.86995, 84},
			},
			wants: []keyValue{
				{"2987-06-30", "cmd/go", "go/invocations", "go/invocations", 0.86995, 84},
			},
		},
		{
			name: "sum together when calling multiple times",
			inputs: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 1},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 2},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 3},
			},
			wants: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "GOOS:windows", 0.86018, 6},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "GOOS", 0.86018, 6},
			},
		},
		{
			name: "sum together when the prefix is the same",
			inputs: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 1},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "windows", 0.86018, 2},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "linux", 0.86018, 4},
			},
			wants: []keyValue{
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "GOOS:windows", 0.86018, 3},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "GOOS:linux", 0.86018, 4},
				{"2987-06-30", "golang.org/x/tools/gopls", "GOOS", "GOOS", 0.86018, 7},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			d := make(data)
			for _, input := range tc.inputs {
				d.writeCount(input.week, input.program, input.prefix, input.counter, input.x, input.value)
			}

			for _, want := range tc.wants {
				got, _ := d.readCount(want.week, want.program, want.prefix, want.counter, want.x)
				if want.value != got {
					t.Errorf("d[%q][%q][%q][%q][%v] = %v, want %v", want.week, want.program, want.prefix, want.counter, want.x, got, want.value)
				}
			}
		})
	}
}
