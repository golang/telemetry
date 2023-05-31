// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/fullstorydev/emulators/storage/gcsemu"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/telemetry/godev/internal/testenv"
)

type jsondata struct {
	Tars string
	Case string
	Kipp map[string]int
}

var writeData = jsondata{
	Tars: "foo",
	Case: "bar",
	Kipp: map[string]int{"plex": 0},
}

func TestGCStore(t *testing.T) {
	testenv.NeedsLocalhostNet(t)

	server, err := gcsemu.NewServer("localhost:0", gcsemu.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	addr := server.Addr
	if err := os.Setenv("STORAGE_EMULATOR_HOST", addr); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	s, err := NewGCStore(ctx, "go-test-project", "test-bucket")
	if err != nil {
		t.Fatal(err)
	}

	writeObj, err := s.Writer(ctx, "test-object")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(writeObj).Encode(writeData); err != nil {
		t.Fatal(err)
	}
	if err := writeObj.Close(); err != nil {
		t.Fatal(err)
	}

	readObj, err := s.Reader(ctx, "test-object")
	if err != nil {
		t.Fatal(err)
	}
	var readData jsondata
	if err := json.NewDecoder(readObj).Decode(&readData); err != nil {
		t.Fatal(err)
	}
	if err := readObj.Close(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(writeData, readData); diff != "" {
		t.Errorf("data write read mismatch (-wrote +read):\n%s", diff)
	}
}

func TestFSStore(t *testing.T) {
	ctx := context.Background()
	s, err := NewFSStore(ctx, "testdata")
	if err != nil {
		t.Fatal(err)
	}

	writeObj, err := s.Writer(ctx, "test-file")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(writeObj).Encode(writeData); err != nil {
		t.Fatal(err)
	}
	if err := writeObj.Close(); err != nil {
		t.Fatal(err)
	}

	readObj, err := s.Reader(ctx, "test-file")
	if err != nil {
		t.Fatal(err)
	}
	var readData jsondata
	if err := json.NewDecoder(readObj).Decode(&readData); err != nil {
		t.Fatal(err)
	}
	if err := readObj.Close(); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(writeData, readData); diff != "" {
		t.Errorf("data write read mismatch (-wrote +read):\n%s", diff)
	}

	if err := os.Remove(path.Join("testdata", "test-file")); err != nil {
		t.Fatal(err)
	}
}
