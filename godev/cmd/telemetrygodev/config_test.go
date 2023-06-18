// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"reflect"
	"testing"
)

func TestConfig(t *testing.T) {
	want := &config{
		Port:                "8080",
		ProjectID:           "go-telemetry",
		StorageEmulatorHost: "localhost:8081",
		LocalStorage:        ".localstorage",
		UploadBucket:        "local-telemetry-uploaded",
		UploadConfig:        "../config/config.json",
		MaxRequestBytes:     1024 * 100,
	}
	if got := newConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("Config() = %v, want %v", got, want)
	}
}
