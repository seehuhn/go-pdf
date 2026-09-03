// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFindXref(t *testing.T) {
	in := "%PDF-1.7\nhello\nstartxref\n9\n%%EOF"
	r := &Reader{
		r: strings.NewReader(in),
	}
	start, err := r.findXRef(int64(len(in)))
	if err != nil {
		t.Error(err)
	}
	if start != 9 {
		t.Errorf("wrong xref start, expected 9 but got %d", start)
	}
}

func TestDecodeInt(t *testing.T) {
	cases := []struct {
		name    string
		buf     []byte
		want    int64
		wantErr bool
	}{
		{"empty", nil, 0, false},
		{"one byte", []byte{0x42}, 0x42, false},
		{"three bytes", []byte{0x01, 0x02, 0x03}, 0x010203, false},
		{"max int64", []byte{0x7f, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 1<<63 - 1, false},
		// W=[*,8,*] with the high bit set used to wrap to a negative
		// signed int64; now it must be rejected outright.
		{"high bit overflow", []byte{0x80, 0, 0, 0, 0, 0, 0, 0}, 0, true},
		{"all ones overflow", []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeInt(tc.buf)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestCheckXRefStreamDict(t *testing.T) {
	cases := []struct {
		name    string
		dict    Dict
		wantErr bool
	}{
		{
			name: "valid",
			dict: Dict{
				"Size": Integer(10),
				"W":    Array{Integer(1), Integer(2), Integer(1)},
			},
		},
		{
			name: "Size above cap",
			dict: Dict{
				"Size": Integer(maxXRefSize + 1),
				"W":    Array{Integer(1), Integer(2), Integer(1)},
			},
			wantErr: true,
		},
		{
			name: "Index subsection extends past Size",
			dict: Dict{
				"Size":  Integer(10),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(0), Integer(11)},
			},
			wantErr: true,
		},
		{
			name: "Index subsection start past Size",
			dict: Dict{
				"Size":  Integer(10),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(11), Integer(1)},
			},
			wantErr: true,
		},
		{
			name: "Index covering exactly Size",
			dict: Dict{
				"Size":  Integer(10),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(0), Integer(10)},
			},
		},
		{
			name: "multiple Index subsections within Size",
			dict: Dict{
				"Size":  Integer(100),
				"W":     Array{Integer(1), Integer(2), Integer(1)},
				"Index": Array{Integer(0), Integer(10), Integer(50), Integer(20)},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := checkXRefStreamDict(tc.dict, 1<<20)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

// TestXRefStreamEntryCountCap verifies that the number of declared entries is
// bounded in proportion to the stream's raw size: the same /Size is rejected
// for a tiny stream but accepted once the raw length is large enough.
func TestXRefStreamEntryCountCap(t *testing.T) {
	dict := Dict{
		"Size": Integer(100000),
		"W":    Array{Integer(1), Integer(2), Integer(1)},
	}

	if _, _, err := checkXRefStreamDict(dict, 10); err == nil {
		t.Error("100000 entries from a 10-byte stream should be rejected")
	}
	if _, _, err := checkXRefStreamDict(dict, 100000); err != nil {
		t.Errorf("100000 entries from a 100000-byte stream should be accepted: %v", err)
	}
}

// TestXRefStreamObjectNumberOverflow verifies that compressed-object
// (type-2) xref-stream entries whose field-2 object number exceeds
// MaxUint32 are skipped rather than silently truncated.
func TestXRefStreamObjectNumberOverflow(t *testing.T) {
	xref := map[uint32]*xRefEntry{}
	// W = [1 8 1]: type byte, 8-byte object number, 1-byte index
	w := []int{1, 8, 1}
	ss := []*xRefSubSection{{Start: 5, Size: 1}}

	// type 2, object number 0x1_0000_0000 (> MaxUint32), index 0
	data := []byte{2, 0, 0, 0, 1, 0, 0, 0, 0, 0}
	if err := decodeXRefStream(xref, bytes.NewReader(data), w, ss); err != nil {
		t.Fatalf("decodeXRefStream: %v", err)
	}
	if _, ok := xref[5]; ok {
		t.Errorf("entry with object number > MaxUint32 should have been skipped")
	}
}

// TestXRefStreamGenerationOverflow verifies that xref-stream entries whose
// field-3 (generation) exceeds 65535 are silently dropped rather than
// truncated.  PDF 32000-2 §7.5.4 caps generations at 65535, and §7.6.3.2
// reserves only 2 bytes for the generation in per-object key derivation;
// preserving an out-of-range entry would collide with another object's
// encryption key.
func TestXRefStreamGenerationOverflow(t *testing.T) {
	xref := map[uint32]*xRefEntry{}
	// W = [1 1 8]: type byte, 1-byte byte-offset, 8-byte generation
	w := []int{1, 1, 8}
	ss := []*xRefSubSection{{Start: 5, Size: 1}}

	// type 1 (in-use), byte offset 0, generation 0x1_0000 (> 65535).
	// Generation is 8 bytes big-endian: 0x00 0x00 0x00 0x00 0x00 0x01 0x00 0x00
	data := []byte{1, 0, 0, 0, 0, 0, 0, 1, 0, 0}
	if err := decodeXRefStream(xref, bytes.NewReader(data), w, ss); err != nil {
		t.Fatalf("decodeXRefStream: %v", err)
	}
	if _, ok := xref[5]; ok {
		t.Errorf("entry with generation > 65535 should have been skipped")
	}
}

// TestAllocOverflowPanics verifies that Writer.Alloc panics rather than
// minting object numbers >= 2^24, which would collide on encryption-key
// derivation (PDF 32000-2 §7.6.3.2).
func TestAllocOverflowPanics(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.nextRef = maxXRefSize

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Alloc at nextRef=maxXRefSize did not panic")
		}
	}()
	w.Alloc()
}

// TestNewReferenceOverflowPanics verifies that NewReference panics for
// object numbers >= 2^24, since the encryption-key derivation only uses
// the low 3 bytes (PDF 32000-2 §7.6.3.2).
func TestNewReferenceOverflowPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewReference(maxXRefSize, 0) did not panic")
		}
	}()
	NewReference(maxXRefSize, 0)
}

func TestLastOccurence(t *testing.T) {
	buf := make([]byte, 2048)
	pat := "ABC"
	copy(buf[1023:], pat)

	r := &Reader{
		r: bytes.NewReader(buf),
	}
	pos, err := r.lastOccurence(pat, int64(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	if pos != 1023 {
		t.Errorf("found wrong position: expected 1023, got %d", pos)
	}
}

// lastStartXRef returns the value following the last startxref keyword.
func lastStartXRef(t *testing.T, data []byte) int64 {
	t.Helper()
	i := bytes.LastIndex(data, []byte("startxref"))
	if i < 0 {
		t.Fatal("no startxref keyword")
	}
	// Skip whitespace after "startxref"
	j := i + 9
	for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
		j++
	}
	var pos int64
	if _, err := fmt.Sscanf(string(data[j:]), "%d", &pos); err != nil {
		t.Fatal(err)
	}
	return pos
}

var rootRefPat = regexp.MustCompile(`/Root\s+(\d+)\s+0\s+R`)

// rootRef returns the catalog reference named in the last trailer of data.
func rootRef(t *testing.T, data []byte) Reference {
	t.Helper()
	m := rootRefPat.FindAllSubmatch(data, -1)
	if m == nil {
		t.Fatal("no /Root entry")
	}
	n, err := strconv.Atoi(string(m[len(m)-1][1]))
	if err != nil {
		t.Fatal(err)
	}
	return NewReference(uint32(n), 0)
}

var sizePat = regexp.MustCompile(`/Size\s+(\d+)`)

// trailerSize returns the /Size named in the last trailer of data.
func trailerSize(t *testing.T, data []byte) int {
	t.Helper()
	m := sizePat.FindAllSubmatch(data, -1)
	if m == nil {
		t.Fatal("no /Size entry")
	}
	n, err := strconv.Atoi(string(m[len(m)-1][1]))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// appendEmptySection appends an update section with no entries whose
// trailer holds extra plus /Prev, and returns the new file contents.
func appendEmptySection(t *testing.T, data []byte, extra string) []byte {
	t.Helper()
	prev := lastStartXRef(t, data)
	buf := bytes.NewBuffer(bytes.Clone(data))
	pos := buf.Len()
	fmt.Fprintf(buf, "xref\n0 0\ntrailer\n<< /Prev %d %s >>\nstartxref\n%d\n%%%%EOF\n",
		prev, extra, pos)
	return buf.Bytes()
}

// writeBaseFile writes a one-page PDF 1.4 file with the given options and
// an Info title, and returns its bytes.
func writeBaseFile(t *testing.T, opt *WriterOptions, title string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_4, opt)
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}
	w.GetMeta().Info.Title = TextString(title)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadXRefRecordsNewestSection(t *testing.T) {
	data := writeBaseFile(t, nil, "")
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.startXRef != lastStartXRef(t, data) {
		t.Errorf("startXRef = %d, want %d", r.startXRef, lastStartXRef(t, data))
	}
	if r.trailerSize != int64(trailerSize(t, data)) {
		t.Errorf("trailerSize = %d, want %d", r.trailerSize, trailerSize(t, data))
	}
}

func TestReadXRefMergesRootFromOlderTrailer(t *testing.T) {
	data := writeBaseFile(t, nil, "")
	data = appendEmptySection(t, data, fmt.Sprintf("/Size %d", trailerSize(t, data)))
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if r.meta.Catalog.Pages == 0 {
		t.Error("catalog not found via older trailer")
	}
}

func TestReadXRefDoesNotMergeInfo(t *testing.T) {
	data := writeBaseFile(t, nil, "hello")
	extra := fmt.Sprintf("/Size %d /Root %d 0 R", trailerSize(t, data), rootRef(t, data).Number())
	data = appendEmptySection(t, data, extra)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.meta.Info != nil {
		t.Errorf("info merged from older trailer: %v", r.meta.Info)
	}
}

func TestReadXRefNewestTrailerWins(t *testing.T) {
	id0 := bytes.Repeat([]byte{0xAA}, 16)
	data := writeBaseFile(t, &WriterOptions{ID: [][]byte{id0}}, "")
	newID := bytes.Repeat([]byte{0xBB}, 16)
	extra := fmt.Sprintf("/Size %d /Root %d 0 R /ID [<%x> <%x>]",
		trailerSize(t, data), rootRef(t, data).Number(), newID, newID)
	data = appendEmptySection(t, data, extra)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.meta.ID) != 2 || !bytes.Equal(r.meta.ID[0], newID) {
		t.Errorf("ID = %x, want newest trailer's value", r.meta.ID)
	}
}

// appendUpdateSection appends an update section holding the given objects
// (object number to body), free entries for freed, a cross-reference table
// covering exactly those numbers, and a trailer holding extra plus /Prev.
func appendUpdateSection(t *testing.T, data []byte, objects map[uint32]string, freed []uint32, extra string) []byte {
	t.Helper()
	prev := lastStartXRef(t, data)
	buf := bytes.NewBuffer(bytes.Clone(data))
	type entry struct {
		pos  int64
		free bool
	}
	entries := map[uint32]entry{}
	for _, n := range freed {
		entries[n] = entry{free: true}
	}
	for _, n := range slices.Sorted(maps.Keys(objects)) {
		entries[n] = entry{pos: int64(buf.Len())}
		fmt.Fprintf(buf, "%d 0 obj\n%s\nendobj\n", n, objects[n])
	}
	xrefPos := buf.Len()
	buf.WriteString("xref\n")
	for _, n := range slices.Sorted(maps.Keys(entries)) {
		e := entries[n]
		if e.free {
			fmt.Fprintf(buf, "%d 1\n0000000000 00001 f\r\n", n)
		} else {
			fmt.Fprintf(buf, "%d 1\n%010d 00000 n\r\n", n, e.pos)
		}
	}
	fmt.Fprintf(buf, "trailer\n<< /Prev %d %s >>\nstartxref\n%d\n%%%%EOF\n",
		prev, extra, xrefPos)
	return buf.Bytes()
}

// writeBaseFileWith writes a one-page PDF 1.4 file containing one extra
// object with the given body, and returns the bytes and that object's
// reference.
func writeBaseFileWith(t *testing.T, body Object) ([]byte, Reference) {
	t.Helper()
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_4, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}
	ref := w.Alloc()
	if err := w.Put(ref, body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), ref
}

func TestReadXRefChainNewestObjectWins(t *testing.T) {
	data, ref := writeBaseFileWith(t, Name("old"))
	extra := fmt.Sprintf("/Size %d /Root %d 0 R", trailerSize(t, data), rootRef(t, data).Number())
	data = appendUpdateSection(t, data, map[uint32]string{ref.Number(): "/new"}, nil, extra)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := r.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != Name("new") {
		t.Errorf("got %v, want /new", obj)
	}
}

func TestReadXRefChainFreeEntryDeletes(t *testing.T) {
	data, ref := writeBaseFileWith(t, Name("old"))
	extra := fmt.Sprintf("/Size %d /Root %d 0 R", trailerSize(t, data), rootRef(t, data).Number())
	data = appendUpdateSection(t, data, nil, []uint32{ref.Number()}, extra)
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := r.Get(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != nil {
		t.Errorf("got %v, want null for freed object", obj)
	}
}

func TestReadXRefHybridXRefStm(t *testing.T) {
	data := writeBaseFile(t, nil, "")
	prev := lastStartXRef(t, data)
	size := uint32(trailerSize(t, data))
	hidden := size     // only listed in the cross-reference stream
	stmNum := size + 1 // the cross-reference stream object

	buf := bytes.NewBuffer(bytes.Clone(data))
	hiddenPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n/hidden\nendobj\n", hidden)

	// one type-1 entry for hidden: W [1 4 2]
	entry := []byte{1,
		byte(hiddenPos >> 24), byte(hiddenPos >> 16), byte(hiddenPos >> 8), byte(hiddenPos),
		0, 0}
	stmPos := buf.Len()
	fmt.Fprintf(buf, "%d 0 obj\n<< /Type /XRef /Size %d /W [1 4 2] /Index [%d 1] /Length %d >>\nstream\n",
		stmNum, size+2, hidden, len(entry))
	buf.Write(entry)
	buf.WriteString("\nendstream\nendobj\n")

	xrefPos := buf.Len()
	fmt.Fprintf(buf, "xref\n0 0\ntrailer\n<< /Size %d /Root %d 0 R /Prev %d /XRefStm %d >>\nstartxref\n%d\n%%%%EOF\n",
		size+2, rootRef(t, data).Number(), prev, stmPos, xrefPos)
	data = buf.Bytes()

	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := r.Get(NewReference(hidden, 0), true)
	if err != nil {
		t.Fatal(err)
	}
	if obj != Name("hidden") {
		t.Errorf("got %v, want /hidden via /XRefStm", obj)
	}
	if r.meta.Catalog.Pages == 0 {
		t.Error("catalog lost when following /Prev after /XRefStm")
	}
}

func TestXRefRunsOf(t *testing.T) {
	xref := map[uint32]*xRefEntry{}
	for _, n := range []uint32{1, 2, 3, 7, 9, 10} {
		xref[n] = &xRefEntry{}
	}
	got := xRefRunsOf(xref, 11)
	want := []xRefRun{{1, 3}, {7, 1}, {9, 3}}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(xRefRun{})); diff != "" {
		t.Errorf("runs differ (-want +got):\n%s", diff)
	}
	if got := xRefRunsOf(nil); len(got) != 0 {
		t.Errorf("empty map gave runs %v", got)
	}
}

func TestXRefStreamListsItself(t *testing.T) {
	buf := &bytes.Buffer{}
	w, err := NewWriter(buf, V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := addPage(w); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	r, err := NewReader(bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		t.Fatal(err)
	}
	stmNum := w.nextRef - 1 // the last allocated object is the xref stream
	entry := r.xref[stmNum]
	if entry.IsFree() {
		t.Fatalf("xref stream object %d has no in-use entry", stmNum)
	}
	if entry.Pos != lastStartXRef(t, data) {
		t.Errorf("xref stream entry points to %d, want %d", entry.Pos, lastStartXRef(t, data))
	}
}
