// Copyright 2024 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmap_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"

	"golang.org/x/telemetry/internal/mmap"
	"golang.org/x/telemetry/internal/testenv"
)

// If the sharedFileEnv environment variable is set,
// increment an atomic value in that file rather than
// run the test.
const sharedFileEnv = "MMAP_TEST_SHARED_FILE"

func TestMain(m *testing.M) {
	if name := os.Getenv(sharedFileEnv); name != "" {
		_, mapping, err := openMapped(name)
		if err != nil {
			log.Fatalf("openMapped failed: %v", err)
		}

		v := (*atomic.Uint64)(unsafe.Pointer(&mapping.Data[0]))
		v.Add(1)
		// Exit without explicitly calling munmap/close.
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func openMapped(name string) (*os.File, mmap.Data, error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, mmap.Data{}, fmt.Errorf("open failed: %v", err)
	}
	data, err := mmap.Mmap(f, nil)
	if err != nil {
		return nil, mmap.Data{}, fmt.Errorf("Mmap failed: %v", err)
	}
	return f, data, nil
}

func TestSharedMemory(t *testing.T) {
	testenv.SkipIfUnsupportedPlatform(t)

	// This test verifies that Mmap'ed files are usable for concurrent
	// cross-process atomic operations.

	dir := t.TempDir()
	name := filepath.Join(dir, "shared.count")

	var zero [8]byte
	if err := os.WriteFile(name, zero[:], 0666); err != nil {
		t.Fatal(err)
	}

	// Fork+exec the current test process.
	// Child processes atomically increment the counter file in shared memory.

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	const concurrency = 100
	var wg sync.WaitGroup
	env := append(os.Environ(), sharedFileEnv+"="+name)
	for i := 0; i < concurrency; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(exe)
			cmd.Env = env

			if err := cmd.Run(); err != nil {
				t.Errorf("subcommand #%d failed: %v", i, err)
			}
		}()
	}

	wg.Wait()

	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("final read failed: %v", err)
	}
	v := (*atomic.Uint64)(unsafe.Pointer(&data[0]))
	if got := v.Load(); got != concurrency {
		t.Errorf("incremented %d times, want %d", got, concurrency)
	}
}
