// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package godev exports the content dir as an embed.FS.
package godev

import (
	"embed"
)

//go:embed content third_party
var FS embed.FS
