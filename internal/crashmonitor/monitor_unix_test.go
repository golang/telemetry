// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package crashmonitor_test

import (
	"log"
	"runtime"
	"syscall"
	"unsafe"
)

func init() {
	childSystemstackCrash = func() {
		// ReadMemStats writes to the supplied variable while
		// running on the system stack. We pass it a readonly
		// variable to trigger a SIGBUS.
		runtime.ReadMemStats(newReadOnly[runtime.MemStats]())
	}
}

func newReadOnly[T any]() *T {
	const PageSize = 4096
	length := (unsafe.Sizeof(*new(T)) + PageSize - 1) &^ (PageSize - 1)
	data, err := syscall.Mmap(-1, 0, int(length), syscall.PROT_READ, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		log.Fatalf("mmap: %v", err)
	}
	return (*T)(unsafe.Pointer(&data[0]))
}
