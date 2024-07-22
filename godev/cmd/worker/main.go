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
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"golang.org/x/exp/slog"
	"golang.org/x/mod/semver"
	"golang.org/x/telemetry/godev/internal/config"
	"golang.org/x/telemetry/godev/internal/content"
	ilog "golang.org/x/telemetry/godev/internal/log"
	"golang.org/x/telemetry/godev/internal/middleware"
	"golang.org/x/telemetry/godev/internal/storage"
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
		slog.SetDefault(slog.New(ilog.NewGCPLogHandler()))
	}

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
	mux.Handle("/merge/", handleMerge(buckets))
	mux.Handle("/chart/", handleChart(ucfg, buckets))
	mux.Handle("/queue-tasks/", handleTasks(cfg))

	mw := middleware.Chain(
		middleware.Log(slog.Default()),
		middleware.Timeout(cfg.RequestTimeout),
		middleware.RequestSize(cfg.MaxRequestBytes),
		middleware.Recover(),
	)

	fmt.Printf("server listening at http://localhost:%s\n", cfg.WorkerPort)
	log.Fatal(http.ListenAndServe(":"+cfg.WorkerPort, mw(mux)))
}

// handleTasks will populate the task queue that processes report
// data. Cloud Scheduler will be instrumented to call this endpoint
// daily to merge reports and generate chart data.
// The merge tasks will merge the previous 7 days reports.
// The chart tasks generate daily and weekly charts for the 7 days preceding
// today.
// - Daily chart: utilizes data exclusively from the specific date.
// - Weekly chart: encompasses 7 days of data, concluding on the specified date.
// TODO(golang/go#62575): adjust the date range to align with report
// upload cutoff.
func handleTasks(cfg *config.Config) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		now := time.Now().UTC()
		for i := 7; i > 0; i-- {
			date := now.AddDate(0, 0, -1*i).Format(time.DateOnly)
			url := cfg.WorkerURL + "/merge/?date=" + date
			if _, err := createHTTPTask(cfg, url); err != nil {
				return err
			}
		}
		// TODO(hxjiang): have an endpoint to produce all the json instead of a hard
		// coded one day delay.
		for i := 8; i > 1; i-- {
			// Daily chart: generate chart using one day's data.
			date := now.AddDate(0, 0, -1*i).Format(time.DateOnly)
			url := cfg.WorkerURL + "/chart/?date=" + date
			if _, err := createHTTPTask(cfg, url); err != nil {
				return err
			}

			// Weekly chart: generate chart using past 7 days' data.
			end := now.AddDate(0, 0, -1*i)
			start := end.AddDate(0, 0, -6)
			url = cfg.WorkerURL + "/chart/?start=" + start.Format(time.DateOnly) + "&end=" + end.Format(time.DateOnly)
			if _, err := createHTTPTask(cfg, url); err != nil {
				return err
			}
		}
		return nil
	}
}

// createHTTPTask constructs a task with a authorization token
// and HTTP target then adds it to a Queue.
func createHTTPTask(cfg *config.Config, url string) (*taskspb.Task, error) {
	ctx := context.Background()
	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cloudtasks.NewClient: %w", err)
	}
	defer client.Close()

	queuePath := fmt.Sprintf("projects/%s/locations/%s/queues/%s", cfg.ProjectID, cfg.LocationID, cfg.QueueID)
	req := &taskspb.CreateTaskRequest{
		Parent: queuePath,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        url,
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: cfg.IAPServiceAccount,
							Audience:            cfg.ClientID,
						},
					},
				},
			},
		},
	}

	createdTask, err := client.CreateTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cloudtasks.CreateTask: %w", err)
	}
	return createdTask, nil
}

// TODO: monitor duration and processed data volume.
func handleMerge(s *storage.API) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()
		date := r.URL.Query().Get("date")
		if _, err := time.Parse(time.DateOnly, date); err != nil {
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

func fileName(start, end time.Time) string {
	if start.Equal(end) {
		return end.Format(time.DateOnly) + ".json"
	}

	return start.Format(time.DateOnly) + "_" + end.Format(time.DateOnly) + ".json"
}

// parseDateRange returns the start and end date from the given url.
func parseDateRange(url *url.URL) (start, end time.Time, _ error) {
	if dateString := url.Query().Get("date"); dateString != "" {
		if url.Query().Get("start") != "" || url.Query().Get("end") != "" {
			return time.Time{}, time.Time{}, content.Error(fmt.Errorf("start or end key should be empty when date key is being used"), http.StatusBadRequest)
		}
		date, err := time.Parse(time.DateOnly, dateString)
		if err != nil {
			return time.Time{}, time.Time{}, content.Error(err, http.StatusBadRequest)
		}
		return date, date, nil
	}

	var err error
	startString := url.Query().Get("start")
	start, err = time.Parse(time.DateOnly, startString)
	if err != nil {
		return time.Time{}, time.Time{}, content.Error(err, http.StatusBadRequest)
	}
	endString := url.Query().Get("end")
	end, err = time.Parse(time.DateOnly, endString)
	if err != nil {
		return time.Time{}, time.Time{}, content.Error(err, http.StatusBadRequest)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, content.Error(fmt.Errorf("end date is earlier than start"), http.StatusBadRequest)
	}
	return start, end, nil
}

func readMergedReports(ctx context.Context, fileName string, s *storage.API) ([]telemetry.Report, error) {
	in, err := s.Merge.Object(fileName).NewReader(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, content.Error(fmt.Errorf("merge file %s not found", fileName), http.StatusNotFound)
	}
	if err != nil {
		return nil, err
	}
	defer in.Close()

	var reports []telemetry.Report
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		var report telemetry.Report
		if err := json.Unmarshal(scanner.Bytes(), &report); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}

	return reports, nil
}

func handleChart(cfg *tconfig.Config, s *storage.API) content.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := r.Context()

		start, end, err := parseDateRange(r.URL)
		if err != nil {
			return err
		}

		var reports []telemetry.Report
		var xs []float64
		for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
			dailyReports, err := readMergedReports(ctx, date.Format(time.DateOnly)+".json", s)
			if err != nil {
				return err
			}
			for _, r := range dailyReports {
				reports = append(reports, r)
				xs = append(xs, r.X)
			}
		}

		data := nest(reports)
		charts := charts(cfg, start.Format(time.DateOnly), end.Format(time.DateOnly), data, xs)

		obj := fileName(start, end)
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

		msg := fmt.Sprintf("processed %d reports from date %s to %s into %s", len(reports), start.Format(time.DateOnly), end.Format(time.DateOnly), s.Chart.URI()+"/"+obj)
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

func charts(cfg *tconfig.Config, start, end string, d data, xs []float64) *chartdata {
	result := &chartdata{DateRange: [2]string{start, end}, NumReports: len(xs)}
	for _, p := range cfg.Programs {
		prog := &program{ID: "charts:" + p.Name, Name: p.Name}
		result.Programs = append(result.Programs, prog)
		var charts []*chart
		if !telemetry.IsToolchainProgram(p.Name) {
			charts = append(charts, d.partition(p.Name, "Version", p.Versions))
		}
		charts = append(charts,
			d.partition(p.Name, "GOOS", cfg.GOOS),
			d.partition(p.Name, "GOARCH", cfg.GOARCH),
			d.partition(p.Name, "GoVersion", cfg.GoVersion))
		for _, c := range p.Counters {
			// TODO: add support for histogram counters by getting the counter type
			// from the chart config.
			charts = append(charts, d.partition(p.Name, c.Name, tconfig.Expand(c.Name)))
		}
		for _, p := range charts {
			if p != nil {
				prog.Charts = append(prog.Charts, p)
			}
		}
	}
	return result
}

// partition builds a chart for the program and the counter. It can return nil
// if there is no data for the counter in dat.
func (d data) partition(program, counterPrefix string, counters []string) *chart {
	count := &chart{
		ID:   "charts:" + program + ":" + counterPrefix,
		Name: counterPrefix,
		Type: "partition",
	}
	pk := programName(program)
	prefix, _ := splitCounterName(counterPrefix)
	gk := graphName(prefix)

	var (
		counts = make(map[string]float64) // bucket name -> total count
		end    weekName                   // latest week observed
	)
	for wk := range d {
		if wk >= end {
			end = wk
		}
		// TODO: when should this be number of reports?
		// total := len(xs)
		if total := len(d[wk][pk][gk][counterName(gk)]); total == 0 {
			continue
		}
		// We group versions into major minor buckets, we must skip
		// major minor versions we've already added to the dataset.
		seen := make(map[string]bool)
		for _, b := range counters {
			// TODO(hyangah): let caller normalize names in counters.
			counter := normalizeCounterName(counterPrefix, b)
			if seen[counter] {
				continue
			}
			seen[counter] = true
			ck := counterName(counter)
			// number of reports where count prefix:bucket > 0
			n := len(d[wk][pk][gk][ck])
			_, bucket := splitCounterName(counter)

			counts[bucket] += float64(n)
		}
	}

	if len(counts) == 0 {
		return nil
	}

	// datum.Week always points to the end date
	for k, v := range counts {
		d := &datum{
			Week:  string(end),
			Key:   k,
			Value: v,
		}
		count.Data = append(count.Data, d)
	}
	// Sort the data based on bucket name to ensure deterministic output.
	sort.Slice(count.Data, func(i, j int) bool {
		return count.Data[i].Key < count.Data[j].Key
	})

	return count
}

// weekName is the date of the report week in the format "YYYY-MM-DD".
type weekName string

// programName is the package path of the program, as used in
// telemetry.ProgramReport and chartconfig.Program.
// e.g. golang.org/x/tools/gopls, cmd/go.
type programName string

// graphName is the graph name.
// A graph plots distribution or timeseries of related counters.
type graphName string

// counterName is the counter name.
type counterName string

// reportID is the upload report ID.
// The current implementation uses telemetry.Report.X,
// a random number, computed by the uploader when creating a Report object.
// See x/telemetry/internal/upload.(*uploader).createReport.
type reportID float64

type data map[weekName]map[programName]map[graphName]map[counterName]map[reportID]int64

// Names of special counters.
// Unlike other counters, these are constructed from the metadata in the report.
const (
	versionCounter   = "Version"
	goosCounter      = "GOOS"
	goarchCounter    = "GOARCH"
	goversionCounter = "GoVersion"
)

// nest groups the report data by week, program, prefix, counter, and x value
// summing together counter values for each program report in a report.
func nest(reports []telemetry.Report) data {
	result := make(data)
	for _, r := range reports {
		for _, p := range r.Programs {
			result.writeCount(r.Week, p.Program, versionCounter, p.Version, r.X, 1)
			result.writeCount(r.Week, p.Program, goosCounter, p.GOOS, r.X, 1)
			result.writeCount(r.Week, p.Program, goarchCounter, p.GOARCH, r.X, 1)
			result.writeCount(r.Week, p.Program, goversionCounter, p.GoVersion, r.X, 1)
			for c, value := range p.Counters {
				prefix, _ := splitCounterName(c)
				result.writeCount(r.Week, p.Program, prefix, c, r.X, value)
			}
		}
	}
	return result
}

// readCount reads the count value based on the input keys.
// Return error if any key does not exist.
func (d data) readCount(week, program, prefix, counter string, x float64) (int64, error) {
	wk := weekName(week)
	if _, ok := d[wk]; !ok {
		return -1, fmt.Errorf("missing weekKey %q", week)
	}
	pk := programName(program)
	if _, ok := d[wk][pk]; !ok {
		return -1, fmt.Errorf("missing programKey %q", program)
	}
	gk := graphName(prefix)
	if _, ok := d[wk][pk][gk]; !ok {
		return -1, fmt.Errorf("missing graphKey key %q", prefix)
	}
	ck := counterName(counter)
	if _, ok := d[wk][pk][gk][ck]; !ok {
		return -1, fmt.Errorf("missing counterKey %v", counter)
	}
	return d[wk][pk][gk][ck][reportID(x)], nil
}

// writeCount writes the counter values to the result. When a report contains
// multiple program reports for the same program, the value of the counters
// in that report are summed together.
func (d data) writeCount(week, program, prefix, counter string, x float64, value int64) {
	wk := weekName(week)
	if _, ok := d[wk]; !ok {
		d[wk] = make(map[programName]map[graphName]map[counterName]map[reportID]int64)
	}
	pk := programName(program)
	if _, ok := d[wk][pk]; !ok {
		d[wk][pk] = make(map[graphName]map[counterName]map[reportID]int64)
	}
	// We want to group and plot bucket/histogram counters with the same prefix.
	// Use the prefix as the graph name.
	gk := graphName(prefix)
	if _, ok := d[wk][pk][gk]; !ok {
		d[wk][pk][gk] = make(map[counterName]map[reportID]int64)
	}
	// TODO(hyangah): let caller pass the normalized counter name.
	counter = normalizeCounterName(prefix, counter)
	ck := counterName(counter)
	if _, ok := d[wk][pk][gk][ck]; !ok {
		d[wk][pk][gk][ck] = make(map[reportID]int64)
	}

	// x is a random number sent with each upload report.
	// Since there is no identifier for the uploader, we use x as the uploader ID
	// to approximate the number of unique uploader.
	id := reportID(x)
	d[wk][pk][gk][ck][id] += value
	// TODO: each uploader should send the report only once.
	// Shouldn't we overwrite, instead of summing?

	// If the counter is an instance of a bucket counter or histogram counter
	// record the value with a special counter (prefix). For example, if
	// there are gopls/client:vscode-go, gopls/vlient:vim-go, ...,
	// we compute the total number of gopls/client:* by summing up all values
	// with a special counter name "gopls/client".
	// TODO(hyangah): why do we want to compute the fraction, instead of showing
	// the absolute number of reports?
	if prefix != counter {
		ck = counterName(prefix)
		if _, ok := d[wk][pk][gk][ck]; !ok {
			d[wk][pk][gk][ck] = make(map[reportID]int64)
		}
		d[wk][pk][gk][ck][id] += value
	}
}

// normalizeCounterName normalizes the counter name.
// More specifically, program version, goos, goarch, and goVersion
// are not a real counter, but information from the metadata in the report.
// This function constructs pseudo counter names to handle them
// like other normal counters in aggregation and chart drawing.
// To limit the cardinality of version and goVersion, this function
// uses only major and minor version numbers in the pseudo-counter names.
// If the counter is a normal counter name, it is returned as is.
func normalizeCounterName(prefix, counter string) string {
	switch prefix {
	case versionCounter:
		if counter == "devel" {
			return prefix + ":" + counter
		}
		if strings.HasPrefix(counter, "go") {
			return prefix + ":" + goMajorMinor(counter)
		}
		return prefix + ":" + semver.MajorMinor(counter)
	case goosCounter:
		return prefix + ":" + counter
	case goarchCounter:
		return prefix + ":" + counter
	case goversionCounter:
		return prefix + ":" + goMajorMinor(counter)
	}
	return counter
}

// splitCounterName gets splits the prefix and bucket splitCounterName of a counter name
// or a bucket name. For an input with no bucket part prefix and bucket
// are the same.
func splitCounterName(name string) (prefix, bucket string) {
	prefix, bucket, found := strings.Cut(name, ":")
	if !found {
		bucket = prefix
	}
	return prefix, bucket
}

// goMajorMinor gets the go<Maj>,<Min> version for a given go version.
// For example, go1.20.1 -> go1.20.
// TODO(hyangah): replace with go/version.Lang (available from go1.22)
// after our builders stop running go1.21.
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
	var f fs.FS = contentfs.FS
	if fromOS {
		f = os.DirFS("internal/content")
	}
	f, err := unionfs.Sub(f, "worker", "shared")
	if err != nil {
		log.Fatal(err)
	}
	return f
}
