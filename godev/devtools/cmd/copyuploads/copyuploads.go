// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The copyuploads command copies uploads from GCS to the local filesystem
// storage, for use with local development of the worker.
//
// By default, this command copies the last 3 days of uploads from the
// dev-telemetry-uploaded bucket in GCS to the local filesystem bucket
// local-telemetry-uploaded, at which point this data will be available when
// running ./godev/cmd/worker with no arguments.
//
// This command requires read permission to the go-telemetry GCS buckets.
// TODO(rfindley): we could avoid the need for read permission by instead
// downloading the public merged reports, and reassembling the individual
// uploads.
//
// See --help for more details.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/telemetry/godev/internal/config"
	"golang.org/x/telemetry/godev/internal/storage"
)

var (
	bucket   = flag.String("bucket", "dev-telemetry-uploaded", "The bucket to copy from.")
	daysBack = flag.Int("days_back", 3, "The number of days back to copy")
	verbose  = flag.Bool("v", false, "If set, enable verbose logging.")
)

func main() {
	flag.Parse()

	if !strings.HasSuffix(*bucket, "-uploaded") {
		log.Fatal("-bucket must end in -uploaded")
	}

	cfg := config.NewConfig()
	ctx := context.Background()

	gcs, err := storage.NewGCSBucket(ctx, cfg.ProjectID, *bucket)
	if err != nil {
		log.Fatal(err)
	}
	fs, err := storage.NewFSBucket(ctx, cfg.LocalStorage, "local-telemetry-uploaded")
	if err != nil {
		log.Fatal(err)
	}

	// Copy files concurrently.
	const concurrency = 5
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	start := time.Now()
	for dayOffset := range *daysBack {
		date := start.AddDate(0, 0, -dayOffset)
		it := gcs.Objects(ctx, date.Format(time.DateOnly))
		for {
			name, err := it.Next()
			if errors.Is(err, storage.ErrObjectIteratorDone) {
				break
			}

			// Skip objects that already exist in local storage.
			dest := fs.Object(name)
			if _, err := os.Stat(dest.(*storage.FSObject).Filename()); err == nil {
				if *verbose {
					log.Printf("Skipping existing object %s", name)
				}
				continue
			}
			if *verbose {
				log.Printf("Starting copying object %s", name)
			}

			g.Go(func() error {
				if err != nil {
					return err
				}
				return storage.Copy(ctx, dest, gcs.Object(name))
			})
		}
	}

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}
