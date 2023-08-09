// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graphconfig

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// Parse parses GraphConfig records from the provided raw data, returning an
// error if the config has invalid syntax. See the package documentation for a
// description of the record syntax.
//
// Even with correct syntax, the resulting GraphConfig may not meet all the
// requirements described in the package doc. Call [Validate] to check whether
// the config data is coherent.
func Parse(data []byte) ([]GraphConfig, error) {
	// Collect field information for the record type.
	var (
		prefixes []string                               // for parse errors
		fields   = make(map[string]reflect.StructField) // key -> struct field
	)
	{
		typ := reflect.TypeOf(GraphConfig{})
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			key := strings.ToLower(f.Name)
			if _, ok := fieldParsers[key]; !ok {
				panic(fmt.Sprintf("no parser for field %q", f.Name))
			}
			prefixes = append(prefixes, "'"+key+":'")
			fields[key] = f
		}
		sort.Strings(prefixes)
	}

	// Read records, separated by '---'
	var (
		records    []GraphConfig
		inProgress = new(GraphConfig)      // record value currently being parsed
		set        = make(map[string]bool) // fields that are set so far; empty records are skipped
	)
	flushRecord := func() {
		if len(set) > 0 { // only flush non-empty records
			records = append(records, *inProgress)
		}
		inProgress = new(GraphConfig)
		set = make(map[string]bool)
	}

	for lineNum, line := range strings.Split(string(data), "\n") {
		if line == "---" {
			flushRecord()
			continue
		}
		text, _, _ := strings.Cut(line, "#") // trim comments

		var key string
		for k := range fields {
			prefix := k + ":"
			if strings.HasPrefix(text, prefix) {
				key = k
				text = text[len(prefix):]
				break
			}
		}

		text = strings.TrimSpace(text)
		if text == "" {
			// Check for empty lines before the field == nil check below.
			// Lines consisting only of whitespace and comments are OK.
			continue
		}
		if key == "" {
			return nil, fmt.Errorf("line %d: invalid line %q: lines must be '---', consist only of whitespace/comments, or start with %s", lineNum, line, strings.Join(prefixes, ", "))
		}
		field := fields[key]
		v := reflect.ValueOf(inProgress).Elem().FieldByName(field.Name)
		if set[key] && field.Type.Kind() != reflect.Slice {
			return nil, fmt.Errorf("line %d: field %s may not be repeated", lineNum, strings.ToLower(field.Name))
		}
		parser := fieldParsers[key]
		if err := parser(v, text); err != nil {
			return nil, fmt.Errorf("line %d: field %q: %v", lineNum, field.Name, err)
		}
		set[key] = true
	}
	flushRecord()
	return records, nil
}

// A fieldParser parses the provided input and writes to v, which must be
// addressable.
type fieldParser func(v reflect.Value, input string) error

var fieldParsers = map[string]fieldParser{
	"title":       parseString,
	"description": parseString,
	"issue":       parseSlice(parseString),
	"type":        parseString,
	"program":     parseString,
	"counter":     parseString,
	"depth":       parseInt,
	"error":       parseFloat,
	"version":     parseSlice(parseVersionInterval),
}

func parseString(v reflect.Value, input string) error {
	v.SetString(input)
	return nil
}

func parseInt(v reflect.Value, input string) error {
	i, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int value %q", input)
	}
	v.SetInt(i)
	return nil
}

func parseFloat(v reflect.Value, input string) error {
	f, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return fmt.Errorf("invalid float value %q", input)
	}
	v.SetFloat(f)
	return nil
}

func parseSlice(elemParser fieldParser) fieldParser {
	return func(v reflect.Value, input string) error {
		elem := reflect.New(v.Type().Elem()).Elem()
		v.Set(reflect.Append(v, elem))
		elem = v.Index(v.Len() - 1)
		if err := elemParser(elem, input); err != nil {
			return err
		}
		return nil
	}
}

func parseVersionInterval(v reflect.Value, input string) error {
	bad := func() error {
		return fmt.Errorf("versions must be of the form v<version> or [v<low>, v<high>]")
	}
	if input[0] != '[' { // a single version value
		vi := VersionInterval{Low: input, High: input}
		v.Set(reflect.ValueOf(vi))
		return nil
	}
	if input[len(input)-1] != ']' {
		return bad()
	}
	input = input[1 : len(input)-1]
	parts := strings.Split(input, ",")
	if len(parts) != 2 {
		return bad()
	}
	low, high := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	var vi VersionInterval
	if low != "" {
		vi.Low = low
	}
	if high != "" {
		vi.High = high
	}
	v.Set(reflect.ValueOf(vi))
	return nil
}
