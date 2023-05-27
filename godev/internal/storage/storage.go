// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package storage provides an interface and types for reading and writing
// files to Cloud Storage or a filesystem.
package storage

import (
	"context"
	"io"
	"os"
	"path"

	"cloud.google.com/go/storage"
)

var _ Store = &gcStore{}
var _ Store = &fsStore{}

type Store interface {
	Writer(_ context.Context, object string) (io.WriteCloser, error)
	Reader(_ context.Context, object string) (io.ReadCloser, error)
}

type gcStore struct {
	bucket *storage.BucketHandle
}

// NewGCStore returns a store for that writes to a GCS bucket. If the bucket does
// not exist it will be created.
func NewGCStore(ctx context.Context, project, bucket string) (*gcStore, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	bkt := client.Bucket(bucket)
	// Check if the bucket exists by reading its metadata and on error create the bucket.
	_, err = bkt.Attrs(ctx)
	if err != nil {
		if err := bkt.Create(ctx, project, nil); err != nil {
			return nil, err
		}
	}
	return &gcStore{bkt}, nil
}

// Writer creates a new object if it does not exist. Any previous object with the same
// name will be replaced.
func (s *gcStore) Writer(ctx context.Context, object string) (io.WriteCloser, error) {
	obj := s.bucket.Object(object)
	w := obj.NewWriter(ctx)
	return w, nil
}

// Reader creates a new Reader to read the contents of the object.
func (s *gcStore) Reader(ctx context.Context, object string) (io.ReadCloser, error) {
	obj := s.bucket.Object(object)
	return obj.NewReader(ctx)
}

type fsStore struct {
	dir string
}

// NewFSStore returns a store for that writes to a directory. If the directory does
// not exist it will be created.
func NewFSStore(ctx context.Context, dir string) (*fsStore, error) {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, err
	}
	return &fsStore{dir}, nil
}

// Writer creates a new file if it does not exist. Any previous file with the same
// name will be truncated.
func (s *fsStore) Writer(ctx context.Context, file string) (io.WriteCloser, error) {
	return os.Create(path.Join(s.dir, file))
}

// Reader opens the named file for reading.
func (s *fsStore) Reader(ctx context.Context, file string) (io.ReadCloser, error) {
	return os.Open(path.Join(s.dir, file))
}
