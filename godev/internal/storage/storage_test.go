// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/fullstorydev/emulators/storage/gcsemu"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/telemetry/internal/testenv"
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
	s, err := NewGCSBucket(ctx, "go-test-project", "test-bucket")
	if err != nil {
		t.Fatal(err)
	}

	runTest(t, ctx, s)
}

func TestFSStore(t *testing.T) {
	ctx := context.Background()
	s, err := NewFSBucket(ctx, t.TempDir(), "test-bucket")
	if err != nil {
		t.Fatal(err)
	}
	runTest(t, ctx, s)
}

func runTest(t *testing.T, ctx context.Context, s BucketHandle) {
	// write the object to store
	if err := write(ctx, s, "prefix/test-object", writeData); err != nil {
		t.Fatal(err)
	}
	// read same object from store
	readData, err := read(ctx, s, "prefix/test-object")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(writeData, readData); diff != "" {
		t.Errorf("data write read mismatch (-wrote +read):\n%s", diff)
	}

	// write an object with a different prefix to store
	if err = write(ctx, s, "other-prefix/test-object-2", writeData); err != nil {
		t.Fatal(err)
	}
	// check that prefix matches single object
	it := s.Objects(ctx, "prefix")
	var list1 []string
	for {
		elem, err := it.Next()
		if errors.Is(err, ErrObjectIteratorDone) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		list1 = append(list1, elem)
	}
	if diff := cmp.Diff(list1, []string{"prefix/test-object"}); diff != "" {
		t.Errorf("Objects() mismatch (-want +got):\n%s", diff)
	}

	// check that prefix matches with partial path and separator
	it = s.Objects(ctx, "prefix/test")
	var list2 []string
	for {
		elem, err := it.Next()
		if errors.Is(err, ErrObjectIteratorDone) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		list2 = append(list2, elem)
	}

	if diff := cmp.Diff(list2, []string{"prefix/test-object"}); diff != "" {
		t.Errorf("Objects() mismatch (-want +got):\n%s", diff)
	}
}

func write(ctx context.Context, s BucketHandle, object string, data any) error {
	obj, err := s.Object("prefix/test-object").NewWriter(ctx)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(obj).Encode(writeData); err != nil {
		return err
	}
	return obj.Close()
}

func read(ctx context.Context, s BucketHandle, object string) (any, error) {
	obj, err := s.Object("prefix/test-object").NewReader(ctx)
	if err != nil {
		return nil, err
	}
	var data jsondata
	if err := json.NewDecoder(obj).Decode(&data); err != nil {
		return nil, err
	}
	if err := obj.Close(); err != nil {
		return nil, err
	}
	return data, nil
}
