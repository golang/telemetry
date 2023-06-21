// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package counter implements a simple counter system for collecting
// totally public telemetry data.
//
// There are two kinds of counters, simple counters and stack counters.
// Simple counters are created by New(<counter-name>).
// Stack counters are created by NewStack(<counter-name>, depth).
// Both are incremented by calling Inc().
//
// Counter files are stored in LocalDir(). Their content can be accessed
// by Parse().
//
// Simple counters are very cheap. Stack counters are not collected if
// go telemetry is disabled ("off").
// (Stack counters are implemented as a set of regular counters whose names
// are the concatenation of the name and the stack trace. There is an upper
// limit on the size of this name, about 256 bytes. If the name is too long
// the stack will be truncated and "truncated" appended.)
//
// Counter files are turned into reports by the upload package.
// This happens weekly, except for the first time a counter file is
// created. Then it happens on a random day of the week more than 7 days
// in the future. After that the counter files expire weekly on the same day of
// the week.
package counter
