// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package storage provides an interface and types for reading and writing
// files to Cloud Storage or a filesystem.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

var (
	_ BucketHandle = &GCSBucket{}
	_ BucketHandle = &FSBucket{}
)

var (
	ErrObjectIteratorDone = errors.New("object iterator done")
	ErrObjectNotExist     = errors.New("object not exist")
)

type BucketHandle interface {
	Object(name string) ObjectHandle
	Objects(ctx context.Context, prefix string) ObjectIterator
	URI() string
}

type ObjectHandle interface {
	NewReader(ctx context.Context) (io.ReadCloser, error)
	NewWriter(ctx context.Context) (io.WriteCloser, error)
}

type ObjectIterator interface {
	Next() (name string, err error)
}

type GCSBucket struct {
	*storage.BucketHandle
	url string
}

// Copy read the content from the source and write the content to the
// destination.
func Copy(ctx context.Context, dst, src ObjectHandle) error {
	srcGCS, srcOk := src.(*GCSObject)
	dstGCS, dstOk := dst.(*GCSObject)
	if srcOk && dstOk {
		if _, err := dstGCS.CopierFrom(srcGCS.ObjectHandle).Run(ctx); err != nil {
			return fmt.Errorf("failed to use gcs copier to copy from %s to %s: %w", srcGCS.ObjectName(), dstGCS.ObjectName(), err)
		}
		return nil
	}

	reader, err := src.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader for source: %w", err)
	}
	defer reader.Close()

	writer, err := dst.NewWriter(ctx)
	if err != nil {
		return fmt.Errorf("failed to create writer for destination: %w", err)
	}
	defer writer.Close()

	if _, err := io.Copy(writer, reader); err != nil {
		return err
	}

	return nil
}

func NewGCSBucket(ctx context.Context, project, bucket string) (BucketHandle, error) {
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
	url := "https://storage.googleapis.com/" + bucket
	return &GCSBucket{bkt, url}, nil
}

func (b *GCSBucket) Object(name string) ObjectHandle {
	return NewGCSObject(b, name)
}

type GCSObject struct {
	*storage.ObjectHandle
}

func NewGCSObject(b *GCSBucket, name string) ObjectHandle {
	return &GCSObject{b.BucketHandle.Object(name)}
}

func (o *GCSObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return o.ObjectHandle.NewReader(ctx)
}

func (o *GCSObject) NewWriter(ctx context.Context) (io.WriteCloser, error) {
	return o.ObjectHandle.NewWriter(ctx), nil
}

func (b *GCSBucket) Objects(ctx context.Context, prefix string) ObjectIterator {
	return &GCSObjectIterator{b.BucketHandle.Objects(ctx, &storage.Query{Prefix: prefix})}
}

type GCSObjectIterator struct {
	*storage.ObjectIterator
}

func (it *GCSObjectIterator) Next() (elem string, err error) {
	o, err := it.ObjectIterator.Next()
	if errors.Is(err, iterator.Done) {
		return "", ErrObjectIteratorDone
	}
	if err != nil {
		return "", err
	}
	return o.Name, nil
}

func (b *GCSBucket) URI() string {
	return b.url
}

type FSBucket struct {
	dir, bucket, uri string
}

func NewFSBucket(ctx context.Context, dir, bucket string) (BucketHandle, error) {
	if err := os.MkdirAll(filepath.Join(dir, bucket), os.ModePerm); err != nil {
		return nil, err
	}
	uri, err := filepath.Abs(filepath.Join(dir, filepath.Clean(bucket)))
	if err != nil {
		return nil, err
	}
	return &FSBucket{dir, bucket, uri}, nil
}

func (b *FSBucket) Object(name string) ObjectHandle {
	return NewFSObject(b, name)
}

type FSObject struct {
	filename string
}

func NewFSObject(b *FSBucket, name string) ObjectHandle {
	filename := filepath.Join(b.dir, b.bucket, filepath.FromSlash(name))
	return &FSObject{filename}
}

func (o *FSObject) Filename() string {
	return o.filename
}

func (o *FSObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	r, err := os.Open(o.filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrObjectNotExist
	}
	return r, err
}

func (o *FSObject) NewWriter(ctx context.Context) (io.WriteCloser, error) {
	if err := os.MkdirAll(filepath.Dir(o.filename), os.ModePerm); err != nil {
		return nil, err
	}
	return os.Create(o.filename)
}

func (b *FSBucket) Objects(ctx context.Context, prefix string) ObjectIterator {
	var names []string
	err := fs.WalkDir(
		os.DirFS(filepath.Join(b.dir, b.bucket)),
		".",
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			name := filepath.ToSlash(path)
			if strings.HasPrefix(name, prefix) {
				names = append(names, name)
			}
			return nil
		},
	)
	return &FSObjectIterator{names, err, 0}
}

type FSObjectIterator struct {
	names []string
	err   error
	index int
}

func (it *FSObjectIterator) Next() (name string, err error) {
	if it.index >= len(it.names) {
		return "", ErrObjectIteratorDone
	}
	name = it.names[it.index]
	it.index++
	return name, nil
}

func (b *FSBucket) URI() string {
	return b.uri
}
