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
		xs      []float64
	}
	tests := []struct {
		name string
		args args
		want *chart
	}{
		{
			"versions counter",
			args{
				"example.com/mod/pkg",
				"Version",
				[]string{"v1.2.3", "v2.3.4"},
				[]float64{0.123456789, 0.987654321},
			},
			&chart{
				ID:   "charts:example.com/mod/pkg:Version",
				Name: "Version",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "v1.2",
						Value: 1,
					},
					{
						Week:  "2999-01-01",
						Key:   "v2.3",
						Value: 0.5,
					},
				},
			},
		},
		{
			"goos counter",
			args{
				"example.com/mod/pkg",
				"GOOS",
				[]string{"darwin", "linux"},
				[]float64{0.123456789, 0.987654321},
			},
			&chart{
				ID:   "charts:example.com/mod/pkg:GOOS",
				Name: "GOOS",
				Type: "partition",
				Data: []*datum{
					{
						Week:  "2999-01-01",
						Key:   "darwin",
						Value: 0.5,
					},
					{
						Week:  "2999-01-01",
						Key:   "linux",
						Value: 0.5,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := partition(dat, tt.args.program, tt.args.name, tt.args.buckets); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("histogram() = %v, want %v", got, tt.want)
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
							Value: 0.5,
						}},
					},
					{
						ID:   "charts:example.com/mod/pkg:GOARCH",
						Name: "GOARCH",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "amd64",
							Value: 0.5,
						}},
					},
					{
						ID:   "charts:example.com/mod/pkg:GoVersion",
						Name: "GoVersion",
						Type: "partition",
						Data: []*datum{{
							Week:  "2999-01-01",
							Key:   "go1.2",
							Value: 1,
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
