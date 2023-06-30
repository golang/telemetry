// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gotelemetry provides utilities to manage telemetry collection settings.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/telemetry/internal/counter"
	"golang.org/x/telemetry/internal/telemetry"
)

func main() {
	log.SetFlags(0)
	flag.Parse()
	flag.Usage = func() { usage(os.Stderr) }

	args := flag.Args()
	if len(args) == 0 {
		printSetting()
		return
	}
	switch cmd := args[0]; cmd {
	case "set":
		if err := setMode(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			usage(os.Stderr)
			os.Exit(1)
		}
	case "dump":
		counterDump(args[1:]...)
	case "help":
		usage(os.Stdout)
	}
}

func printSetting() {
	fmt.Println(telemetry.Mode())
	fmt.Println()
	fmt.Println("modefile: ", telemetry.ModeFile)
	fmt.Println("localdir: ", telemetry.LocalDir)
	fmt.Println("uploaddir:", telemetry.UploadDir)
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
	fmt.Fprintln(w, "\tgotelemetry dump [file1 file2 ...]")
	fmt.Fprintln(w, "\tgotelemetry help")
}

func counterDump(args ...string) {
	localdir := telemetry.LocalDir
	fi, err := os.ReadDir(localdir)
	if err != nil && len(args) == 0 {
		log.Fatal(err)
	}
	for _, f := range fi {
		args = append(args, filepath.Join(localdir, f.Name()))
	}
	for _, file := range args {
		if !strings.HasSuffix(file, ".count") {
			log.Printf("%s: not a counter file, skipping", file)
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("%v, skipping", err)
			continue
		}
		f, err := counter.Parse(file, data)
		if err != nil {
			log.Printf("%v, skipping", err)
			continue
		}
		js, err := json.MarshalIndent(f, "", "\t")
		if err != nil {
			log.Printf("%s: failed to print - %v", file, err)
		}
		fmt.Printf("-- %v --\n%s\n", file, js)
	}
}
