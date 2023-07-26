// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The view command is a server intended to be run on a user's machine to
// display the local counters and time series graphs of counters.
package view

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/internal/config"
	"golang.org/x/telemetry/internal/configstore"
	tcounter "golang.org/x/telemetry/internal/counter"
)

var (
	addr     = flag.String("addr", "localhost:4040", "server listens on the given TCP network address")
	dev      = flag.Bool("dev", false, "rebuild static assets on save")
	fsConfig = flag.String("config", "", "load a config from the filesystem")

	//go:embed *
	content embed.FS
)

func Start() {
	flag.Parse()
	var fsys fs.FS = content
	if *dev {
		fsys = os.DirFS("cmd/gotelemetry/internal/view")
		watchStatic()
	}
	mux := http.NewServeMux()
	mux.Handle("/", handleIndex(fsys))
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("server listening at http://%s\n", listener.Addr())
	log.Fatal(http.Serve(listener, mux))
}

//go:generate go run golang.org/x/telemetry/godev/devtools/cmd/esbuild --outdir static

// watchStatic runs the same command as the generator above when the server is
// started in dev mode, rebuilding static assets on save.
func watchStatic() {
	cmd := exec.Command("go", "run", "golang.org/x/telemetry/godev/devtools/cmd/esbuild", "--outdir", "static", "--watch")
	cmd.Dir = "cmd/gotelemetry/internal/view"
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Fatal(err)
		}
	}()
}

type page struct {
	// Config is the config used to render the requested page.
	Config *config.Config

	// PrettyConfig is the Config struct formatted as indented JSON for display on the page.
	PrettyConfig string

	// ConfigVersion is used to render a dropdown list of config versions for a user to select.
	ConfigVersions []string

	// RequestedConfig is the URL query param value for config.
	RequestedConfig string

	// Files are the local counter files for display on the page.
	Files []*counterFile

	// Reports are the local reports for display on the page.
	Reports []*telemetryReport

	// Charts is the counter data from files and reports grouped by program and counter name.
	Charts []*program
}

// TODO: filtering and pagination for date ranges
func handleIndex(fsys fs.FS) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if r.URL.Path != "/" {
			http.FileServer(http.FS(fsys)).ServeHTTP(w, r)
			return nil
		}
		requestedConfig := r.URL.Query().Get("config")
		cfg, err := configAt(requestedConfig)
		if err != nil {
			return err
		}
		cfgVersionList, err := configVersions()
		if err != nil {
			return err
		}
		cfgJSON, err := json.MarshalIndent(cfg, "", "\t")
		if err != nil {
			return err
		}
		reports, err := reports(telemetry.LocalDir, cfg)
		if err != nil {
			return err
		}
		files, err := files(telemetry.LocalDir, cfg)
		if err != nil {
			return err
		}
		charts := charts(append(reports, pending(files, cfg)...), cfg).Programs
		data := page{
			Config:          cfg,
			PrettyConfig:    string(cfgJSON),
			ConfigVersions:  cfgVersionList,
			Reports:         reports,
			Files:           files,
			Charts:          charts,
			RequestedConfig: requestedConfig,
		}
		return renderTemplate(w, fsys, "index.html", data, http.StatusOK)
	}
}

// configAt gets the config at a given version.
func configAt(version string) (ucfg *config.Config, err error) {
	if version == "" || version == "empty" {
		return config.NewConfig(&telemetry.UploadConfig{Version: "empty"}), nil
	}
	if *fsConfig != "" {
		ucfg, err = config.ReadConfig(*fsConfig)
		if err != nil {
			return nil, err
		}
		if ucfg.Version == "" {
			ucfg.Version = *fsConfig
		}
	} else {
		cfg, err := configstore.Download(version, nil)
		if err != nil {
			return nil, err
		}
		ucfg = config.NewConfig(&cfg)
	}
	return ucfg, nil
}

// configVersions is the set of config versions the user may select from the UI.
// TODO: get the list of versions available from the proxy.
func configVersions() ([]string, error) {
	v := []string{"empty", "latest", "master"}
	return v, nil
}

// reports reads the local report files from a directory.
func reports(dir string, cfg *config.Config) ([]*telemetryReport, error) {
	fsys := os.DirFS(dir)
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var reports []*telemetryReport
	for _, e := range entries {
		if path.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			log.Printf("read report file failed: %v", err)
			continue
		}
		var report *telemetry.Report
		if err := json.Unmarshal(data, &report); err != nil {
			log.Printf("unmarshal report file failed: %v", err)
			continue
		}
		reports = append(reports, newTelemetryReport(report, cfg))
	}
	// sort the reports descending by week.
	sort.Slice(reports, func(i, j int) bool {
		return reports[j].Week < reports[i].Week
	})
	return reports, nil
}

// telemetryReport wraps telemetry report to add convenience fields for the UI.
type telemetryReport struct {
	*telemetry.Report
	ID       string
	Programs []*telemetryProgram
}

type telemetryProgram struct {
	*telemetry.ProgramReport
	ID      string
	Summary template.HTML
}

func newTelemetryReport(t *telemetry.Report, cfg *config.Config) *telemetryReport {
	var prgms []*telemetryProgram
	for _, p := range t.Programs {
		meta := map[string]string{
			"Program":   p.Program,
			"Version":   p.Version,
			"GOOS":      p.GOOS,
			"GOARCH":    p.GOARCH,
			"GoVersion": p.GoVersion,
		}
		counters := make(map[string]uint64)
		for k, v := range p.Counters {
			counters[k] = uint64(v)
		}
		prgms = append(prgms, &telemetryProgram{
			ProgramReport: p,
			ID:            strings.Join([]string{"reports", t.Week, p.Program, p.Version, p.GOOS, p.GOARCH, p.GoVersion}, ":"),
			Summary:       summary(cfg, meta, counters),
		})
	}
	return &telemetryReport{
		Report:   t,
		ID:       "reports:" + t.Week,
		Programs: prgms,
	}
}

// files reads the local counter files from a directory.
func files(dir string, cfg *config.Config) ([]*counterFile, error) {
	fsys := os.DirFS(dir)
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var files []*counterFile
	for _, e := range entries {
		if err != nil || e.IsDir() || path.Ext(e.Name()) != ".count" {
			continue
		}
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			log.Printf("read counter file failed: %v", err)
			continue
		}

		file, err := tcounter.Parse(e.Name(), data)
		if err != nil {
			log.Printf("parse counter file failed: %v", err)
			continue
		}
		files = append(files, newCounterFile(e.Name(), file, cfg))
	}
	return files, nil
}

// counterFile wraps counter file to add convenience fields for the UI.
type counterFile struct {
	*tcounter.File
	ID            string
	Summary       template.HTML
	ActiveMeta    map[string]bool
	ActiveCounter map[string]bool
}

func newCounterFile(name string, c *tcounter.File, cfg *config.Config) *counterFile {
	meta := map[string]bool{
		"Program":   cfg.HasProgram(c.Meta["Program"]),
		"Version":   cfg.HasVersion(c.Meta["Program"], c.Meta["Version"]),
		"GOOS":      cfg.HasGOOS(c.Meta["GOOS"]),
		"GOARCH":    cfg.HasGOARCH(c.Meta["GOARCH"]),
		"GoVersion": cfg.HasGoVersion(c.Meta["GoVersion"]),
	}
	count := make(map[string]bool)
	for k := range c.Count {
		count[k] = cfg.HasCounter(c.Meta["Program"], k)
	}
	return &counterFile{
		File:          c,
		ID:            name,
		ActiveMeta:    meta,
		ActiveCounter: count,
		Summary:       summary(cfg, c.Meta, c.Count),
	}
}

// summary generates a summary of a set of telemetry data. It describes what data is
// located in the set is not allowed given a config and how the data would be handled
// in the event of a telemetry upload event.
func summary(cfg *config.Config, meta map[string]string, counts map[string]uint64) template.HTML {
	if prog := meta["Program"]; !(cfg.HasProgram(prog)) {
		return template.HTML(fmt.Sprintf(
			"The program <code>%s</code> is unregistered. No data from this set would be uploaded to the Go team.",
			html.EscapeString(prog),
		))
	}
	var result strings.Builder
	var metaFields []string
	unknown := func(key string) {
		metaFields = append(metaFields, fmt.Sprintf("<code>%s=%s</code>", key, html.EscapeString(meta[key])))
	}
	if !(cfg.HasGOOS(meta["GOOS"])) || !(cfg.HasGOARCH(meta["GOARCH"])) {
		return template.HTML(fmt.Sprintf(
			"The GOOS/GOARCH combination <code>%s/%s</code> is unregistered. No data from this set would be uploaded to the Go team.",
			html.EscapeString(meta["GOOS"]),
			html.EscapeString(meta["GOARCH"]),
		))
	}
	if !(cfg.HasGoVersion(meta["GoVersion"])) {
		unknown("GoVersion")
	}
	if !(cfg.HasVersion(meta["Program"], meta["Version"])) {
		unknown("Version")
	}
	if len(metaFields) > 0 {
		result.WriteString("Unregistered attribute(s) ")
		result.WriteString(strings.Join(metaFields, ", "))
		result.WriteString(" would be reported as other. ")
	}
	var counters []string
	for c := range counts {
		if !(cfg.HasCounter(meta["Program"], c)) {
			counters = append(counters, fmt.Sprintf("<code>%s</code>", html.EscapeString(c)))
		}
	}
	if len(counters) > 0 {
		result.WriteString("Unregistered counter(s) ")
		result.WriteString(strings.Join(counters, ", "))
		result.WriteString(" would be excluded from a report. ")
	}
	return template.HTML(result.String())
}

type chartdata struct {
	Programs []*program
}

type program struct {
	ID       string
	Name     string
	Counters []*counter
	Active   bool
}

type counter struct {
	ID     string
	Name   string
	Data   []*datum
	Active bool
}

type datum struct {
	Week      string
	Program   string
	Version   string
	GOARCH    string
	GOOS      string
	GoVersion string
	Key       string
	Value     int64
}

// charts returns chartdata for a set of telemetry reports. It uses the config
// to determine if the programs and counters are active.
func charts(reports []*telemetryReport, cfg *config.Config) *chartdata {
	data := grouped(reports)
	result := &chartdata{}
	for pg, pgdata := range data {
		prog := &program{ID: "charts:" + pg.Name, Name: pg.Name, Active: cfg.HasProgram(pg.Name)}
		result.Programs = append(result.Programs, prog)
		for c, cdata := range pgdata {
			count := &counter{
				ID:     "charts:" + pg.Name + ":" + c.Name,
				Name:   c.Name,
				Data:   cdata,
				Active: cfg.HasCounter(pg.Name, c.Name) || cfg.HasCounterPrefix(pg.Name, c.Name),
			}
			prog.Counters = append(prog.Counters, count)
			sort.Slice(count.Data, func(i, j int) bool {
				a, err1 := strconv.ParseFloat(count.Data[i].Key, 32)
				b, err2 := strconv.ParseFloat(count.Data[j].Key, 32)
				if err1 == nil && err2 == nil {
					return a < b
				}
				return count.Data[i].Key < count.Data[j].Key
			})
		}
		sort.Slice(prog.Counters, func(i, j int) bool {
			return prog.Counters[i].Name < prog.Counters[j].Name
		})
	}
	sort.Slice(result.Programs, func(i, j int) bool {
		return result.Programs[i].Name < result.Programs[j].Name
	})
	return result
}

type programKey struct {
	Name string
}

type counterKey struct {
	Name string
}

// grouped returns normalized counter data grouped by program and counter.
func grouped(reports []*telemetryReport) map[programKey]map[counterKey][]*datum {
	result := make(map[programKey]map[counterKey][]*datum)
	for _, r := range reports {
		for _, e := range r.Programs {
			pgkey := programKey{e.Program}
			if _, ok := result[pgkey]; !ok {
				result[pgkey] = make(map[counterKey][]*datum)
			}
			for counter, value := range e.Counters {
				name, bucket, found := strings.Cut(counter, ":")
				key := name
				if found {
					key = bucket
				}
				element := &datum{
					Week:      r.Week,
					Program:   e.Program,
					Version:   e.Version,
					GOARCH:    e.GOARCH,
					GOOS:      e.GOOS,
					GoVersion: e.GoVersion,
					Key:       key,
					Value:     value,
				}
				ckey := counterKey{name}
				result[pgkey][ckey] = append(result[pgkey][ckey], element)
			}
		}
	}
	return result
}

// pending transforms the active counter files into a report. Used to add
// the data they contain to the charts in the UI.
func pending(files []*counterFile, cfg *config.Config) []*telemetryReport {
	reports := make(map[string]*telemetry.Report)
	for _, f := range files {
		week := f.Meta["TimeBegin"]
		if _, ok := reports[week]; !ok {
			reports[week] = &telemetry.Report{Week: week}
		}
		program := &telemetry.ProgramReport{
			Program:   f.Meta["Program"],
			GOOS:      f.Meta["GOOS"],
			GOARCH:    f.Meta["GOARCH"],
			GoVersion: f.Meta["GoVersion"],
			Version:   f.Meta["Version"],
		}
		program.Counters = make(map[string]int64)
		for k, v := range f.Count {
			program.Counters[k] = int64(v)
		}
		reports[week].Programs = append(reports[week].Programs, program)
	}
	var result []*telemetryReport
	for _, r := range reports {
		result = append(result, newTelemetryReport(r, cfg))
	}
	return result
}

type handlerFunc func(http.ResponseWriter, *http.Request) error

func (f handlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := f(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderTemplate executes a template response.
func renderTemplate(w http.ResponseWriter, fsys fs.FS, tmplPath string, data any, code int) error {
	patterns, err := tmplPatterns(fsys, tmplPath)
	if err != nil {
		return err
	}
	patterns = append(patterns, tmplPath)
	tmpl, err := template.ParseFS(fsys, patterns...)
	if err != nil {
		return err
	}
	name := path.Base(tmplPath)
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return err
	}
	if code != 0 {
		w.WriteHeader(code)
	}
	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	if _, err := w.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// tmplPatters generates a slice of file patterns to use in template.ParseFS.
func tmplPatterns(fsys fs.FS, tmplPath string) ([]string, error) {
	var patterns []string
	globs := []string{"*.tmpl", path.Join(path.Dir(tmplPath), "*.tmpl")}
	for _, g := range globs {
		matches, err := fs.Glob(fsys, g)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, matches...)
	}
	return patterns, nil
}