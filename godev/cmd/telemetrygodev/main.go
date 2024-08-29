// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Telemetrygodev serves the telemetry.go.dev website.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/exp/slog"
	"golang.org/x/mod/semver"
	"golang.org/x/telemetry/godev/internal/config"
	"golang.org/x/telemetry/godev/internal/content"
	ilog "golang.org/x/telemetry/godev/internal/log"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/storage"
	"golang.org/x/telemetry/internal/chartconfig"
	tconfig "golang.org/x/telemetry/internal/config"
	contentfs "golang.org/x/telemetry/internal/content"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/unionfs"
)

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg := config.NewConfig()

	if cfg.UseGCS {
		// We are likely running on GCP. Use GCP logging JSON format.
		slog.SetDefault(slog.New(ilog.NewGCPLogHandler()))
	}

	handler := newHandler(ctx, cfg)

	fmt.Printf("server listening at http://:%s\n", cfg.ServerPort)
	log.Fatal(http.ListenAndServe(":"+cfg.ServerPort, handler))
}

// renderer implements shared template rendering for handlers below.
type renderer func(w http.ResponseWriter, tmpl string, page any) error

func newHandler(ctx context.Context, cfg *config.Config) http.Handler {
	buckets, err := storage.NewAPI(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	ucfg, err := tconfig.ReadConfig(cfg.UploadConfig)
	if err != nil {
		log.Fatal(err)
	}
	fsys := fsys(cfg.DevMode)
	mux := http.NewServeMux()

	render := func(w http.ResponseWriter, tmpl string, page any) error {
		return content.Template(w, fsys, tmpl, page, http.StatusOK)
	}

	logger := slog.Default()
	// TODO(rfindley): use Go 1.22 routing once 1.23 is released and we can bump
	// the go directive to 1.22.
	mux.Handle("/", handleRoot(render, fsys, buckets.Chart, logger))
	mux.Handle("/config", handleConfig(fsys, ucfg))
	// TODO(rfindley): restrict this routing to POST
	mux.Handle("/upload/", handleUpload(ucfg, buckets.Upload))
	mux.Handle("/charts/", handleCharts(render, buckets.Chart))
	mux.Handle("/data/", handleData(render, buckets.Merge))

	mw := middleware.Chain(
		middleware.Log(logger),
		middleware.Timeout(cfg.RequestTimeout),
		middleware.RequestSize(cfg.MaxRequestBytes),
		middleware.Recover(),
	)
	return mw(mux)
}

// breadcrumb holds a breadcrumb nav element.
//
// If Link is empty, breadcrumbs are rendered as plain text.
type breadcrumb struct {
	Link, Label string
}

type indexPage struct {
	ChartTitle string
	Charts     map[string]any
	ChartError string // if set, the error
}

func (indexPage) Breadcrumbs() []breadcrumb {
	return []breadcrumb{{Link: "/", Label: "Go Telemetry"}, {Label: "Home"}}
}

func handleRoot(render renderer, fsys fs.FS, chartBucket storage.BucketHandle, log *slog.Logger) content.HandlerFunc {
	// TODO(rfindley): handle static serving with a different route.
	cserv := content.Server(fsys)
	return func(w http.ResponseWriter, r *http.Request) error {
		if r.URL.Path != "/" {
			cserv.ServeHTTP(w, r)
			return nil
		}
		page := indexPage{}

		ctx := r.Context()
		var (
			chartDate string // end date of chart data
			chartObj  string // object name of chart file
		)
		it := chartBucket.Objects(ctx, "")
		for {
			obj, err := it.Next()
			if errors.Is(err, storage.ErrObjectIteratorDone) {
				break
			} else if err != nil {
				return err
			}
			date := strings.TrimSuffix(obj, ".json")
			if date == obj {
				// We have discussed eventually have nested subdirectories in the
				// charts bucket. Defensively check for json files.
				continue // not a chart object
			}
			// Chart objects may be for a single date (<date>.json), or for a date
			// span (<start>_<end>.json).
			_, end, aggregate := strings.Cut(date, "_")
			if aggregate {
				date = end
			}
			if date >= chartDate {
				chartDate = date
				// Prefer aggregate charts to daily charts, but consider the latest
				// available date.
				if aggregate || date > chartDate {
					chartObj = obj
				}
			}
		}
		if chartObj == "" {
			page.ChartError = "No data."
		} else {
			page.ChartTitle = chartTitle(chartObj)
			charts, err := loadCharts(ctx, chartObj, chartBucket)
			if err != nil {
				log.ErrorContext(ctx, fmt.Sprintf("error loading index charts: %v", err))
				page.ChartError = "Error loading charts."
			} else {
				page.Charts = charts
			}
		}
		return render(w, "index.html", page)
	}
}

func chartTitle(objName string) string {
	start, end, aggregate := strings.Cut(strings.TrimSuffix(objName, ".json"), "_")
	if aggregate {
		return fmt.Sprintf("Aggregate charts for %s to %s", start, end)
	}
	return fmt.Sprintf("Charts for %s", start)
}

type chartsPage []string

func (chartsPage) Breadcrumbs() []breadcrumb {
	return []breadcrumb{{Link: "/", Label: "Go Telemetry"}, {Label: "Charts"}}
}

func handleCharts(render renderer, chartBucket storage.BucketHandle) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		if p := strings.TrimPrefix(r.URL.Path, "/charts/"); p != "" {
			return handleChart(ctx, w, p, render, chartBucket)
		}
		it := chartBucket.Objects(ctx, "")
		var page chartsPage
		for {
			obj, err := it.Next()
			if errors.Is(err, storage.ErrObjectIteratorDone) {
				break
			} else if err != nil {
				return err
			}
			date := strings.TrimSuffix(obj, ".json")
			if date == obj {
				continue // not a chart object
			}
			page = append(page, date)
		}
		return render(w, "allcharts.html", page)
	}
}

type chartPage struct {
	Date       string
	ChartTitle string
	Charts     map[string]any
}

func (p chartPage) Breadcrumbs() []breadcrumb {
	return []breadcrumb{
		{Link: "/", Label: "Go Telemetry"},
		{Link: "/charts/", Label: "Charts"},
		{Label: p.Date},
	}
}

func handleChart(ctx context.Context, w http.ResponseWriter, date string, render renderer, chartBucket storage.BucketHandle) error {
	// TODO(rfindley): refactor to return a content.HandlerFunc once we can use Go 1.22 routing.
	page := chartPage{Date: date}
	var err error
	objName := date + ".json"
	page.ChartTitle = chartTitle(objName)
	page.Charts, err = loadCharts(ctx, objName, chartBucket)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return content.Status(w, http.StatusNotFound)
	} else if err != nil {
		return err
	}
	return render(w, "charts.html", page)
}

type dataPage struct {
	BucketURL string
	Dates     []string
}

func (dataPage) Breadcrumbs() []breadcrumb {
	return []breadcrumb{{Link: "/", Label: "Go Telemetry"}, {Label: "Data"}}
}

func handleData(render renderer, mergeBucket storage.BucketHandle) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		it := mergeBucket.Objects(r.Context(), "")
		var page dataPage
		page.BucketURL = mergeBucket.URI()
		for {
			obj, err := it.Next()
			if errors.Is(err, storage.ErrObjectIteratorDone) {
				break
			} else if err != nil {
				return err
			}
			date := strings.TrimSuffix(obj, ".json")
			if date == obj {
				continue // not a data object
			}
			page.Dates = append(page.Dates, date)
		}
		return render(w, "data.html", page)
	}
}

func loadCharts(ctx context.Context, chartObj string, bucket storage.BucketHandle) (map[string]any, error) {
	reader, err := bucket.Object(chartObj).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var charts map[string]any
	if err := json.Unmarshal(data, &charts); err != nil {
		return nil, err
	}
	return charts, nil
}

func handleUpload(ucfg *tconfig.Config, uploadBucket storage.BucketHandle) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if r.Method == "POST" {
			ctx := r.Context()
			var report telemetry.Report
			if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
				return content.Error(fmt.Errorf("invalid JSON payload: %v", err), http.StatusBadRequest)
			}
			if err := validate(&report, ucfg); err != nil {
				return content.Error(fmt.Errorf("invalid report: %v", err), http.StatusBadRequest)
			}
			// TODO: capture metrics for collisions.
			name := fmt.Sprintf("%s/%g.json", report.Week, report.X)
			f, err := uploadBucket.Object(name).NewWriter(ctx)
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
func validate(r *telemetry.Report, cfg *tconfig.Config) error {
	// TODO: reject/drop data arrived too early or too late.
	if _, err := time.Parse(telemetry.DateOnly, r.Week); err != nil {
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
			return fmt.Errorf("unknown program build %s@%q %q %s/%s", p.Program, p.Version, p.GoVersion, p.GOOS, p.GOARCH)
		}
		for c := range p.Counters {
			if !cfg.HasCounter(p.Program, c) {
				return fmt.Errorf("unknown counter %s", c)
			}
		}
		for s := range p.Stacks {
			prefix, _, _ := strings.Cut(s, "\n")
			if !cfg.HasStack(p.Program, prefix) {
				return fmt.Errorf("unknown stack %s", s)
			}
		}
	}
	return nil
}

func fsys(fromOS bool) fs.FS {
	var f fs.FS = contentfs.FS
	if fromOS {
		f = os.DirFS("internal/content")
		contentfs.RunESBuild(true)
	}
	f, err := unionfs.Sub(f, "telemetrygodev", "shared")
	if err != nil {
		log.Fatal(err)
	}
	return f
}

type configPage struct {
	Version      string
	ChartConfig  string
	UploadConfig string
}

func (configPage) Breadcrumbs() []breadcrumb {
	return []breadcrumb{{Link: "/", Label: "Go Telemetry"}, {Label: "Upload Configuration"}}
}

func handleConfig(fsys fs.FS, ucfg *tconfig.Config) content.HandlerFunc {
	ccfg := chartconfig.Raw()
	cfg := ucfg.UploadConfig
	version := "default"

	return func(w http.ResponseWriter, r *http.Request) error {
		cfgJSON, err := json.MarshalIndent(cfg, "", "\t")
		if err != nil {
			cfgJSON = []byte("unknown")
		}
		page := configPage{
			Version:      version,
			ChartConfig:  string(ccfg),
			UploadConfig: string(cfgJSON),
		}
		return content.Template(w, fsys, "config.html", page, http.StatusOK)
	}
}
