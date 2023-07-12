// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Telemetrygodev serves the telemetry.go.dev website.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/telemetry"
	"golang.org/x/telemetry/godev"
	"golang.org/x/telemetry/godev/internal/content"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/storage"
	"golang.org/x/telemetry/godev/internal/unionfs"
	"golang.org/x/telemetry/godev/internal/upload"
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg := newConfig()
	store, err := uploadBucket(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	ucfg, err := upload.ReadConfig(cfg.UploadConfig)
	if err != nil {
		log.Fatal(err)
	}
	fsys := fsys(cfg.DevMode)
	cserv := content.Server(fsys)
	mux := http.NewServeMux()

	mux.Handle("/", cserv)
	mux.Handle("/upload/", handleUpload(ucfg, store))

	mw := middleware.Chain(
		middleware.Log,
		middleware.RequestSize(cfg.MaxRequestBytes),
		middleware.Recover,
	)

	fmt.Printf("server listening at http://localhost:%s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mw(mux)))
}

func handleUpload(ucfg *upload.Config, store storage.Store) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == "POST" {
			var report telemetry.Report
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				return content.Error(err, http.StatusBadRequest)
			}
			if err := validate(&report, ucfg); err != nil {
				return content.Error(err, http.StatusBadRequest)
			}
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
			return content.Status(w, http.StatusOK)
		}
		return content.Status(w, http.StatusMethodNotAllowed)
	}
}

// validate validates the telemetry report data against the latest config.
func validate(r *telemetry.Report, cfg *upload.Config) error {
	// TODO: reject/drop data arrived too early or too late.
	if _, err := time.Parse("2006-01-02", r.Week); err != nil {
		return fmt.Errorf("invalid week %s", r.Week)
	}
	if !semver.IsValid(r.Config) {
		return fmt.Errorf("invalid config %s", r.Config)
	}
	if r.X == 0 {
		return fmt.Errorf("invalid X %g", r.X)
	}
	// TODO: We can probably keep known programs and counters even when a report
	// includes something that has been removed from the latest config.
	for _, p := range r.Programs {
		if !cfg.HasGOARCH(p.GOARCH) ||
			!cfg.HasGOOS(p.GOOS) ||
			!cfg.HasGoVersion(p.GoVersion) ||
			!cfg.HasProgram(p.Program) ||
			!cfg.HasVersion(p.Program, p.Version) {
			return fmt.Errorf("unknown program build %s@%s %s %s/%s", p.Program, p.Version, p.GoVersion, p.GOOS, p.GOARCH)
		}
		for c := range p.Counters {
			if !cfg.HasCounter(p.Program, c) {
				return fmt.Errorf("unknown counter %s", c)
			}
		}
		for s := range p.Stacks {
			if !cfg.HasStack(p.Program, s) {
				return fmt.Errorf("unknown stack %s", s)
			}
		}
	}
	return nil
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
