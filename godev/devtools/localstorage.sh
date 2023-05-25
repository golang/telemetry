#!/usr/bin/env bash

# Copyright 2023 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
set -e

# Script for running a Google Cloud Storage API emulator.
#
# Connect to the emulator by adding an endpoint option when creating a storage
# client.
#
#   client, err := storage.NewClient(
#     context.Background(),
#     option.WithEndpoint("http://localhost:8081/storage/v1/"),
#   )
#
# Or by setting STORAGE_EMULATOR_HOST=localhost:8081 when running your program.
#
#   STORAGE_EMULATOR_HOST=localhost:8081 go run ./cmd/my-command
#
#   OR
#
#	  if err := os.Setenv("STORAGE_EMULATOR_HOST", "localhost:8081"); err != nil {
#		  log.Fatal(err)
#   }
#
# By default, the emulator will read and write from godev/.localstorage. Pass a
# directory to override that location.
#
#   ./devtools/localstorage.sh ~/storage

version=v0.0.0-20230523204811-eccb7d2267b0
port=8081
dir=.localstorage
if [ ! -z "$1" ]; then
  dir="$1"
fi

if ! command -v gcsemulator &> /dev/null
then
  echo "Command gcsemulator could not be found. Installing..."
  go install github.com/fullstorydev/emulators/storage/cmd/gcsemulator@$version
fi

gcsemulator -port $port -dir $dir
