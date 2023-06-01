// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package counter

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/telemetry"
)

// On the disk, and upstream, stack counters look like sets of
// regular counters with names that include newlines.

// a StackCounter is the in-memory knowledge about a stack counter.
// StackCounters are more expensive to use than regular Counters,
// requiring, at a minimum, a call to runtime.Callers.
type StackCounter struct {
	name  string
	depth int

	mu sync.Mutex
	// as this is a detail of the implementation, it could be replaced
	// by a more efficient mechanism
	stacks []stack
}

type stack struct {
	pcs     []uintptr
	counter *Counter
}

func NewStack(name string, depth int) *StackCounter {
	return &StackCounter{name: name, depth: depth}
}

// Inc increments a stack counter. It computes the caller's stack and
// looks up the corresponding counter. It then increments that counter,
// creating it if necessary.
func (c *StackCounter) Inc() {
	if !telemetry.Enabled {
		return
	}
	pcs := make([]uintptr, c.depth)
	n := runtime.Callers(2, pcs) // caller of Inc
	pcs = pcs[:n]
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.stacks {
		if eq(s.pcs, pcs) {
			s.counter.Inc()
			return
		}
	}
	// have to create the new counter's name, and the new counter itself
	locs := make([]string, c.depth)
	frs := runtime.CallersFrames(pcs)
	for i := 0; i < n; i++ {
		fr, more := frs.Next()
		_, pcline := fr.Func.FileLine(pcs[i])
		entryptr := fr.Func.Entry()
		_, entryline := fr.Func.FileLine(entryptr)
		locs[i] = fmt.Sprintf("%s:%d", fr.Function, pcline-entryline)
		if pcline-entryline < 0 {
			// should never happen, remove before production TODO(pjw)
			log.Printf("i=%d, f=%s, pcline=%d entryLine=%d", i, fr.Function, pcline, entryline)
			log.Printf("pcs[i]=%x, entryptr=%x", pcs[i], entryptr)
		}
		if !more {
			break
		}
	}

	name := c.name + "\n" + strings.Join(locs, "\n")
	if len(name) > maxNameLen {
		return // fails silently, every time
	}
	ctr := New(name)
	c.stacks = append(c.stacks, stack{pcs: pcs, counter: ctr})
	ctr.Inc()
}

// Names reports all the counter names associated with a StackCounter.
func (c *StackCounter) Names() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, len(c.stacks))
	for i, s := range c.stacks {
		names[i] = s.counter.Name()
	}
	return names
}

// Counters returns the known Counters for a StackCounter.
// There may be more in the count file.
func (c *StackCounter) Counters() []*Counter {
	c.mu.Lock()
	defer c.mu.Unlock()
	counters := make([]*Counter, len(c.stacks))
	for i, s := range c.stacks {
		counters[i] = s.counter
	}
	return counters
}

func eq(a, b []uintptr) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
