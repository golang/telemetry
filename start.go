// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telemetry

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sync/errgroup"
	"golang.org/x/telemetry/counter"
	"golang.org/x/telemetry/internal/crashmonitor"
	"golang.org/x/telemetry/internal/telemetry"
	"golang.org/x/telemetry/upload"
)

// Config controls the behavior of [Start].
type Config struct {
	// ReportCrashes, if set, will enable crash reporting.
	// ReportCrashes uses the [debug.SetCrashOutput] mechanism, which is a
	// process-wide resource.
	// Do not make other calls to that function within your application.
	// ReportCrashes is a non-functional unless the program is built with go1.23+.
	ReportCrashes bool

	// Upload causes this program to periodically upload approved counters
	// from the local telemetry database to telemetry.go.dev.
	//
	// This option has no effect unless the user has given consent
	// to enable data collection, for example by running
	// cmd/gotelemetry or affirming the gopls dialog.
	//
	// (This feature is expected to be used only by gopls.
	// Longer term, the go command may become the sole program
	// responsible for uploading.)
	Upload bool
}

// Start initializes telemetry using the specified configuration.
//
// Start opens the local telemetry database so that counter increment
// operations are durably recorded in the local file system.
//
// If [Config.Upload] is set, and the user has opted in to telemetry
// uploading, this process may attempt to upload approved counters
// to telemetry.go.dev.
//
// If [Config.ReportCrashes] is set, any fatal crash will be
// recorded by incrementing a counter named for the stack of the
// first running goroutine in the traceback.
//
// If either of these flags is set, Start re-executes the current
// executable as a child process, in a special mode in which it
// acts as a telemetry sidecar for the parent process (the application).
// In that mode, the call to Start will never return, so Start must
// be called immediately within main, even before such things as
// inspecting the command line. The application should avoid expensive
// steps or external side effects in init functions, as they will
// be executed twice (parent and child).
func Start(config Config) {
	counter.Open()

	// Crash monitoring and uploading both require a sidecar process.
	if (config.ReportCrashes && crashmonitor.Supported()) || config.Upload {
		if os.Getenv(telemetryChildVar) != "" {
			child(config)
			os.Exit(0)
		}

		parent(config)
	}
}

var daemonize = func(cmd *exec.Cmd) {}

const telemetryChildVar = "X_TELEMETRY_CHILD"

func parent(config Config) {
	// This process is the application (parent).
	// Fork+exec the telemetry child.
	exe, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command(exe, "** telemetry **") // this unused arg is just for ps(1)
	daemonize(cmd)
	cmd.Env = append(os.Environ(), telemetryChildVar+"=1")

	// The child process must write to a log file, not
	// the stderr file it inherited from the parent, as
	// the child may outlive the parent but should not prolong
	// the life of any pipes created (by the grandparent)
	// to gather the output of the parent.
	//
	// By default, we discard the child process's stderr,
	// but in line with the uploader, log to a file in local/debug
	// only if that directory was created by the user.
	localDebug := filepath.Join(telemetry.LocalDir, "debug")
	fd, err := os.Stat(localDebug)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Fatalf("failed to stat debug directory: %v", err)
		}
	} else if fd.IsDir() {
		// local/debug exists and is a directory. Set stderr to a log file path
		// in local/debug.
		childLogPath := filepath.Join(localDebug, "sidecar.log")
		childLog, err := os.OpenFile(childLogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("opening sidecar log file for child: %v", err)
		}
		defer childLog.Close()
		cmd.Stderr = childLog
	}

	if config.ReportCrashes {
		pipe, err := cmd.StdinPipe()
		if err != nil {
			log.Fatalf("StdinPipe: %v", err)
		}

		crashmonitor.Parent(pipe.(*os.File)) // (this conversion is safe)
	}

	if err := cmd.Start(); err != nil {
		log.Fatalf("can't start telemetry child process: %v", err)
	}
}

func child(config Config) {
	log.SetPrefix(fmt.Sprintf("telemetry-sidecar (pid %v): ", os.Getpid()))

	// Start crashmonitoring and uploading depending on what's requested
	// and wait for the longer running child to complete before exiting:
	// if we collected a crash before the upload finished, wait for the
	// upload to finish before exiting
	var g errgroup.Group

	if config.Upload {
		g.Go(func() error {
			uploaderChild()
			return nil
		})
	}
	if config.ReportCrashes {
		g.Go(func() error {
			crashmonitor.Child()
			return nil
		})
	}
	g.Wait()
}

func uploaderChild() {
	// TODO(matloob): Do rate-limiting here.
	upload.Run(&upload.Control{Logger: os.Stderr})
}
