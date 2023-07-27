// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build go1.21

package countertest

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"golang.org/x/telemetry/counter"
)

// TODO(hyangah): move to internal/testenv.
func skipIfUnsupportedPlatform(t *testing.T) {
	t.Helper()
	switch runtime.GOOS {
	case "openbsd", "js", "wasip1", "solaris", "android":
		// BUGS: #60614 - openbsd, #60967 - android , #60968 - solaris #60970 - solaris #60971 - wasip1)
		t.Skip("broken for openbsd etc")
	}
	if runtime.GOARCH == "386" {
		// BUGS: #60615 #60692 #60965 #60967
		t.Skip("broken for GOARCH 386")
	}
}

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "counter")
	if err != nil {
		panic(err)
	}

	Open(tmp)
	os.Exit(m.Run())
}

func TestReadCounter(t *testing.T) {
	skipIfUnsupportedPlatform(t)
	c := counter.New("foobar")

	if got, err := ReadCounter(c); err != nil || got != 0 {
		t.Errorf("ReadCounter = (%v, %v), want (%v, nil)", got, err, 0)
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			c.Inc()
			wg.Done()
		}()
	}
	wg.Wait()
	if got, err := ReadCounter(c); err != nil || got != 100 {
		t.Errorf("ReadCounter = (%v, %v), want (%v, nil)", got, err, 100)
	}
}

func TestReadStackCounter(t *testing.T) {
	skipIfUnsupportedPlatform(t)
	c := counter.NewStack("foobar", 8)

	if got, err := ReadStackCounter(c); err != nil || len(got) != 0 {
		t.Errorf("ReadStackCounter = (%q, %v), want (%v, nil)", got, err, 0)
	}

	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			c.Inc()
			wg.Done()
		}()
	}
	wg.Wait()

	want := map[string]uint64{}
	for _, n := range c.Names() {
		want[n] = 100
	}
	if got, err := ReadStackCounter(c); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ReadStackCounter = (%v, %v), want (%v, nil)", stringify(got), err, stringify(want))
	}
}

func stringify(m map[string]uint64) string {
	kv := make([]string, 0, len(m))
	for k, v := range m {
		kv = append(kv, fmt.Sprintf("%q:%v", k, v))
	}
	slices.Sort(kv)
	return "{" + strings.Join(kv, " ") + "}"
}
