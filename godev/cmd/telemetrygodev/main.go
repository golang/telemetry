// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Telemetrygodev serves the telemetry.go.dev website.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/godev"
	"golang.org/x/telemetry/godev/internal/content"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/storage"
	"golang.org/x/telemetry/godev/internal/unionfs"
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg := newConfig()
	store, err := uploadBucket(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	fsys := fsys(cfg.DevMode)
	cserv := content.Server(fsys)
	mux := http.NewServeMux()

	mux.Handle("/", cserv)
	mux.Handle("/upload/", handleUpload(store))

	fmt.Printf("server listening at http://localhost:%s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, middleware.Default(mux)))
}

func handleUpload(store storage.Store) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == "POST" {
			var report telemetry.Report
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				return err
			}
			// TODO: validate the report, checking that the counters and stacks contained are things
			// we want to collect and file name is something reasonable.
			// TODO: capture metrics for collisions.
			ctx := r.Context()
			name := fmt.Sprintf("%s/%g.json", report.Week, report.X)
			f, err := store.Writer(ctx, name)
			if err != nil {
				return err
			}
			defer f.Close()
			if err := json.NewEncoder(f).Encode(report); err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			return content.Text(w, "ok", http.StatusOK)
		}
		return content.ErrorStatus(http.StatusMethodNotAllowed)
	}
}

func fsys(fromOS bool) fs.FS {
	var f fs.FS = godev.FS
	if fromOS {
		f = os.DirFS(".")
	}
	f, err := unionfs.Sub(f, "content/telemetrygodev", "content/shared")
	if err != nil {
		log.Fatal(err)
	}
	return f
}

// uploadBucket returns a telemtry upload bucket for the given config.
func uploadBucket(ctx context.Context, cfg *config) (storage.Store, error) {
	if cfg.UseGCS && !cfg.onCloudRun() {
		if err := os.Setenv("STORAGE_EMULATOR_HOST", cfg.StorageEmulatorHost); err != nil {
			return nil, err
		}
	}
	if cfg.UseGCS {
		return storage.NewGCStore(ctx, cfg.ProjectID, cfg.UploadBucket)
	}
	return storage.NewFSStore(ctx, path.Join(cfg.LocalStorage, cfg.UploadBucket))
}
