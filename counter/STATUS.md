# Issues in the counter package

## `Open()`

`Open()` is an exported function that asociates counters with a
 counter file. Should it remain exported? It is presently called in
 `init()` in upload.go. [We don't want to call it if telemetry is off,
 but it could be called conditionally in counter.init(), which would
 create a disk file even if no counter is ever incremented.]

## Generating reports and uploading

The simplest story would be to generate and upload reports when the
counter file is rotated, but uploads might fail, so that would not be
enough. The proposed way is to start a separate command each time the
counter package starts.

The code could be in the counter package, or in a separate package, or
in a separate command, for instance 'go telemetry upload'. The latter ties
updates to the 'go' command release cycle, and separates the upload code from the
counter package. Thus the code will be in the upload package.

The init() function in upload.go handles this. It checks to see if the
program was invoked with a single argument `__telemetry_upload__`, and if
so, executes the code to generate reports and upload them. If not it spawns
a copy of the current program with that argument.

This commentary can be moved to the upload package when it is checked in.

## TODOs

There are a bunch of TODOs. Also there are many places in the upload code
where log messages are written, but it's unclear how to recover from the
errors. The log messages are written to files named `telemetry-<pid>.log`
in `os.TempDir()`.

