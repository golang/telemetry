// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gotelemetry provides utilities to manage telemetry collection settings.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/telemetry/cmd/gotelemetry/internal/csv"
	"golang.org/x/telemetry/cmd/gotelemetry/internal/view"
	"golang.org/x/telemetry/internal/counter"
	it "golang.org/x/telemetry/internal/telemetry"
)

func main() {
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printSetting()
		return
	}
	switch cmd := args[0]; cmd {
	case "on", "off":
		if err := setMode(args); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			usage()
			os.Exit(1)
		} else if cmd == "on" {
			// We could perhaps only show the telemetry on message when the mode goes
			// from off->on (i.e. check the previous state before calling setMode),
			// but that seems like an unnecessary optimization.
			fmt.Fprintln(os.Stderr, telemetryOnMessage())
		}
	case "dump":
		counterDump(args[1:]...)
	case "help":
		flag.CommandLine.SetOutput(os.Stdout)
		flag.Usage()
	case "view":
		view.Start()
	case "csv":
		csv.Csv()
	default:
		flag.Usage()
	}
}

func printSetting() {
	fmt.Println("[-h for help]")
	fmt.Printf("mode: %s\n", it.Mode())
	fmt.Println()
	fmt.Println("modefile: ", it.ModeFile)
	fmt.Println("localdir: ", it.LocalDir)
	fmt.Println("uploaddir:", it.UploadDir)
}

func setMode(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("expected 2 args for set, not %d", len(args))
	}
	return it.SetMode(args[0])
}

func telemetryOnMessage() string {
	reportDate := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	return fmt.Sprintf(`Telemetry uploading is now enabled and may be sent to https://telemetry.go.dev/ starting %s. Uploaded data is used to help improve the Go toolchain and related tools, and it will be published as part of a public dataset.

For more details, see https://telemetry.go.dev/privacy.
This data is collected in accordance with the Google Privacy Policy (https://policies.google.com/privacy).

To disable telemetry uploading, run “gotelemetry off”`, reportDate)
}

func usage() {
	w := flag.CommandLine.Output()
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "\tgotelemetry")
	fmt.Fprintln(w, "\tgotelemetry on")
	fmt.Fprintln(w, "\tgotelemetry off")
	fmt.Fprintln(w, "\tgotelemetry dump [file1 file2 ...]")
	fmt.Fprintln(w, "\tgotelemetry view (runs web server)")
	fmt.Fprintln(w, "\tgotelemetry csv (prints all known counters)")
	fmt.Fprintln(w, "\tgotelemetry help")
	fmt.Fprintln(w, "Flags:")
	flag.CommandLine.PrintDefaults()
}

func counterDump(args ...string) {
	if len(args) == 0 {
		localdir := it.LocalDir
		fi, err := os.ReadDir(localdir)
		if err != nil && len(args) == 0 {
			log.Fatal(err)
		}
		for _, f := range fi {
			args = append(args, filepath.Join(localdir, f.Name()))
		}
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
