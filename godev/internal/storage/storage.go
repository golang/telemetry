// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package storage provides an interface and types for reading and writing
// files to Cloud Storage or a filesystem.
package storage

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

var _ Store = &gcStore{}
var _ Store = &fsStore{}

type Store interface {
	// Writer creates a new object if it does not exist. Any previous object with the same
	// name will be replaced.
	Writer(_ context.Context, object string) (io.WriteCloser, error)

	// Reader creates a new Reader to read the contents of the object.
	Reader(_ context.Context, object string) (io.ReadCloser, error)

	// List returns the names of objects in the bucket that match the prefix.
	List(_ context.Context, prefix string) (*ObjectIterator, error)

	// Location returns the URI representing the location of the store. It may be
	// a URL for a cloud storage bucket or directory on a filesystem.
	Location() string
}

type gcStore struct {
	bucket   *storage.BucketHandle
	location string
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
	loc := "https://storage.googleapis.com/" + bucket
	return &gcStore{bkt, loc}, nil
}

func (s *gcStore) Writer(ctx context.Context, object string) (io.WriteCloser, error) {
	obj := s.bucket.Object(object)
	w := obj.NewWriter(ctx)
	return w, nil
}

func (s *gcStore) Reader(ctx context.Context, object string) (io.ReadCloser, error) {
	obj := s.bucket.Object(object)
	r, err := obj.NewReader(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil, ErrObjectNotExist
	}
	return r, err
}

func (s *gcStore) List(ctx context.Context, prefix string) (*ObjectIterator, error) {
	query := &storage.Query{Prefix: prefix}
	it := s.bucket.Objects(ctx, query)
	return &ObjectIterator{
		Next: func() (string, error) {
			attrs, err := it.Next()
			if errors.Is(err, iterator.Done) {
				return "", ErrObjectIteratorDone
			}
			if err != nil {
				return "", err
			}
			return attrs.Name, nil
		},
	}, nil
}

func (s *gcStore) Location() string {
	return s.location
}

type fsStore struct {
	dir, bucket, location string
}

// NewFSStore returns a store for that writes to a directory. If the directory does
// not exist it will be created.
func NewFSStore(ctx context.Context, dir, bucket string) (*fsStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, bucket), os.ModePerm); err != nil {
		return nil, err
	}
	uri, err := filepath.Abs(filepath.Join(dir, filepath.Clean(bucket)))
	if err != nil {
		return nil, err
	}
	return &fsStore{dir, bucket, uri}, nil
}

func (s *fsStore) Writer(ctx context.Context, object string) (io.WriteCloser, error) {
	name := filepath.Join(s.dir, s.bucket, filepath.FromSlash(object))
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(name)
}

func (s *fsStore) Reader(ctx context.Context, object string) (io.ReadCloser, error) {
	r, err := os.Open(filepath.Join(s.dir, s.bucket, filepath.FromSlash(object)))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrObjectNotExist
	}
	return r, err
}

func (s *fsStore) List(ctx context.Context, prefix string) (*ObjectIterator, error) {
	var elems []string
	if err := fs.WalkDir(
		os.DirFS(filepath.Join(s.dir, s.bucket)),
		".",
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			if strings.HasPrefix(path, prefix) {
				elems = append(elems, path)
			}
			return nil
		}); err != nil {
		return nil, err
	}
	i := 0
	return &ObjectIterator{
		Next: func() (string, error) {
			if i >= len(elems) {
				return "", ErrObjectIteratorDone
			}
			elem := elems[i]
			i++
			return elem, nil
		},
	}, nil
}

func (s *fsStore) Location() string {
	return s.location
}

var ErrObjectIteratorDone = errors.New("object iterator done")
var ErrObjectNotExist = errors.New("object does not exist")

type ObjectIterator struct {
	Next func() (elem string, err error)
}
