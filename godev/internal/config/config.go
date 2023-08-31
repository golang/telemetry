// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"flag"
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// ServerPort is the port for the cmd/telemetrygodev.
	ServerPort string

	// WorkerPort is the port for the cmd/worker.
	WorkerPort string

	// WorkerURL is the location url of the worker used in queueing worker tasks.
	WorkerURL string

	// ProjectID is a GCP project ID.
	ProjectID string

	// LocationID is a GCP location ID.
	LocationID string

	// QueueID is the name of the queue for worker tasks.
	QueueID string

	// ServiceAccount is the service account used when queueing worker tasks.
	ServiceAccount string

	// ClientID is the OAuth client used in authentication for queue tasks.
	ClientID string

	// StorageEmulatorHost is a network address for a Cloud Storage emulator.
	StorageEmulatorHost string

	// LocalStorage is a directory for storage I/O used when the using the filesystem
	// or storage emulator modes.
	LocalStorage string

	// MergedBucket is the storage bucket for merged reports. The worker merges the
	// reports from the upload bucket and saves them here.
	MergedBucket string

	// UploadBucket is the storage bucket for report uploads.
	UploadBucket string

	// ChartDataBucket is the storage bucket for chart data.
	ChartDataBucket string

	// UploadConfig is the location of the upload config deployed with the server.
	// It's used to validate telemetry uploads.
	UploadConfig string

	// MaxRequestBytes is the maximum request body size the server will allow.
	MaxRequestBytes int64

	// RequestTimeout is the default request timeout for the server.
	RequestTimeout time.Duration

	// UseGCS is true if the server should use the Cloud Storage API for reading and
	// writing storage objects.
	UseGCS bool

	// DevMode is true if the server should read content files from the filesystem.
	// If false, content files are read from the embed.FS in ../content.go.
	DevMode bool
}

var (
	devMode = flag.Bool("dev", false, "load static content and templates from the filesystem")
	useGCS  = flag.Bool("gcs", false, "use Cloud Storage for reading and writing storage objects")
)

// NewConfig returns a new config. Getting the config should follow a call to flag.Parse.
func NewConfig() *Config {
	environment := env("GO_TELEMETRY_ENV", "local")
	return &Config{
		ServerPort:          env("PORT", "8080"),
		WorkerPort:          env("PORT", "8082"),
		WorkerURL:           env("GO_TELEMETRY_WORKER_URL", "http://localhost:8082"),
		ProjectID:           env("GO_TELEMETRY_PROJECT_ID", ""),
		LocationID:          env("GO_TELEMETRY_LOCATION_ID", ""),
		QueueID:             environment + "-worker-tasks",
		ServiceAccount:      env("GO_TELEMETRY_SERVICE_ACCOUNT", ""),
		ClientID:            env("GO_TELEMETRY_CLIENT_ID", ""),
		StorageEmulatorHost: env("GO_TELEMETRY_STORAGE_EMULATOR_HOST", "localhost:8081"),
		LocalStorage:        env("GO_TELEMETRY_LOCAL_STORAGE", ".localstorage"),
		ChartDataBucket:     environment + "-telemetry-charted",
		MergedBucket:        environment + "-telemetry-merged",
		UploadBucket:        environment + "-telemetry-uploaded",
		UploadConfig:        env("GO_TELEMETRY_UPLOAD_CONFIG", "./config/config.json"),
		MaxRequestBytes:     env("GO_TELEMETRY_MAX_REQUEST_BYTES", int64(100*1024)),
		RequestTimeout:      10 * time.Duration(time.Minute),
		UseGCS:              *useGCS,
		DevMode:             *devMode,
	}
}

// env reads a value from the os environment and returns a fallback
// when it is unset.
func env[T string | int64](key string, fallback T) T {
	if s, ok := os.LookupEnv(key); ok {
		switch any(fallback).(type) {
		case string:
			return any(s).(T)
		case int64:
			v, err := strconv.Atoi(s)
			if err != nil {
				log.Fatalf("bad value %q for %s: %v", s, key, err)
			}
			return any(v).(T)
		}
	}
	return fallback
}
