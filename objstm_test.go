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

package pdf

import (
	"bytes"
	"compress/zlib"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"seehuhn.de/go/pdf/internal/limits"
)

// writeObjStmDoc writes a document holding n small objects inside one
// compressed object stream, plus a page tree so the file is valid.
func writeObjStmDoc(t *testing.T, path string, n int) {
	t.Helper()
	w, err := Create(path, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	refs := make([]Reference, n)
	objs := make([]Object, n)
	for i := range refs {
		refs[i] = w.Alloc()
		objs[i] = Dict{"i": Integer(i)}
	}
	if err := w.WriteCompressed(refs, objs...); err != nil {
		t.Fatal(err)
	}
	pagesRef := w.Alloc()
	if err := w.Put(pagesRef, Dict{"Type": Name("Pages"), "Count": Integer(0)}); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestGetFromObjStmCached checks that all objects of an object stream
// resolve correctly, repeatedly, from the decoded-stream cache.
func TestGetFromObjStmCached(t *testing.T) {
	path := t.TempDir() + "/objstm.pdf"
	const n = 50
	writeObjStmDoc(t, path, n)

	r, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	for pass := range 2 {
		for i := range n {
			// object numbers start at 1; the page tree occupies the last one
			num := uint32(i + 1)
			obj, err := r.Get(NewReference(num, 0), true)
			if err != nil {
				t.Fatalf("pass %d: object %d: %v", pass, num, err)
			}
			dict, ok := obj.(Dict)
			if !ok {
				t.Fatalf("pass %d: object %d: got %T, want Dict", pass, num, obj)
			}
			if iv := dict["i"]; iv != Integer(num-1) {
				t.Errorf("pass %d: object %d carries wrong payload %v", pass, num, iv)
			}
		}
	}

	if len(r.objstms.entries) == 0 {
		t.Error("object stream was not cached")
	}
}

// TestGetFromObjStmConcurrent exercises concurrent resolution of objects
// from one cached object stream (run with -race).
func TestGetFromObjStmConcurrent(t *testing.T) {
	path := t.TempDir() + "/objstm.pdf"
	const n = 100
	writeObjStmDoc(t, path, n)

	r, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			for i := range n {
				ref := NewReference(uint32(i+1), 0)
				if _, err := r.Get(ref, true); err != nil {
					t.Errorf("worker %d: object %d: %v", worker, i+1, err)
					return
				}
			}
		})
	}
	wg.Wait()
}

// TestObjStmOutOfOrderIndex resolves objects whose index entries are not in
// increasing offset order.  The cached implementation reads each object from
// its absolute offset, so declaration order does not matter.
func TestObjStmOutOfOrderIndex(t *testing.T) {
	b6 := "<< /K (six) >>"
	b7 := "<< /K (seven) >>"
	index := fmt.Sprintf("7 %d\n6 0\n", len(b6)+1)
	first := len(index)
	body := []byte(index + b6 + "\n" + b7)

	// objects: 1 catalog, 2 pages, 4 ObjStm; 6 and 7 live inside object 4,
	// with the index entry of 7 declared before that of 6
	offsets := map[int]int{}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	emit := func(n int, body string) {
		offsets[n] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	emit(1, "<< /Type /Catalog /Pages 2 0 R >>")
	emit(2, "<< /Type /Pages /Kids [] /Count 0 >>")
	streamDict := fmt.Sprintf("<< /Type /ObjStm /N 2 /First %d /Length %d >>\nstream\n", first, len(body))
	emit(4, streamDict+string(body)+"\nendstream")

	// xref stream: /W [1 4 2]; objects 6 and 7 are type-2 entries pointing
	// into object stream 4
	xrefPos := buf.Len()
	rows := []struct {
		typ    uint64
		f2, f3 uint64
	}{
		{0, 0, 0},
		{1, uint64(offsets[1]), 0},
		{1, uint64(offsets[2]), 0},
		{0, 0, 0},
		{1, uint64(offsets[4]), 0},
		{0, 0, 0},
		{2, 4, 0},
		{2, 4, 1},
	}
	var data bytes.Buffer
	for _, row := range rows {
		putBE(&data, row.typ, 1)
		putBE(&data, row.f2, 4)
		putBE(&data, row.f3, 2)
	}
	xrefDict := fmt.Sprintf("<< /Type /XRef /Size %d /W [1 4 2] /Root 1 0 R /Length %d >>\nstream\n",
		len(rows), data.Len())
	fmt.Fprintf(&buf, "5 0 obj\n%s", xrefDict)
	buf.Write(data.Bytes())
	buf.WriteString("\nendstream\nendobj\n")
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefPos)

	path := t.TempDir() + "/outoforder.pdf"
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	want := map[uint32]string{6: "six", 7: "seven"}
	for _, num := range []uint32{7, 6, 7, 6} {
		obj, err := r.Get(NewReference(num, 0), true)
		if err != nil {
			t.Fatalf("object %d: %v", num, err)
		}
		dict, ok := obj.(Dict)
		if !ok {
			t.Fatalf("object %d: got %T, want Dict", num, obj)
		}
		if got := string(dict["K"].(String)); got != want[num] {
			t.Errorf("object %d: got %q, want %q", num, got, want[num])
		}
	}
}

// zlibBytes compresses data with zlib (FlateDecode).
func zlibBytes(data []byte) []byte {
	var buf bytes.Buffer
	z := zlib.NewWriter(&buf)
	z.Write(data)
	z.Close()
	return buf.Bytes()
}

// putBE writes v as a big-endian fixed-width integer.
func putBE(buf *bytes.Buffer, v uint64, width int) {
	for shift := 8 * (width - 1); shift >= 0; shift -= 8 {
		buf.WriteByte(byte(v >> shift))
	}
}

// TestObjstmCacheEviction checks the FIFO eviction and the refusal to retain
// oversized streams.
func TestObjstmCacheEviction(t *testing.T) {
	c := newObjstmCache()

	big := bytes.Repeat([]byte("a"), maxObjstmCacheBytes+1)
	c.put(NewReference(1, 0), big, nil)
	if len(c.entries) != 0 {
		t.Error("oversized stream was retained")
	}

	half := bytes.Repeat([]byte("a"), maxObjstmCacheBytes/2+1)
	c.put(NewReference(2, 0), half, nil)
	c.put(NewReference(3, 0), half, nil)
	if _, ok := c.entries[NewReference(2, 0)]; ok {
		t.Error("older entry was not evicted")
	}
	if _, ok := c.entries[NewReference(3, 0)]; !ok {
		t.Error("newer entry missing after eviction")
	}
	if c.used > maxObjstmCacheBytes {
		t.Errorf("cache holds %d bytes, bound is %d", c.used, maxObjstmCacheBytes)
	}
}

// TestGetFromObjStmTooLarge checks that an object stream decoding beyond
// limits.MaxObjStmBytes is rejected as malformed rather than slurped into
// memory.  The stream is ~66 KiB compressed but inflates past the 64 MiB cap.
func TestGetFromObjStmTooLarge(t *testing.T) {
	// the pre-compression payload must itself exceed the 64 MiB cap;
	// it compresses to a few hundred bytes
	body := bytes.Repeat([]byte("q"), limits.MaxObjStmBytes+4096)
	compressed := zlibBytes(body)

	offsets := map[int]int{}
	var buf bytes.Buffer
	buf.WriteString("%PDF-1.7\n")
	emit := func(n int, body string) {
		offsets[n] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", n, body)
	}
	emit(1, "<< /Type /Catalog /Pages 2 0 R >>")
	emit(2, "<< /Type /Pages /Kids [] /Count 0 >>")
	streamDict := fmt.Sprintf("<< /Type /ObjStm /N 1 /First 4 /Filter /FlateDecode /Length %d >>\nstream\n",
		len(compressed))
	emit(4, streamDict+string(compressed)+"\nendstream")

	xrefPos := buf.Len()
	rows := []struct {
		typ    uint64
		f2, f3 uint64
	}{
		{0, 0, 0},
		{1, uint64(offsets[1]), 0},
		{1, uint64(offsets[2]), 0},
		{0, 0, 0},
		{1, uint64(offsets[4]), 0},
		{0, 0, 0},
		{2, 4, 0},
	}
	var data bytes.Buffer
	for _, row := range rows {
		putBE(&data, row.typ, 1)
		putBE(&data, row.f2, 4)
		putBE(&data, row.f3, 2)
	}
	xrefDict := fmt.Sprintf("<< /Type /XRef /Size %d /W [1 4 2] /Root 1 0 R /Length %d >>\nstream\n",
		len(rows), data.Len())
	fmt.Fprintf(&buf, "5 0 obj\n%s", xrefDict)
	buf.Write(data.Bytes())
	buf.WriteString("\nendstream\nendobj\n")
	fmt.Fprintf(&buf, "startxref\n%d\n%%%%EOF\n", xrefPos)

	path := t.TempDir() + "/toobig.pdf"
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, err = r.Get(NewReference(6, 0), true)
	if err == nil {
		t.Fatal("oversized object stream resolved without error")
	}
	var mfe *MalformedFileError
	if !errors.As(err, &mfe) {
		t.Errorf("got %T, want *MalformedFileError", err)
	}
}

// BenchmarkGetFromObjStm measures the cost of resolving every object of a
// compressed object stream; it guards against reintroducing the per-lookup
// full re-decode.
func BenchmarkGetFromObjStm(b *testing.B) {
	dir := b.TempDir()
	path := dir + "/objstm.pdf"
	const n = 4000
	w, err := Create(path, V1_7, nil)
	if err != nil {
		b.Fatal(err)
	}
	refs := make([]Reference, n)
	objs := make([]Object, n)
	for i := range refs {
		refs[i] = w.Alloc()
		objs[i] = Dict{"i": Integer(i), "s": String(strings.Repeat("x", 2000))}
	}
	if err := w.WriteCompressed(refs, objs...); err != nil {
		b.Fatal(err)
	}
	pagesRef := w.Alloc()
	if err := w.Put(pagesRef, Dict{"Type": Name("Pages"), "Count": Integer(0)}); err != nil {
		b.Fatal(err)
	}
	w.GetMeta().Catalog.Pages = pagesRef
	if err := w.Close(); err != nil {
		b.Fatal(err)
	}

	r, err := Open(path, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer r.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for num := uint32(1); num <= uint32(n); num++ {
			if _, err := r.Get(NewReference(num, 0), true); err != nil {
				b.Fatal(err)
			}
		}
	}
}
