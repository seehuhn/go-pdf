// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package bundled

import (
	"sync"
	"testing"

	"seehuhn.de/go/pdf/font/type1"
)

// A font the cache names is read once, however many instances are asked for,
// and a font nobody asks for is not read at all.
func TestGetReadsEachFontOnce(t *testing.T) {
	r := newReader()
	c := New([]string{"Helvetica", "Times-Roman"}, r.read)

	for range 3 {
		if _, err := c.Get("Helvetica"); err != nil {
			t.Fatal(err)
		}
	}

	if got := r.reads("Helvetica"); got != 1 {
		t.Errorf("three instances took %d reads, want 1", got)
	}
	if got := r.reads("Times-Roman"); got != 0 {
		t.Errorf("a font nobody asked for was read %d times, want 0", got)
	}
}

// The instances of a font are clones drawing on one copy of the data.  That
// sharing is what makes every call after the first cheap, and it is why the
// data behind an instance must be left alone.
func TestGetSharesFontData(t *testing.T) {
	c := New([]string{"Helvetica"}, newReader().read)

	first := get(t, c, "Helvetica")
	second := get(t, c, "Helvetica")

	if first == second {
		t.Fatal("two callers got the same instance")
	}
	if first.Font != second.Font {
		t.Error("the font programs are not shared")
	}
	if first.Metrics != second.Metrics {
		t.Error("the metrics are not shared")
	}
	if first.Geometry != second.Geometry {
		t.Error("the geometry is not shared")
	}
}

// An instance belongs to the one document it is embedded into, so the
// character codes it allocates must not be taken from another instance.
func TestGetHandsOutIndependentInstances(t *testing.T) {
	c := New([]string{"Helvetica"}, newReader().read)

	first := get(t, c, "Helvetica")
	second := get(t, c, "Helvetica")

	gid := first.Layout(nil, 10, "A").Seq[0].GID
	if _, ok := first.Encode(gid, "A"); !ok {
		t.Fatal("no code was allocated")
	}
	if first.CodesRemaining() == second.CodesRemaining() {
		t.Error("a code allocated in one instance was taken from the other")
	}
}

// A key the cache does not name has nothing to share, so it is served afresh
// rather than turned away.
func TestGetServesUnnamedKey(t *testing.T) {
	r := newReader()
	c := New([]string{"Times-Roman"}, r.read)

	for range 2 {
		if _, err := c.Get("Helvetica"); err != nil {
			t.Fatal(err)
		}
	}

	if got := r.reads("Helvetica"); got != 2 {
		t.Errorf("two instances took %d reads, want 2", got)
	}
}

// A font which cannot be read has no instance to hand out, so every caller
// hears about the failure instead of receiving a half-built font.
func TestGetReportsReadFailure(t *testing.T) {
	c := New([]string{"NoSuchFont"}, newReader().read)

	for range 2 {
		F, err := c.Get("NoSuchFont")
		if err == nil {
			t.Fatal("a font which cannot be read was returned without an error")
		}
		if F != nil {
			t.Error("an instance was returned alongside the error")
		}
	}
}

// One cache serves the callers of a whole process, which may run at the same
// time: the font is still read once, and each caller still gets an instance of
// its own.
func TestGetConcurrent(t *testing.T) {
	r := newReader()
	c := New([]string{"Helvetica"}, r.read)

	const n = 8
	instances := make([]*type1.Instance, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			F, err := c.Get("Helvetica")
			if err != nil {
				t.Error(err)
				return
			}
			instances[i] = F
		})
	}
	wg.Wait()

	if got := r.reads("Helvetica"); got != 1 {
		t.Errorf("%d callers took %d reads, want 1", n, got)
	}
	seen := make(map[*type1.Instance]bool, n)
	for _, F := range instances {
		if F == nil {
			t.Fatal("a caller got no instance")
		}
		if seen[F] {
			t.Error("two callers got the same instance")
		}
		seen[F] = true
	}
}

// get returns an instance of the given font, failing the test if it cannot be
// made.
func get(t *testing.T, c *Cache[string], key string) *type1.Instance {
	t.Helper()

	F, err := c.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	return F
}

// reader builds font instances from the bundled data the way the packages
// using this one do, recording how often each font was read.
type reader struct {
	mu    sync.Mutex
	count map[string]int
}

func newReader() *reader {
	return &reader{count: make(map[string]int)}
}

func (r *reader) read(name string) (*type1.Instance, error) {
	r.mu.Lock()
	r.count[name]++
	r.mu.Unlock()

	psFont, metrics, err := Read(name)
	if err != nil {
		return nil, err
	}
	FixUpMetrics(metrics)
	return type1.New(psFont, metrics)
}

func (r *reader) reads(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count[name]
}
