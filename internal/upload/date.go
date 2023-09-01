// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package upload

import (
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/telemetry/internal/counter"
)

// time and date handling

// all the upload processing takes place (conceptually) at
// a single instant. Most of the time this wouldn't matter
// but it protects against time skew if time.Now
// increases the day between calls, as might happen (rarely) by chance
// or if there are long scheduling delays between calls.
var thisInstant = time.Now().UTC()

var distantPast = 21 * 24 * time.Hour

// reports that are too old (21 days) are not uploaded
func tooOld(date string) bool {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		logger.Printf("tooOld: %v", err)
		return false
	}
	age := thisInstant.Sub(t)
	return age > distantPast
}

// return the expiry date of a countfile in YYYY-MM-DD format
func expiryDate(fname string) string {
	t := expiry(fname)
	if t.IsZero() {
		return ""
	}
	// PJW: is this sometimes off by a day?
	year, month, day := t.Date()
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// a time in the far future for the expiry time with errors
var farFuture = time.UnixMilli(1 << 62)

// expiry returns the expiry time of a countfile. For errors
// it returns a time far in the future, so that erroneous files
// don't look like they should be used.
func expiry(fname string) time.Time {
	parsed, err := parse(fname)
	if err != nil {
		logger.Printf("expiry Parse: %v for %s", err, fname)
		return farFuture // don't process it, whatever it is
	}
	expiry, err := time.Parse(time.RFC3339, parsed.Meta["TimeEnd"])
	if err != nil {
		logger.Printf("time.Parse: %v for %s", err, fname)
		return farFuture // don't process it, whatever it is
	}
	return expiry
}

// stillOpen returns true if the counter file might still be active
func stillOpen(fname string) bool {
	expiry := expiry(fname)
	return expiry.After(thisInstant)
}

// avoid parsing count files multiple times
type parsedCache struct {
	mu sync.Mutex
	m  map[string]*counter.File
}

var cache parsedCache

func parse(fname string) (*counter.File, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.m == nil {
		cache.m = make(map[string]*counter.File)
	}
	if f, ok := cache.m[fname]; ok {
		return f, nil
	}
	buf, err := os.ReadFile(fname)
	if err != nil {
		return nil, fmt.Errorf("parse ReadFile: %v for %s", err, fname)
	}
	f, err := counter.Parse(fname, buf)
	if err != nil {

		return nil, fmt.Errorf("parse Parse: %v for %s", err, fname)
	}
	cache.m[fname] = f
	return f, nil
}
