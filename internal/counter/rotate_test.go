// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"golang.org/x/telemetry/internal/mmap"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/internal/testenv"
)

// this test traces the life of a counter from creation
// through two file.rotate()s, followed by a failure to rotate
func TestRotateCounters(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)
	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	setup(t)
	defer restore()
	var f file
	defer close(&f)
	c := f.New("gophers")
	if c.ptr.count != nil {
		t.Error("new counter has non-nil ptr.count")
	}
	c.Inc() // make sure neither hits Counter.add()
	c.Inc() // second use takes a different code path
	// at this point c.file is not mapped so c's value is stored in extra.
	if c.ptr.count != nil {
		t.Error("counter without mapped file has non-nil ptr.count")
	}
	if c.file.current.Load() != nil {
		t.Error("counter has mapped file unexpectedly")
	}
	state := c.state.load()
	if state.extra() != 2 {
		// the value of c is in its extra field
		t.Errorf("got %d, expected 2", state.extra())
	}
	f.rotate()
	c.Inc() // this goes through counter.add() safely
	if c.file.current.Load() == nil {
		t.Error("rotated file has no mapping")
	}
	// rotate called c.releaseLock(), moving c's value from extra to the file
	state = c.state.load()
	if state.extra() != 0 {
		t.Errorf("got %d, expected 0", state.extra())
	}
	if c.ptr.count == nil {
		t.Errorf("c has unexpected nil ptr")
	} else if c.ptr.count.Load() != 3 {
		// the value of c is in the mapped file
		t.Errorf("got %d, expected 3", c.ptr.count.Load())
	}

	// move into the future and rotate the file, remapping it
	now := getnow()
	counterTime = func() time.Time { return now.Add(7 * 24 * time.Hour) }
	f.rotate()

	// c has value 0 in the new file
	// but c won't have a pointer until the next Inc()
	state = c.state.load()
	if c.ptr.count == nil {
		t.Errorf("c unexpedtedly has nil ptr")
	} else if state.havePtr() {
		t.Error("unexpected pointer")
	}
	if state.extra() != 0 {
		t.Errorf("got %d, expected 0", state.extra())
	}
	c.Inc()
	state = c.state.load()
	if state.extra() != 0 {
		// as expected
		t.Errorf("got %d, expected 0", state.extra())
	}
	if !state.havePtr() {
		t.Errorf("expectd havePtr")
	}
	if c.ptr.count == nil || c.ptr.count.Load() != 1 {
		t.Errorf("c has wrong value")
	}
	// add a counter
	y := f.New("counter")

	// simulate failure to remap
	oldmap := memmap
	counterTime = func() time.Time { return now.Add(14 * 24 * time.Hour) }
	memmap = func(*os.File, *mmap.Data) (mmap.Data, error) { return mmap.Data{}, fmt.Errorf("too bad") }
	f.rotate()
	memmap = oldmap

	// no mapping
	if f.current.Load() != nil {
		t.Errorf("unexpected mapping")
	}
	c.Inc()
	// c should not have a pointer, but its internal
	// count should have been incremented
	if c.ptr.count != nil {
		t.Error("expected nil ptr")
	}
	if c.state.load().extra() != 1 {
		t.Errorf("got %d, but expected extra to be 1", c.state.load().extra())
	}
	// make sure a new counter doesn't fault
	x := f.New("newcounter")
	x.Inc()
	if x.state.load().extra() != 1 {
		t.Errorf("got %d, but expected extra to be 1", c.state.load().extra())
	}
	// make sure an existing unused counter doesn't fault
	// (it's incremented, but not visible externally)
	y.Inc()
	if y.state.load().extra() != 1 {
		t.Errorf("got %d, but expected extra to be 1", c.state.load().extra())
	}
}

// return the current date according to counterTime()
func getnow() time.Time {
	year, month, day := counterTime().Date()
	now := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	return now
}
func TestRotate(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	t.Logf("GOOS %s GOARCH %s", runtime.GOOS, runtime.GOARCH)
	now := getnow()
	setup(t)
	defer restore()
	// pretend something was uploaded
	os.WriteFile(filepath.Join(telemetry.UploadDir, "anything"), []byte{}, 0666)
	var f file
	defer close(&f)
	c := f.New("gophers")
	c.Inc()
	var modified int
	for i := 0; i < 2; i++ {
		// nothing should change on the second rotate
		f.rotate()
		fi, err := os.ReadDir(telemetry.LocalDir)
		if err != nil || len(fi) != 2 {
			t.Fatalf("err=%v, len(fi) = %d, want 2", err, len(fi))
		}
		x := fi[0].Name()
		y := x[len(x)-len("2006-01-02")-len(".v1.count") : len(x)-len(".v1.count")]
		us, err := time.ParseInLocation("2006-01-02", y, time.UTC)
		if err != nil {
			t.Fatal(err)
		}
		// we expect today's date?
		if us != now {
			t.Errorf("us = %v, want %v, i=%d y=%s", us, now, i, y)
		}
		fd, err := os.Open(filepath.Join(telemetry.LocalDir, fi[0].Name()))
		if err != nil {
			t.Fatal(err)
		}
		stat, err := fd.Stat()
		if err != nil {
			t.Fatal(err)
		}
		mt := stat.ModTime().Nanosecond()
		if modified == 0 {
			modified = mt
		}
		if modified != mt {
			t.Errorf("modified = %v, want %v", mt, modified)
		}
		fd.Close()
	}
	counterTime = func() time.Time { return now.Add(7 * 24 * time.Hour) }
	f.rotate()
	fi, err := os.ReadDir(telemetry.LocalDir)
	if err != nil || len(fi) != 3 {
		t.Fatalf("err=%v, len(fi) = %d, want 3", err, len(fi))
	}
}

// These were useful while debugging failed mapping
func (s *counterState) String() string {
	if s == nil {
		return "nil"
	}
	return s.load().String()
}

func (b counterStateBits) String() string {
	rdrs := b.readers()
	locked := b.locked()
	if locked {
		rdrs = 0 // rdrs == 1<<30 - 1
	}
	havePtr := b&stateHavePtr != 0
	extra := uint64(b&stateExtra) >> stateExtraShift
	return fmt.Sprintf("rdrs:0x%x locked:%v\thavePtr:%v\textra:%d", rdrs, locked, havePtr, extra)
}
