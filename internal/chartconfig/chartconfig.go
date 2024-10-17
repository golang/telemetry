// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The chartconfig package defines configuration that specifies both the data
// that is uploaded to telemetry.go.dev, and the resulting charts that are
// rendered. Each record in [config.txt] defines the collection and aggregation
// of a group of related counters, in the custom format specified below. We
// refer to these records as "chart configs" (and represent them with the
// [ChartConfig] type), based on the principle that telemetry collection should
// be derived from views of the data that we want to see. For basic counters,
// this is indeed the case: chart configs correspond 1:1 with charts on
// telemetry.go.dev. For stack counters, we use the same format, but have no
// way to render them graphically.
//
// The format itself is based on the original telemetry blog post, though it
// has been modified slightly and may be subject to further change:
//
// https://research.swtch.com/telemetry-design#configuration
//
// # Chart records
//
// Chart config records consist of fields, comments, and whitespace. A field is
// defined by a line starting with a valid key, followed immediately by ":",
// and then a textual value, which cannot include the comment separator '#'.
//
// Comments start with '#', and extend to the end of the line.
//
// The following keys are supported. Any entry not marked as (optional) must be
// provided for the record to pass validation.
//
//   - counter: an expression for the group of counters this chart aggregates.
//     More detail on the syntax of this expression is provided below.
//   - title: the chart title.
//   - description: (optional) a longer description of the chart.
//   - issue: a Go issue tracker URL proposing the chart configuration.
//     Multiple issues may be provided by including additional 'issue:' lines.
//   - type: the chart type. Currently only partition and stack are supported.
//   - program: the package path of the program for which this chart applies.
//   - version: (optional) the first program version for which this chart
//     applies. Must be a valid semver value. If not provided, the chart
//     applies to all versions.
//   - depth: (optional) stack counters only; the maximum stack depth to collect
//   - error: (optional) the desired error rate for this chart, which
//     determines collection rate
//
// Multiple records are separated by "---" lines.
//
// # Counter expressions
//
// Each record must specify in its 'counter' field either a single counter, or
// a group of related counters.
//
// As described in the [counter documentation], we decompose counter names into
// two parts, separated by a ':'. The part before the ':' is referred to as the
// 'chart name', and the part after the ':' is the 'bucket'. So in the example
// gopls/gotoolchain:auto, gopls/gotoolchain is the chart name, and auto is the
// bucket name.
//
// In order to be grouped into a single chart, counters must share a common
// chart name, and so we specify the group of counters using the following
// compact syntax:
//
//	chartname:{bucket1,bucket2,bucket3}
//
// Which expands to the following group of counters:
//
//	chartname:bucket1
//	chartname:bucket2
//	chartname:bucket3
//
// # Chart types
//
// There are two supported chart types for the 'type' field:
//
//   - A 'partition' chart is a bar chart with one bar for each related counter.
//     The value of the bar is the aggregation of all counts for the program
//     over the applicable time period.
//   - A 'stack' chart is not a real chart. It just means that we want to
//     collect the given stack counter or group of stack counters.
//
// # Example
//
// Here is a fully worked example, including both partition and stack records:
//
//	# This config defines an ordinary counter.
//	counter: gopls/editor:{emacs,vim,vscode,other} # TODO(golang/go#34567): add more editors
//	title: Editor Distribution
//	description: measure editor distribution for gopls users.
//	type: partition
//	issue: https://go.dev/issue/12345
//	program: golang.org/x/tools/gopls
//	version: v1.0.0
//
//	---
//
//	# This config defines a stack counter.
//	counter: gopls/bug
//	title: Gopls bug reports.
//	description: Stacks of bugs encountered on the gopls server.
//	issue: https://go.dev/12345
//	issue: https://go.dev/23456 # increase stack depth
//	type: stack
//	program: golang.org/x/tools/gopls
//	depth: 10
//
// [config.txt]: https://go.googlesource.com/telemetry/+/refs/heads/master/internal/chartconfig/config.txt
// [counter documentation]: https://go.dev/doc/telemetry#counters
package chartconfig

// A ChartConfig defines the configuration for a single chart/collection on the
// telemetry server.
//
// See the package documentation for field definitions.
type ChartConfig struct {
	Title       string
	Description string
	Issue       []string
	Type        string
	Program     string
	Counter     string
	Depth       int
	Error       float64 // TODO(rfindley) is Error still useful?
	Version     string
}
