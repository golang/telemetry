// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"golang.org/x/telemetry"
	"golang.org/x/telemetry/internal/counter"
	it "golang.org/x/telemetry/internal/telemetry"
)

// This test contains multiple tests because Open() can only be called once.
// There are a number of date-sensitive tests in the tests array, but before
// doing them subtest() checks the correctness of a single upload to the local server.
func TestDates(t *testing.T) {
	skipIfUnsupportedPlatform(t)
	setup(t, "2019-12-01") // back-date the telemetry acceptance
	defer restore()
	thisInstant = future(0)
	finished := counter.Open()
	c := counter.New("testing")
	c.Inc()
	x := counter.NewStack("aStack", 4)
	x.Inc()
	thisInstant = future(15) // so it creates the count file
	// Windows will not be able to remove the count file if it is still open
	// (in non-test situations it would have been rotated out and closed)
	finished() // for Windows

	// compute the UploadConfig and remmember information about
	// the counter file before the subtest uses it
	cs, uc := createcounterStuff(t)
	uploadConfig = uc

	subtest(t) // do and check a report

	// create a lot of tests, and run them
	const today = "2020-01-24"
	const yesterday = "2020-01-23"
	tests := []Test{ // each date must be different to subvert the parse cache
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
	// Used maps ensures that test cases are for distinct dates.
	used := make(map[string]string)
	for _, tx := range tests {
		if used[tx.name] != "" || used[tx.date] != "" {
			t.Errorf("test %s reusing name or date. name:%s, date:%s",
				tx.name, used[tx.name], used[tx.date])
		}
		used[tx.name] = tx.name
		used[tx.date] = tx.name
		doTest(t, &tx, cs)
	}
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

// count file name:
// "%s%s-%s-%s-%s-", prog, progVers, goVers, runtime.GOOS, runtime.GOARCH
// + now.Format("2006-01-02") + "." + fileVersion + ".count"
// count file name: prog [@progVers] - goVers - GOOS - GOARCH - yyyy - mm  - dd".v1.count"

// from the count file name return a suitable UploadConfig, and the part
// of the file name before the day
func createUploadConfig(cfilename string) (*telemetry.UploadConfig, string) {
	cfilename = filepath.Base(cfilename)
	flds := strings.Split(cfilename, "-")
	if len(flds) != 7 {
		log.Fatalf("got %d fields, expected 7 (%q)", len(flds), cfilename)
	}
	var prog, progVers, goVers, GOOS, GOARCH string
	if pr, ver, ok := strings.Cut(flds[0], "@"); ok {
		prog = pr
		progVers = ver
	} else {
		prog = flds[0]
	}
	goVers = flds[1]
	GOOS = flds[2]
	GOARCH = flds[3]
	ans := telemetry.UploadConfig{
		GOOS:      []string{GOOS},
		GOARCH:    []string{GOARCH},
		GoVersion: []string{goVers},
		Programs: []*telemetry.ProgramConfig{
			{
				Name:     prog,
				Versions: []string{progVers},
				Counters: []telemetry.CounterConfig{
					{
						Name: "counter/{foo,main}", //  file.go:334 has counter/main
					},
				},
				Stacks: []telemetry.CounterConfig{
					{
						Name:  "aStack",
						Depth: 4,
					},
				},
			},
		},
	}
	return &ans, strings.Join(flds[:4], "-") + "-"
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
func createcounterStuff(t *testing.T) (*countFileInfo, *telemetry.UploadConfig) {
	fis, err := os.ReadDir(it.LocalDir)
	if err != nil {
		t.Fatal(err)
	}
	var countFileName string
	var countFileBuf []byte
	for _, f := range fis {
		if strings.HasSuffix(f.Name(), ".count") {
			countFileName = filepath.Join(it.LocalDir, f.Name())
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

	uc, pr := createUploadConfig(countFileName)
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
	return &ans, uc
}

func doTest(t *testing.T, doing *Test, known *countFileInfo) {
	// setup
	thisInstant = setDay(doing.today)
	contents := bytes.Join([][]byte{
		known.buf[:known.beginOffset],
		[]byte(doing.begins),
		known.buf[known.beginOffset+len("YYYY-MM-DD") : known.endOffset],
		[]byte(doing.ends),
		known.buf[known.endOffset+len("YYYY-MM-DD"):],
	}, nil)
	filename := known.namePrefix + doing.date + ".v1.count"
	if err := os.WriteFile(filepath.Join(it.LocalDir, filename), contents, 0666); err != nil {
		t.Errorf("%v writing count file for %s (%s)", err, doing.name, filename)
		return
	}
	for _, x := range doing.locals {
		nm := fmt.Sprintf("local.%s.json", x)
		if err := os.WriteFile(filepath.Join(it.LocalDir, nm), []byte{}, 0666); err != nil {
			t.Errorf("%v writing local file %s", err, nm)
		}
	}
	for _, x := range doing.readys {
		nm := fmt.Sprintf("%s.json", x)
		if err := os.WriteFile(filepath.Join(it.LocalDir, nm), []byte{}, 0666); err != nil {
			t.Errorf("%v writing ready file %s", err, nm)
		}
	}
	for _, x := range doing.uploads {
		nm := fmt.Sprintf("%s.json", x)
		if err := os.WriteFile(filepath.Join(it.UploadDir, nm), []byte{}, 0666); err != nil {
			t.Errorf("%v writing upload %s", err, nm)
		}
	}

	// run
	Run(nil)

	// check results
	var cfiles, rfiles, lfiles, ufiles int
	fis, err := os.ReadDir(it.LocalDir)
	if err != nil {
		t.Errorf("%v reading localdir %s", err, it.LocalDir)
		return
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
		default:
			t.Errorf("for %s, unexpected local file %s", doing.name, f.Name())
		}
	}
	fis, err = os.ReadDir(it.UploadDir)
	if err != nil {
		t.Errorf("%v reading uploaddir %s", err, it.UploadDir)
		return
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
	for i := 0; i < ufiles-len(doing.uploads); i++ {
		// get server responses for the new uploaded reports
		// (uploaded reports are never removed. there's about one per week)
		<-serverChan
	}

	// clean up
	cleanDir(t, doing, it.LocalDir)
	cleanDir(t, doing, it.UploadDir)
}

func subtest(t *testing.T) {
	t.Helper()
	// check state before generating report
	work := findWork(it.LocalDir, it.UploadDir)
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
	if _, err := reports(&work); err != nil {
		t.Fatal(err)
	}
	// expect a single report and nothing else
	got := findWork(it.LocalDir, it.UploadDir)
	if len(got.countfiles) != 0 {
		t.Errorf("expected no countfiles, got %d", len(got.countfiles))
	}
	if len(got.readyfiles) != 1 {
		// the uploadable report
		t.Errorf("expected one readyfile, got %d", len(got.readyfiles))
	}
	fi, err := os.ReadDir(it.LocalDir)
	if len(fi) != 3 || err != nil {
		// one local report, one uploadable report, one weekends file
		t.Errorf("expected three files in LocalDir, got %d, %v", len(fi), err)
	}
	if len(got.uploaded) != 0 {
		t.Errorf("expected no uploadedfiles, got %d", len(got.uploaded))
	}

	// check contents. The semantic difference is "testing:1" in the
	// local file, but the json has some extra commas.
	var localFile, uploadFile []byte
	for _, f := range fi {
		fname := filepath.Join(it.LocalDir, f.Name())
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
	want := regexp.MustCompile("(?s:,. *\"testing\": 1)")
	found := want.FindSubmatchIndex(localFile)
	if len(found) != 2 {
		t.Fatalf("expected to find %q in %q", want, localFile)
	}

	// all the counters except for 'testing' should be in the upload file
	// (counter/main and the stack counter)
	if string(uploadFile) != string(localFile[:found[0]])+string(localFile[found[1]:]) {
		t.Fatalf("got\n%q expected\n%q", uploadFile,
			string(localFile[:found[0]])+string(localFile[found[1]:]))
	}
	// and try uploading to the test
	uploadReport(got.readyfiles[0])
	x := <-serverChan
	if x.length != len(uploadFile) {
		t.Errorf("%v %d", x, len(uploadFile))
	}
	// clean up everything in preparation for the rest of the tests
	cleanDir(t, nil, it.LocalDir)
	cleanDir(t, nil, it.UploadDir)
}
