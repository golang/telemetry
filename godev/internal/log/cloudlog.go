// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"os"
	"time"

	"golang.org/x/exp/slog"
)

func NewGCPLogHandler() slog.Handler {
	return slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: gcpReplaceAttr,
		Level:       slog.LevelDebug,
	})
}

func gcpReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	switch a.Key {
	case slog.TimeKey:
		if a.Value.Kind() == slog.KindTime {
			a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
		}
	case slog.MessageKey:
		a.Key = "message"
	case slog.LevelKey:
		a.Key = "severity"
	case slog.SourceKey:
		a.Key = "logging.googleapis.com/sourceLocation"
	case "traceID":
		a.Key = "logging.googleapis.com/trace"
	case "spanID":
		a.Key = "logging.googleapis.com/spanId"
	}
	return a
}
