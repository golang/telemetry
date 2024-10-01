// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The copyuploads command copies uploads from GCS to the local filesystem
// storage, for use with local development of the worker.
//
// By default, this command copies the last 3 days of uploads from
// telemetry.go.dev to the local filesystem bucket local-telemetry-uploaded, at
// which point this data will be available when running ./godev/cmd/worker with
// no arguments.
//
// See --help for more details.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/telemetry/godev/internal/config"
	"golang.org/x/telemetry/godev/internal/storage"
)

var (
	daysBack = flag.Int("days_back", 3, "The number of days back to copy")
	verbose  = flag.Bool("v", false, "If set, enable verbose logging.")
)

func main() {
	flag.Parse()

	cfg := config.NewConfig()
	ctx := context.Background()

	fs, err := storage.NewFSBucket(ctx, cfg.LocalStorage, "local-telemetry-uploaded")
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	for dayOffset := range *daysBack {
		date := start.AddDate(0, 0, -dayOffset-1) // today's merged reports may not yet be available
		dateString := date.Format(time.DateOnly)
		byFile, err := downloadData(dateString)
		if err != nil {
			log.Fatalf("Downloading data for %s: %v", dateString, err)
		}
		for name, content := range byFile {
			// Skip objects that already exist in local storage.
			dest := fs.Object(path.Join(dateString, name))
			if _, err := os.Stat(dest.(*storage.FSObject).Filename()); err == nil {
				if *verbose {
					log.Printf("Skipping existing object %s", name)
				}
				continue
			}
			if *verbose {
				log.Printf("Starting copying object %s", name)
			}
			w, err := dest.NewWriter(ctx)
			if err != nil {
				log.Fatal(err)
			}
			if _, err := io.Copy(w, bytes.NewReader(content)); err != nil {
				log.Fatal(err)
			}
		}
	}
}

// downloadData downloads the merged telemetry data for the given date string
// (which must be in time.DateOnly format), and splits it back into individual
// uploaded files, keyed by their original name (<X>.json).
func downloadData(dateString string) (map[string][]byte, error) {
	url := fmt.Sprintf("https://storage.googleapis.com/prod-telemetry-merged/%s.json", dateString)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("downloading %s failed with status %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	byFile := make(map[string][]byte)
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue // defensive: skip empty lines
		}
		var x struct{ X float64 }
		if err := json.Unmarshal(line, &x); err != nil {
			return nil, err
		}
		file := fmt.Sprintf("%g.json", x.X)
		byFile[file] = append(line, '\n') // uploaded data is newline terminated
	}
	return byFile, nil
}
