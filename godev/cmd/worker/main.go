// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
	"time"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/godev"
	"golang.org/x/telemetry/godev/internal/content"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/storage"
	"golang.org/x/telemetry/godev/internal/unionfs"
	tconfig "golang.org/x/telemetry/internal/config"
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg := newConfig()
	buckets, err := buckets(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	ucfg, err := tconfig.ReadConfig(cfg.UploadConfig)
	if err != nil {
		log.Fatal(err)
	}
	fsys := fsys(cfg.DevMode)
	cserv := content.Server(fsys)
	mux := http.NewServeMux()

	mux.Handle("/", cserv)
	mux.Handle("/merge/", handleMerge(ucfg, buckets))

	mw := middleware.Chain(
		middleware.Log,
		middleware.Timeout(cfg.RequestTimeout),
		middleware.RequestSize(cfg.MaxRequestBytes),
		middleware.Recover,
	)

	fmt.Printf("server listening at http://localhost:%s\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mw(mux)))
}

// TODO: monitor duration and processed data volume.
func handleMerge(cfg *tconfig.Config, s *stores) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		date := r.URL.Query().Get("date")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return content.Error(err, http.StatusBadRequest)
		}
		objs, err := s.upload.List(ctx, date)
		if err != nil {
			return err
		}
		mergeWriter, err := s.merge.Writer(ctx, date)
		if err != nil {
			return err
		}
		defer mergeWriter.Close()
		encoder := json.NewEncoder(mergeWriter)
		for _, o := range objs {
			reader, err := s.upload.Reader(ctx, o)
			if err != nil {
				return err
			}
			defer reader.Close()
			var report telemetry.Report
			if err := json.NewDecoder(reader).Decode(&report); err != nil {
				return err
			}
			if err := encoder.Encode(report); err != nil {
				return err
			}
			if err := reader.Close(); err != nil {
				return err
			}
		}
		if err := mergeWriter.Close(); err != nil {
			return err
		}
		msg := fmt.Sprintf("merged %d reports into %s/%s", len(objs), s.merge.Location(), date)
		return content.Text(w, msg, http.StatusOK)
	}
}

func fsys(fromOS bool) fs.FS {
	var f fs.FS = godev.FS
	if fromOS {
		f = os.DirFS(".")
	}
	f, err := unionfs.Sub(f, "content/worker", "content/shared")
	if err != nil {
		log.Fatal(err)
	}
	return f
}

type stores struct {
	upload storage.Store
	merge  storage.Store
}

func buckets(ctx context.Context, cfg *config) (*stores, error) {
	if cfg.UseGCS && !cfg.onCloudRun() {
		if err := os.Setenv("STORAGE_EMULATOR_HOST", cfg.StorageEmulatorHost); err != nil {
			return nil, err
		}
	}
	var upload storage.Store
	var merge storage.Store
	var err error
	if cfg.UseGCS {
		upload, err = storage.NewGCStore(ctx, cfg.ProjectID, cfg.UploadBucket)
		if err != nil {
			return nil, err
		}
		merge, err = storage.NewGCStore(ctx, cfg.ProjectID, cfg.MergedBucket)
		if err != nil {
			return nil, err
		}
	} else {
		upload, err = storage.NewFSStore(ctx, cfg.LocalStorage, cfg.UploadBucket)
		if err != nil {
			return nil, err
		}
		merge, err = storage.NewFSStore(ctx, cfg.LocalStorage, cfg.MergedBucket)
		if err != nil {
			return nil, err
		}
	}
	return &stores{upload, merge}, nil
}
