// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// npmdeps copies npm dependencies to be served by the website into the
// third_party directory. It reads the package.json file to get the list
// of dependencies to copy.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	pkgjsonfile = "package.json"
	source      = "node_modules"
	destination = "third_party"
)

var files = []string{"LICENSE", "package.json"}

type metadata struct {
	Dependencies map[string]string `json:"dependencies"`
	Exports      map[string]string `json:"exports"`
}

func run() error {
	f, err := os.ReadFile(pkgjsonfile)
	if err != nil {
		return err
	}
	var pkgjson metadata
	if err := json.Unmarshal(f, &pkgjson); err != nil {
		return err
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	for name, version := range pkgjson.Dependencies {
		src := filepath.Join(source, name)
		// Added version to the path for long term caching.
		dest := filepath.Join(destination, name+"@"+version)
		org, rest, found := strings.Cut(name, "/")
		if found {
			dest = filepath.Join(destination, org, rest+"@"+version)
		}
		f, err := os.ReadFile(filepath.Join(src, pkgjsonfile))
		if err != nil {
			return err
		}
		var dependency metadata
		if err := json.Unmarshal(f, &dependency); err != nil {
			return err
		}
		for _, name := range files {
			if err := copy(filepath.Join(src, name), filepath.Join(dest, name)); err != nil {
				return err
			}
		}
		umd := dependency.Exports["umd"]
		if err := copy(filepath.Join(src, umd), filepath.Join(dest, umd)); err != nil {
			return err
		}
	}
	return nil
}

func copy(from, to string) error {
	stat, err := os.Stat(from)
	if err != nil {
		return err
	}
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", from)
	}
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(to), os.ModePerm); err != nil {
		return err
	}
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
