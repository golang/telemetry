// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"os"
)

type config struct {
	// Port is the port your HTTP server should listen on.
	Port string

	// ProjectID is a GCP project ID.
	ProjectID string

	// StorageEmulatorHost is a network address for a Cloud Storage emulator.
	StorageEmulatorHost string

	// LocalStorage is a directory for storage I/O used when the using the filesystem
	// or storage emulator modes.
	LocalStorage string

	// UploadBucket is the storage bucket for report uploads.
	UploadBucket string

	// UploadConfig is the location of the upload config deployed with the server.
	// It's used to validate telemetry uploads.
	UploadConfig string

	// UseGCS is true if the server should use the Cloud Storage API for reading and
	// writing storage objects.
	UseGCS bool

	// DevMode is true if the server should read content files from the filesystem.
	// If false, content files are read from the embed.FS in ../content.go.
	DevMode bool
}

// onCloudRun reports whether the current process is running on Cloud Run.
func (c *config) onCloudRun() bool {
	// Use the presence of the environment variables provided by Cloud Run.
	// See https://cloud.google.com/run/docs/reference/container-contract.
	for _, ev := range []string{"K_SERVICE", "K_REVISION", "K_CONFIGURATION"} {
		if os.Getenv(ev) == "" {
			return false
		}
	}
	return true
}

var (
	devMode = flag.Bool("dev", false, "load static content and templates from the filesystem")
	useGCS  = flag.Bool("gcs", false, "use Cloud Storage for reading and writing storage objects")
)

// newConfig returns a new config. Getting the config should follow a call to flag.Parse.
func newConfig() *config {
	// K_SERVICE is a Cloud Run environment variable.
	service := env("K_SERVICE", "local-telemetry")
	return &config{
		Port:                env("PORT", "8080"),
		ProjectID:           env("GO_TELEMETRY_PROJECT_ID", "go-telemetry"),
		StorageEmulatorHost: env("GO_TELEMETRY_STORAGE_EMULATOR_HOST", "localhost:8081"),
		LocalStorage:        env("GO_TELEMETRY_LOCAL_STORAGE", ".localstorage"),
		UploadBucket:        service + "-uploaded",
		UploadConfig:        env("GO_TELEMETRY_UPLOAD_CONFIG", "../config/config.json"),
		UseGCS:              *useGCS,
		DevMode:             *devMode,
	}
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
