// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"golang.org/x/telemetry"
	"golang.org/x/telemetry/godev"
	"golang.org/x/telemetry/godev/internal/config"
	"golang.org/x/telemetry/godev/internal/content"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/storage"
	"golang.org/x/telemetry/godev/internal/unionfs"
	tconfig "golang.org/x/telemetry/internal/config"
)

const defaultPort = "8082"

func main() {
	flag.Parse()
	ctx := context.Background()
	cfg := config.NewConfig()
	buckets, err := storage.NewAPI(ctx, cfg)
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
	mux.Handle("/chart/", handleChart(ucfg, buckets))

	mw := middleware.Chain(
		middleware.Log,
		middleware.Timeout(cfg.RequestTimeout),
		middleware.RequestSize(cfg.MaxRequestBytes),
		middleware.Recover,
	)

	port := cfg.Port
	if port == "" {
		port = defaultPort
	}
	fmt.Printf("server listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mw(mux)))
}

// TODO: monitor duration and processed data volume.
func handleMerge(cfg *tconfig.Config, s *storage.API) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		date := r.URL.Query().Get("date")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return content.Error(err, http.StatusBadRequest)
		}
		it := s.Upload.Objects(ctx, date)
		mergeWriter, err := s.Merge.Object(date + ".json").NewWriter(ctx)
		if err != nil {
			return err
		}
		defer mergeWriter.Close()
		encoder := json.NewEncoder(mergeWriter)
		var count int
		for {
			obj, err := it.Next()
			if errors.Is(err, storage.ErrObjectIteratorDone) {
				break
			}
			if err != nil {
				return err
			}
			count++
			reader, err := s.Upload.Object(obj).NewReader(ctx)
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
		msg := fmt.Sprintf("merged %d reports into %s/%s", count, s.Merge.URI(), date)
		return content.Text(w, msg, http.StatusOK)
	}
}

func handleChart(cfg *tconfig.Config, s *storage.API) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		// TODO: use start date and end date to create a timeseries of data.
		date := r.URL.Query().Get("date")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return content.Error(err, http.StatusBadRequest)
		}
		in, err := s.Merge.Object(date + ".json").NewReader(ctx)
		if errors.Is(err, storage.ErrObjectNotExist) {
			return content.Error(err, http.StatusNotFound)
		}
		if err != nil {
			return err
		}
		defer in.Close()

		var reports []*telemetry.Report
		var xs []float64
		scanner := bufio.NewScanner(in)
		for scanner.Scan() {
			var report telemetry.Report
			if err := json.Unmarshal(scanner.Bytes(), &report); err != nil {
				return err
			}
			reports = append(reports, &report)
			xs = append(xs, report.X)
		}
		if err := in.Close(); err != nil {
			return err
		}

		data := nest(reports)
		charts := charts(cfg, date, data, xs)
		obj := fmt.Sprintf("%s.json", date)
		out, err := s.Chart.Object(obj).NewWriter(ctx)
		if err != nil {
			return err
		}
		defer out.Close()

		if err := json.NewEncoder(out).Encode(charts); err != nil {
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}

		msg := fmt.Sprintf("processed %d reports into %s", len(reports), s.Chart.URI()+"/"+obj)
		return content.Text(w, msg, http.StatusOK)
	}
}

type chartdata struct {
	DateRange  [2]string
	Programs   []*program
	NumReports int
}

type program struct {
	ID     string
	Name   string
	Charts []*chart
}

type chart struct {
	ID   string
	Name string
	Type string
	Data []*datum
}

func (c *chart) String() string {
	bytes, _ := json.Marshal(c)
	return string(bytes)
}

type datum struct {
	Week  string
	Key   string
	Value float64
}

func charts(cfg *tconfig.Config, date string, d data, xs []float64) *chartdata {
	result := &chartdata{DateRange: [2]string{date, date}, NumReports: len(xs)}
	for _, p := range cfg.Programs {
		prog := &program{ID: "charts:" + p.Name, Name: p.Name}
		result.Programs = append(result.Programs, prog)
		if p.Name != "cmd/go" {
			prog.Charts = append(prog.Charts,
				partition(d, p.Name, "Version", p.Versions, xs),
			)
		}
		prog.Charts = append(prog.Charts,
			partition(d, p.Name, "GOOS", cfg.GOOS, xs),
			partition(d, p.Name, "GOARCH", cfg.GOARCH, xs),
			partition(d, p.Name, "GoVersion", cfg.GoVersion, xs),
		)
		for _, c := range p.Counters {
			// TODO: add support for histogram counters by getting the counter type
			// from the graph config.
			prog.Charts = append(prog.Charts,
				partition(d, p.Name, c.Name, tconfig.Expand(c.Name), xs),
			)
		}
	}
	return result
}

func histogram(dat data, program, name string, counters []string, xs []float64) *chart {
	count := &chart{
		ID:   "charts:" + program + ":" + name,
		Name: name,
		Type: "histogram",
	}
	for wk := range dat {
		for _, c := range counters {
			prefix, _ := parts(c)
			pk := programKey{program}
			gk := graphKey{prefix}
			ck := counterKey{c}

			// TODO: consider using pre-computed distribution buckets to reduce the size
			// of the output.
			for _, x := range xs {
				xk := xKey{x}
				v := dat[wk][pk][gk][ck][xk]
				total := dat[wk][pk][gk][counterKey{gk.prefix}][xk]
				_, bucket := parts(c)
				if total == 0 {
					d := &datum{
						Week: wk.date,
						Key:  bucket,
					}
					count.Data = append(count.Data, d)
					continue
				}
				d := &datum{
					Week:  wk.date,
					Key:   bucket,
					Value: float64(v) / float64(total),
				}
				count.Data = append(count.Data, d)
			}
		}
	}
	return count
}

func partition(dat data, program, name string, counters []string, xs []float64) *chart {
	count := &chart{
		ID:   "charts:" + program + ":" + name,
		Name: name,
		Type: "partition",
	}
	pk := programKey{program}
	prefix, _ := parts(name)
	gk := graphKey{prefix}
	for wk := range dat {
		// TODO: when should this be number of reports?
		// total := len(xs)
		total := len(dat[wk][pk][gk][counterKey{gk.prefix}])
		if total == 0 {
			return nil
		}
		// We group versions into major minor buckets, we must skip
		// major minor versions we've already added to the dataset.
		seen := make(map[string]bool)
		for _, b := range counters {
			counter := fixCounter(name, b)
			if seen[counter] {
				continue
			}
			seen[counter] = true
			ck := counterKey{counter}
			// number of reports where count prefix:bucket > 0
			n := len(dat[wk][pk][gk][ck])
			_, bucket := parts(counter)
			d := &datum{
				Week:  wk.date,
				Key:   bucket,
				Value: float64(n) / float64(total),
			}
			count.Data = append(count.Data, d)
		}
	}
	return count
}

type weekKey struct {
	date string
}

type programKey struct {
	program string
}

type graphKey struct {
	prefix string
}

type counterKey struct {
	counter string
}

type xKey struct {
	X float64
}

type data map[weekKey]map[programKey]map[graphKey]map[counterKey]map[xKey]int64

const (
	version   = "Version"
	goos      = "GOOS"
	goarch    = "GOARCH"
	goVersion = "GoVersion"
)

// nest groups the report data by week, program, prefix, counter, and x value
// summing together counter values for each program report in a report.
func nest(reports []*telemetry.Report) data {
	result := make(data)
	for _, r := range reports {
		for _, p := range r.Programs {
			writeCount(result, r.Week, p.Program, version, p.Version, r.X, 1)
			writeCount(result, r.Week, p.Program, goos, p.GOOS, r.X, 1)
			writeCount(result, r.Week, p.Program, goarch, p.GOARCH, r.X, 1)
			writeCount(result, r.Week, p.Program, goVersion, p.GoVersion, r.X, 1)
			for c, value := range p.Counters {
				name, _ := parts(c)
				writeCount(result, r.Week, p.Program, name, c, r.X, value)
			}
		}
	}
	return result
}

// writeCount writes the counter values to the result. When a report contains
// multiple program reports for the same program, the value of the counters
// in that report are summed together.
func writeCount(result data, week, program, prefix, counter string, x float64, value int64) {
	wk := weekKey{week}
	if _, ok := result[wk]; !ok {
		result[wk] = make(map[programKey]map[graphKey]map[counterKey]map[xKey]int64)
	}
	pk := programKey{program}
	if _, ok := result[wk][pk]; !ok {
		result[wk][pk] = make(map[graphKey]map[counterKey]map[xKey]int64)
	}
	gk := graphKey{prefix}
	if _, ok := result[wk][pk][gk]; !ok {
		result[wk][pk][gk] = make(map[counterKey]map[xKey]int64)
	}
	counter = fixCounter(prefix, counter)
	ck := counterKey{counter}
	if _, ok := result[wk][pk][gk][ck]; !ok {
		result[wk][pk][gk][ck] = make(map[xKey]int64)
	}
	xk := xKey{x}
	result[wk][pk][gk][ck][xk] += value
	// record the total for all counters with the prefix
	// as the bucket name.
	if prefix != counter {
		ck = counterKey{prefix}
		if _, ok := result[wk][pk][gk][ck]; !ok {
			result[wk][pk][gk][ck] = make(map[xKey]int64)
		}
		result[wk][pk][gk][ck][xk] += value
	}
}

// fixCounter groups counters for version and go version by the major minor
// version. It also adds a prefix to counters derived from report metadata.
func fixCounter(prefix, counter string) string {
	switch prefix {
	case version:
		if counter == "devel" {
			return prefix + ":" + version
		}
		return prefix + ":" + semver.MajorMinor(counter)
	case goos:
		return prefix + ":" + counter
	case goarch:
		return prefix + ":" + counter
	case goVersion:
		return prefix + ":" + goMajorMinor(counter)
	}
	return counter
}

// parts gets splits the prefix and bucket parts of a counter name
// or a bucket name. For an input with no bucket part prefix and bucket
// are the same.
func parts(name string) (prefix, bucket string) {
	prefix, bucket, found := strings.Cut(name, ":")
	if !found {
		bucket = prefix
	}
	return prefix, bucket
}

// goMajorMinor gets the go<Maj>,<Min> version for a given go version.
// For example, go1.20.1 -> go1.20.
func goMajorMinor(v string) string {
	v = v[2:]
	maj, x, ok := cutInt(v)
	if !ok {
		return ""
	}
	x = x[1:]
	min, _, ok := cutInt(x)
	if !ok {
		return ""
	}
	return fmt.Sprintf("go%s.%s", maj, min)
}

// cutInt scans the leading decimal number at the start of x to an integer
// and returns that value and the rest of the string.
func cutInt(x string) (n, rest string, ok bool) {
	i := 0
	for i < len(x) && '0' <= x[i] && x[i] <= '9' {
		i++
	}
	if i == 0 || x[0] == '0' && i != 1 {
		return "", "", false
	}
	return x[:i], x[i:], true
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
