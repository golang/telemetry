// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gotelemetry provides utilities to manage telemetry collection settings.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/telemetry/internal/telemetry"
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println(telemetry.LookupMode())
		return
	}
	switch cmd := args[0]; cmd {
	case "set":
		if err := setMode(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			usage(os.Stderr)
			os.Exit(1)
		}
	case "help":
		usage(os.Stdout)
	}
}

func setMode(args []string) error {
	if len(args) != 2 {
		return errors.New("unexpected number of arguments for 'set'")
	}
	return telemetry.SetMode(args[1])
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "\tgotelemetry")
	fmt.Fprintln(w, "\tgotelemetry set <on|off|local>")
	fmt.Fprintln(w, "\tgotelemetry help")
}
