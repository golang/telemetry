// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build compiler_bootstrap

package counter

import "fmt"

func Add(string, int64) {}
func Inc(string)        {}
func Open()             {}

type Counter struct{ name string }

func New(name string) *Counter  { return &Counter{name} }
func (c *Counter) Add(n int64)  {}
func (c *Counter) Inc()         {}
func (c *Counter) Name() string { return c.name }

type File struct {
	Meta  map[string]string
	Count map[string]uint64
}

func Parse(filename string, data []byte) (*File, error) { return nil, fmt.Errorf("unimplemented") }
