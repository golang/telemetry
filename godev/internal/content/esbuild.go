// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9

package content

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

func esbuild(data []byte) *bytes.Buffer {
	// TODO: cache the output of this transform operation, minify the output.
	js := api.Transform(string(data), api.TransformOptions{
		Loader: api.LoaderTS,
	})
	output := bytes.NewBuffer(js.Code)
	if len(js.Warnings) != 0 {
		messages := api.FormatMessages(js.Warnings, api.FormatMessagesOptions{})
		for _, m := range messages {
			fmt.Fprintf(output, ";console.warn(`%s`);", strings.ReplaceAll(m, "`", "\\`"))
		}
	}
	if len(js.Errors) != 0 {
		messages := api.FormatMessages(js.Errors, api.FormatMessagesOptions{})
		for _, m := range messages {
			fmt.Fprintf(output, ";console.error(`%s`);", strings.ReplaceAll(m, "`", "\\`"))
		}
	}
	return output
}
