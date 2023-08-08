// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graphconfig

import (
	"errors"
	"fmt"

	"golang.org/x/mod/semver"
)

// Validate checks that a graph config is complete and coherent, returning an
// error describing all problems encountered, or nil.
func Validate(cfg GraphConfig) error {
	var errs []error
	reportf := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}
	if cfg.Title == "" {
		reportf("title must be set")
	}
	if len(cfg.Issue) == 0 {
		reportf("at least one issue is required")
	}
	if cfg.Program == "" {
		reportf("program must be set")
	}
	if cfg.Counter == "" {
		reportf("counter must be set")
	}
	if cfg.Type == "" {
		reportf("type must be set")
	}
	if cfg.Depth < 0 {
		reportf("invalid depth %d: must be non-negative", cfg.Depth)
	}
	if cfg.Depth != 0 && cfg.Type != "stack" {
		reportf("depth can only be set for \"stack\" graph types")
	}
	if cfg.Version != "" && !semver.IsValid(cfg.Version) {
		reportf("%q is not valid semver", cfg.Version)
	}
	return errors.Join(errs...)
}
