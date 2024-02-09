// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/regtest"
	it "golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

// Checks the correctness of a single upload to the local server.
func TestUploadBasic(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewProgram(t, "prog", func() int {
		counter.Inc("knownCounter")
		counter.Inc("unknownCounter")
		counter.NewStack("aStack", 4).Inc()
		return 0
	})
	// produce a counter file (timestamped with "today")
	telemetryDir := t.TempDir()
	if out, err := regtest.RunProg(t, telemetryDir, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	uc := createTestUploadConfig(t, []string{"knownCounter"}, []string{"aStack"})

	// Start upload server
	srv, uploaded := createTestUploadServer(t)
	defer srv.Close()

	uploader := newTestUploader(uc, telemetryDir, srv)
	// make it impossible to write a log by creating a non-directory with the log's name
	logName := filepath.Join(telemetryDir, "debug")
	fd, err := os.Create(logName)
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(logName) // perhaps overkill, but Windows is picky
	if err := LogIfDebug(""); err == nil {
		t.Errorf("log writing should have failed")
	}
	fd.Close()
	uploader.StartTime = uploader.StartTime.Add(15*24*time.Hour + 1*time.Minute) // make sure we are in the future.

	// Counter files in telemetryDir were timestamped with "today" timestamp
	// and scheduled to get expired in the future.
	// Let's pretend telemetry was enabled a year ago by mutating the mode file,
	// we are in the future, and test if the count files are successfully uploaded.
	uploader.ModeFilePath.SetModeAsOf("on", uploader.StartTime.Add(-365*24*time.Hour).UTC())
	uploadedContent := subtest(t, uc, uploader) // TODO(hyangah) : inline

	if want, got := [][]byte{uploadedContent}, uploaded(); !reflect.DeepEqual(want, got) {
		t.Errorf("server got %s\nwant %s", got, want)
	}
}

func newTestUploader(uc *telemetry.UploadConfig, telemetryDir string, srv *httptest.Server) *Uploader {
	uploader := NewUploader(uc)
	uploader.LocalDir = filepath.Join(telemetryDir, "local")
	uploader.UploadDir = filepath.Join(telemetryDir, "upload")
	uploader.ModeFilePath = it.ModeFilePath(filepath.Join(telemetryDir, "mode"))
	uploader.UploadServerURL = srv.URL
	return uploader
}

// createTestUploadServer creates a test server that records the uploaded data.
func createTestUploadServer(t *testing.T) (*httptest.Server, func() [][]byte) {
	s := &uploadQueue{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("invalid request received: %v", err)
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		s.Append(buf)
	})), s.Get
}

type uploadQueue struct {
	mu   sync.Mutex
	data [][]byte
}

func (s *uploadQueue) Append(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append(s.data, data)
}

func (s *uploadQueue) Get() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

func createTestUploadConfig(t *testing.T, counterNames, stackCounterNames []string) *telemetry.UploadConfig {
	goVersion, progVersion, progName := regtest.ProgInfo(t)
	GOOS, GOARCH := runtime.GOOS, runtime.GOARCH
	programConfig := &telemetry.ProgramConfig{
		Name:     progName,
		Versions: []string{progVersion},
	}
	for _, c := range counterNames {
		programConfig.Counters = append(programConfig.Counters, telemetry.CounterConfig{Name: c, Rate: 1})
	}
	for _, c := range stackCounterNames {
		programConfig.Stacks = append(programConfig.Stacks, telemetry.CounterConfig{Name: c, Rate: 1, Depth: 16})
	}
	return &telemetry.UploadConfig{
		GOOS:       []string{GOOS},
		GOARCH:     []string{GOARCH},
		SampleRate: 1.0,
		GoVersion:  []string{goVersion},
		Programs:   []*telemetry.ProgramConfig{programConfig},
	}
}

func TestDates(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	prog := regtest.NewProgram(t, "prog", func() int {
		counter.Inc("testing")
		counter.NewStack("aStack", 4).Inc()
		return 0
	})
	// Run a fake program that produces a counter file in the telemetryDir.
	// readCountFileInfo will give us a template counter file content
	// based on the counter file.
	telemetryDir := t.TempDir()
	if out, err := regtest.RunProg(t, telemetryDir, prog); err != nil {
		t.Fatalf("failed to run program: %s", out)
	}
	cs := readCountFileInfo(t, filepath.Join(telemetryDir, "local"))
	uc := createTestUploadConfig(t, nil, []string{"aStack"})

	const today = "2020-01-24"
	const yesterday = "2020-01-23"
	telemetryEnableTime, _ := time.Parse(dateFormat, "2019-12-01") // back-date the telemetry acceptance
	tests := []Test{                                               // each date must be different to subvert the parse cache
		{ // test that existing counters and ready files are not uploaded if they span data before telemetry was enabled
			name:   "beforefirstupload",
			today:  "2019-12-04",
			date:   "2019-12-03",
			begins: "2019-12-01",
			ends:   "2019-12-03",
			readys: []string{"2019-12-01", "2019-12-02"},
			// We get one local report: the newly created report.
			// It is not ready as it begins on the same day that telemetry was
			// enabled, and we err on the side of assuming it holds data from before
			// the user turned on uploading.
			wantLocal: 1,
			// The report for 2019-12-01 is still ready, because it was not uploaded.
			// This could happen in practice if the user disabled and then reenabled
			// telmetry.
			wantReady: 1,
			// The report for 2019-12-02 was uploaded.
			wantUploadeds: 1,
		},
		{ // test that existing counters and ready files are uploaded they only contain data after telemetry was enabled
			name:          "oktoupload",
			today:         "2019-12-10",
			date:          "2019-12-09",
			begins:        "2019-12-02",
			ends:          "2019-12-09",
			readys:        []string{"2019-12-07"},
			wantLocal:     1,
			wantUploadeds: 2, // Both new report and existing report are uploaded.
		},
		{ // test that an old countfile is removed and no reports generated
			name:   "oldcountfile",
			today:  today,
			date:   "2020-01-01",
			begins: "2020-01-01",
			ends:   olderThan(t, today, distantPast, "oldcountfile"),
			// one local; readys, uploads are empty, and there should be nothing left
			wantLocal: 1,
		},
		{ // test that a count file expiring today is left alone
			name:       "todayscountfile",
			today:      today,
			date:       "2020-01-02",
			begins:     "2020-01-08",
			ends:       today,
			wantCounts: 1,
		},
		{ // test that a count file expiring yesterday generates reports
			name:          "yesterdaycountfile",
			today:         today,
			date:          "2020-01-03",
			begins:        "2020-01-16",
			ends:          yesterday,
			wantLocal:     1,
			wantUploadeds: 1,
		},
		{ // count file already has local report, remove count file
			name:       "alreadydonelocal",
			today:      today,
			date:       "2020-01-04",
			begins:     "2020-01-16",
			ends:       yesterday,
			locals:     []string{yesterday},
			wantCounts: 0,
			wantLocal:  1,
		},
		{ // count file already has upload report, remove count file
			name:          "alreadydoneuploaded",
			today:         today,
			date:          "2020-01-05",
			begins:        "2020-01-16",
			ends:          "2020-01-23",
			uploads:       []string{"2020-01-23"},
			wantCounts:    0, // count file has been used, remove it
			wantLocal:     0, // no local report generated
			wantUploadeds: 1, // the existing uploaded report
		},
		{ // for some reason there's a ready file in the future, don't upload it
			name:       "futurereadyfile",
			today:      "2020-01-24",
			date:       "2020-01-06",
			begins:     "2020-01-16",
			ends:       "2020-01-24", // count file not expired
			readys:     []string{"2020-01-25"},
			wantCounts: 1, // active count file
			wantReady:  1, // existing premature ready file
		},
	}

	for _, tx := range tests {
		t.Run(tx.name, func(t *testing.T) {
			telemetryDir := t.TempDir()

			srv, uploaded := createTestUploadServer(t)
			defer srv.Close()

			uploader := newTestUploader(uc, telemetryDir, srv)
			uploader.ModeFilePath.SetModeAsOf("on", telemetryEnableTime)
			uploader.UploadServerURL = srv.URL
			uploader.StartTime = mustParseDate(tx.today)

			wantUploadCount := doTest(t, uploader, &tx, cs)
			if got := len(uploaded()); wantUploadCount != got {
				t.Errorf("server got %d upload requests, want %d", got, wantUploadCount)
			}
		})
	}
}

func mustParseDate(d string) time.Time {
	x, err := time.Parse("2006-01-02", d)
	if err != nil {
		log.Fatalf("couldn't parse time %s", d)
	}
	return x
}

// return a day more than 'old' before 'today'
func olderThan(t *testing.T, today string, old time.Duration, nm string) string {
	x, err := time.Parse("2006-01-02", today)
	if err != nil {
		t.Errorf("%q not a day in test %s (%v)", today, nm, err)
		return today // so test should fail
	}
	ans := x.Add(-old - 24*time.Hour)
	msg := ans.Format("2006-01-02")
	return msg
}

// Test is a single test.
//
// All dates are in YYYY-MM-DD format.
type Test struct {
	name  string // the test name; only used for descriptive output
	today string // the date of the fake upload
	// count file
	date         string // the date in of the upload file name; must be unique among tests
	begins, ends string // the begin and end date stored in the counter metadata

	// Dates of load reports in the local dir.
	locals []string

	// Dates of upload reports in the local dir.
	readys []string

	// Dates of reports already uploaded.
	uploads []string

	// number of expected results
	wantCounts    int
	wantReady     int
	wantLocal     int
	wantUploadeds int
}

// Information from the counter file so its contents can be
// modified for tests
type countFileInfo struct {
	beginOffset, endOffset int    // where the dates are in the file
	buf                    []byte // counter file contents
	namePrefix             string // the part of its name before the date
	originalName           string // its original name
}

// return useful information from the counter file to be used
// in creating tests. also compute and return the UploadConfig
// Note that there must be exactly one counter file in localDir.
func readCountFileInfo(t *testing.T, localDir string) *countFileInfo {
	fis, err := os.ReadDir(localDir)
	if err != nil {
		t.Fatal(err)
	}
	var countFileName string
	var countFileBuf []byte
	for _, f := range fis {
		if strings.HasSuffix(f.Name(), ".count") {
			countFileName = filepath.Join(localDir, f.Name())
			buf, err := os.ReadFile(countFileName)
			if err != nil {
				t.Fatal(err)
			}
			countFileBuf = buf
			break
		}
	}
	if len(countFileBuf) == 0 {
		t.Fatalf("no contents read for %s", countFileName)
	}

	var cfilename string = countFileName
	cfilename = filepath.Base(cfilename)
	flds := strings.Split(cfilename, "-")
	if len(flds) != 7 {
		t.Fatalf("got %d fields, expected 7 (%q)", len(flds), cfilename)
	}
	pr := strings.Join(flds[:4], "-") + "-"

	ans := countFileInfo{
		buf:          countFileBuf,
		namePrefix:   pr,
		originalName: countFileName,
	}
	idx := bytes.Index(countFileBuf, []byte("TimeEnd: "))
	if idx < 0 {
		t.Fatalf("couldn't find TimeEnd in count file %q", countFileBuf[:100])
	}
	ans.endOffset = idx + len("TimeEnd: ")
	idx = bytes.Index(countFileBuf, []byte("TimeBegin: "))
	if idx < 0 {
		t.Fatalf("couldn't find TimeBegin in countfile %q", countFileBuf[:100])
	}
	ans.beginOffset = idx + len("TimeBegin: ")
	return &ans
}

func doTest(t *testing.T, u *Uploader, doing *Test, known *countFileInfo) int {
	dbg := filepath.Join(u.LocalDir, "debug")
	os.MkdirAll(dbg, 0777)
	if err := LogIfDebug(dbg); err != nil {
		t.Errorf("debug logging: %v", err)
	}
	if len(doing.uploads) > 0 {
		os.MkdirAll(u.UploadDir, 0777)
	}
	contents := bytes.Join([][]byte{
		known.buf[:known.beginOffset],
		[]byte(doing.begins),
		known.buf[known.beginOffset+len("YYYY-MM-DD") : known.endOffset],
		[]byte(doing.ends),
		known.buf[known.endOffset+len("YYYY-MM-DD"):],
	}, nil)
	filename := known.namePrefix + doing.date + ".v1.count"
	if err := os.WriteFile(filepath.Join(u.LocalDir, filename), contents, 0666); err != nil {
		t.Errorf("%v writing count file for %s (%s)", err, doing.name, filename)
		return 0
	}
	for _, x := range doing.locals {
		nm := fmt.Sprintf("local.%s.json", x)
		if err := os.WriteFile(filepath.Join(u.LocalDir, nm), []byte{}, 0666); err != nil {
			t.Errorf("%v writing local file %s", err, nm)
		}
	}
	for _, x := range doing.readys {
		nm := fmt.Sprintf("%s.json", x)
		if err := os.WriteFile(filepath.Join(u.LocalDir, nm), []byte{}, 0666); err != nil {
			t.Errorf("%v writing ready file %s", err, nm)
		}
	}
	for _, x := range doing.uploads {
		nm := fmt.Sprintf("%s.json", x)
		if err := os.WriteFile(filepath.Join(u.UploadDir, nm), []byte{}, 0666); err != nil {
			t.Errorf("%v writing upload %s", err, nm)
		}
	}

	// run
	u.Run()

	// check results
	var cfiles, rfiles, lfiles, ufiles, logcnt int
	fis, err := os.ReadDir(u.LocalDir)
	if err != nil {
		t.Errorf("%v reading localdir %s", err, u.LocalDir)
		return 0
	}
	for _, f := range fis {
		switch {
		case strings.HasSuffix(f.Name(), ".v1.count"):
			cfiles++
		case f.Name() == "weekends": // ok
		case strings.HasPrefix(f.Name(), "local."):
			lfiles++
		case strings.HasSuffix(f.Name(), ".json"):
			rfiles++
		case f.Name() == "debug":
			dbgname := filepath.Join(u.LocalDir, "debug")
			logs, err := os.ReadDir(dbgname)
			if err != nil {
				break
			}
			logcnt += len(logs)
		default:
			t.Errorf("for %s, unexpected local file %s", doing.name, f.Name())
		}
	}
	if logcnt != 1 {
		t.Errorf("expected 1 log file, got %d", logcnt)
	}
	fis, err = os.ReadDir(u.UploadDir)
	if err != nil {
		t.Errorf("%v reading uploaddir %s", err, u.UploadDir)
		return 0
	}
	ufiles = len(fis) // assume there's nothing but .json reports
	if doing.wantCounts != cfiles {
		t.Errorf("%s: got %d countfiles, wanted %d", doing.name, cfiles, doing.wantCounts)
	}
	if doing.wantReady != rfiles {
		t.Errorf("%s: got %d ready files, wanted %d", doing.name, rfiles, doing.wantReady)
	}
	if doing.wantLocal != lfiles {
		t.Errorf("%s: got %d localfiles, wanted %d", doing.name, lfiles, doing.wantLocal)
	}
	if doing.wantUploadeds != ufiles {
		t.Errorf("%s: got %d uploaded files, wanted %d", doing.name, ufiles, doing.wantUploadeds)
	}
	// close all loggers, so Windows can clean up the test
	for _, lw := range seenlogwriters {
		if fd, ok := lw.(*os.File); ok {
			fd.Close()
		}
	}
	// and let the next test start in a clean state
	seenlogwriters = []io.Writer{}
	return ufiles - len(doing.uploads)
}

func subtest(t *testing.T, c *telemetry.UploadConfig, u *Uploader) (uploaded []byte) {
	// check state before generating report
	work := u.findWork()
	// expect one count file and nothing else
	if len(work.countfiles) != 1 {
		t.Errorf("expected one countfile, got %d", len(work.countfiles))
	}
	if len(work.readyfiles) != 0 {
		t.Errorf("expected no readyfiles, got %d", len(work.readyfiles))
	}
	if len(work.uploaded) != 0 {
		t.Errorf("expected no uploadedfiles, got %d", len(work.uploaded))
	}
	// generate reports
	if _, err := u.reports(&work); err != nil {
		t.Fatal(err)
	}
	// expect a single report and nothing else
	got := u.findWork()
	if len(got.countfiles) != 0 {
		t.Errorf("expected no countfiles, got %d", len(got.countfiles))
	}
	if len(got.readyfiles) != 1 {
		// the uploadable report
		t.Errorf("expected one readyfile, got %d", len(got.readyfiles))
	}
	fi, err := os.ReadDir(u.LocalDir)
	if len(fi) != 3 || err != nil {
		// one local report, one uploadable report, one weekends file
		t.Errorf("expected three files in LocalDir, got %d, %v", len(fi), err)
	}
	if len(got.uploaded) != 0 {
		t.Errorf("expected no uploadedfiles, got %d", len(got.uploaded))
	}
	// check contents.
	var localFile, uploadFile []byte
	for _, f := range fi {
		fname := filepath.Join(u.LocalDir, f.Name())
		buf, err := os.ReadFile(fname)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(f.Name(), "local") {
			localFile = buf
		} else if strings.HasSuffix(f.Name(), ".json") {
			uploadFile = buf
		}
	}

	want := regexp.MustCompile("(?s:,. *\"unknownCounter\": 1)")
	found := want.FindSubmatchIndex(localFile)
	if len(found) != 2 {
		t.Fatalf("expected to find %q in %q", want, localFile)
	}

	// all the counters except for 'unknownCounter' should be in the upload file.
	if got, want := string(uploadFile), string(localFile[:found[0]])+string(localFile[found[1]:]); got != want {
		t.Fatalf("got\n%s want\n%s", got, want)
	}
	// and try uploading to the test
	u.uploadReport(got.readyfiles[0])
	return uploadFile
}
