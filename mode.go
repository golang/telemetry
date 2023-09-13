// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry

import (
	it "golang.org/x/telemetry/internal/telemetry"
)

// Mode returns the current telemetry mode.
//
// The telemetry mode is a global value that controls both the collection and
// uploading of telemetry data. When collection is enabled, telemetry counters
// and stack information are written to the local file system and may be
// inspected with the [gotelemetry] command. When uploading is enabled, this
// anonymous data is periodically uploaded to remote servers.
//
// Possible mode values are:
//   - "on":    both collection and uploading are enabled
//   - "local": telemetry collection is enabled, but uploading is disabled
//   - "off":   both collection and uploading are disabled
//
// If an error occurs while reading the telemetry mode from the file system,
// Mode returns the default value "local".
//
// [gotelemetry]: https://pkg.go.dev/golang.org/x/telemetry/cmd/gotelemetry
func Mode() string {
	return it.Mode()
}

// SetMode sets the global telemetry mode to the given value.
//
// See the documentation of [Mode] for a description of the supported mode
// values.
//
// An error is returned if the provided mode value is invalid, or if an error
// occurs while persisting the mode value to the file system.
func SetMode(mode string) error {
	return it.SetMode(mode)
}
