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

package page

import (
	"errors"
	"io"
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestRoundTrip_CrossSegmentText verifies that a multi-stream page whose
// BT/ET pair spans the boundary between two PDF stream objects is parsed
// correctly by [Page.NewIter]: scanner state must flow across the stream
// boundary so segment 1's BT pairs with segment 2's ET and no operators
// are lost.
//
// Page.Encode requires each *content.Operators segment to be self-contained,
// so the two cross-segment streams are written directly to the file
// rather than going through Encode.  Decode wraps each stream as a
// *Source; iterating those Sources through [Page.NewIter] exercises
// the byte-stitching path.
func TestRoundTrip_CrossSegmentText(t *testing.T) {
	timesRoman := font.Must(standard.TimesRoman.New())

	w1, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w1.Alloc()

	// Write two content-stream objects: BT in stream 1, ET in stream 2.
	// PDF 32000-1 §7.8.2 says viewers parse the streams as if their
	// decoded bytes were concatenated.
	segRefs := make(pdf.Array, 2)
	for i, body := range []string{
		"BT\n/F1 12 Tf\n50 100 Td\n",
		"(Hello) Tj\nET\n",
	} {
		ref := w1.Alloc()
		stm, err := w1.OpenStream(ref, nil)
		if err != nil {
			t.Fatalf("open content stream %d: %v", i, err)
		}
		if _, err := stm.Write([]byte(body)); err != nil {
			t.Fatalf("write content stream %d: %v", i, err)
		}
		if err := stm.Close(); err != nil {
			t.Fatalf("close content stream %d: %v", i, err)
		}
		segRefs[i] = ref
	}

	// build a Page-typed page that uses our two streams as /Contents
	resources := &content.Resources{
		SingleUse: true,
		Font:      map[pdf.Name]font.Instance{"F1": timesRoman},
	}
	rm1 := pdf.NewResourceManager(w1)
	resObj, err := rm1.Embed(resources)
	if err != nil {
		t.Fatalf("embed resources: %v", err)
	}
	dict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    parentRef,
		"MediaBox":  &pdf.Rectangle{URx: 200, URy: 200},
		"Resources": resObj,
		"Contents":  segRefs,
	}
	if err := rm1.Close(); err != nil {
		t.Fatalf("close rm: %v", err)
	}
	w1.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")})
	if err := w1.Close(); err != nil {
		t.Fatalf("writer close: %v", err)
	}

	// Decode and verify the page has two segments and the combined
	// iteration yields the expected operator sequence.
	decoded, err := Decode(pdf.NewExtractor(w1), nil, dict, false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decoded.Contents) != 2 {
		t.Fatalf("expected 2 content segments, got %d", len(decoded.Contents))
	}
	var names []content.OpName
	it := decoded.NewIter()
	for name := range it.All() {
		names = append(names, name)
	}
	want := []content.OpName{
		content.OpTextBegin,
		content.OpTextSetFont,
		content.OpTextMoveOffset,
		content.OpTextShow,
		content.OpTextEnd,
	}
	if len(names) != len(want) {
		t.Fatalf("operator count: got %d, want %d (%v)", len(names), len(want), names)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("operator %d: got %q, want %q", i, names[i], n)
		}
	}
}

// TestRoundTrip_CrossSegmentArgsAndTokens verifies two things about
// segment boundaries:
//
//  1. operator arguments that sit on the stack at the end of segment 1
//     remain on the stack when segment 2 contributes the matching
//     operator (here, "3 4" on the stack from segment 1 must reach the
//     "l" operator in segment 2);
//
//  2. tokens do not silently join across the segment boundary (here,
//     segment 1's trailing "5" and segment 2's leading "0" must not
//     fuse into the single number "50").
//
// Both properties follow from concatenating the segment bytes with a
// single inter-segment whitespace separator into one byte stream that
// a single scanner consumes (PDF 32000-1 §7.8.2).
func TestRoundTrip_CrossSegmentArgsAndTokens(t *testing.T) {
	w1, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w1.Alloc()

	segRefs := make(pdf.Array, 2)
	for i, body := range []string{
		// ends with one complete number "3", one complete "4", and one
		// trailing "5" — the operator is in the next segment.
		"1 2 m\n3 4 5",
		// "0 l" reads as the integer 0 then the LineTo operator.
		// If the boundary fused "5" + "0" into "50", "l" would see
		// [3 4 50]; if the boundary dropped pending args, "l" would
		// see [0] alone.
		"0 l\n",
	} {
		ref := w1.Alloc()
		stm, err := w1.OpenStream(ref, nil)
		if err != nil {
			t.Fatalf("open content stream %d: %v", i, err)
		}
		if _, err := stm.Write([]byte(body)); err != nil {
			t.Fatalf("write content stream %d: %v", i, err)
		}
		if err := stm.Close(); err != nil {
			t.Fatalf("close content stream %d: %v", i, err)
		}
		segRefs[i] = ref
	}

	rm1 := pdf.NewResourceManager(w1)
	resObj, err := rm1.Embed(&content.Resources{SingleUse: true})
	if err != nil {
		t.Fatalf("embed resources: %v", err)
	}
	dict := pdf.Dict{
		"Type":      pdf.Name("Page"),
		"Parent":    parentRef,
		"MediaBox":  &pdf.Rectangle{URx: 200, URy: 200},
		"Resources": resObj,
		"Contents":  segRefs,
	}
	if err := rm1.Close(); err != nil {
		t.Fatal(err)
	}
	w1.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")})
	if err := w1.Close(); err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(pdf.NewExtractor(w1), nil, dict, false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	var ops []content.Operator
	it := decoded.NewIter()
	for name, args := range it.All() {
		ops = append(ops, content.Operator{Name: name, Args: slices.Clone(args)})
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iter: %v", err)
	}

	// expect:
	//   m(1, 2)
	//   l takes the last two args from the stack [3, 4, 5, 0]; the
	//   scanner forwards the full arg list and the line-to operator
	//   uses (5, 0).
	want := []content.Operator{
		{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(1), pdf.Integer(2)}},
		{Name: content.OpLineTo, Args: []pdf.Object{
			pdf.Integer(3), pdf.Integer(4), pdf.Integer(5), pdf.Integer(0),
		}},
	}
	if len(ops) != len(want) {
		t.Fatalf("op count: got %d, want %d (%v)", len(ops), len(want), ops)
	}
	for i := range want {
		if !ops[i].Equal(want[i]) {
			t.Errorf("op %d: got %+v, want %+v", i, ops[i], want[i])
		}
	}
}

// TestEncode_MixedContents_AllowsCrossStreamPairs verifies that a page
// with a mix of file-backed Source segments and in-memory *Operators
// segments can be encoded.  Each in-memory segment is required to be
// self-contained on its own (validated with fresh writer state); a
// Source neighbour does not contribute open state that the *Operators
// could rely on.
func TestEncode_MixedContents_AllowsCrossStreamPairs(t *testing.T) {
	// Round-trip a page through file to obtain a *Source.
	timesRoman := font.Must(standard.TimesRoman.New())
	w1, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef := w1.Alloc()
	donor := &Page{
		Parent:   parentRef,
		MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		Resources: &content.Resources{
			SingleUse: true,
			Font:      map[pdf.Name]font.Instance{"F1": timesRoman},
		},
		Contents: []Segment{
			&content.Operators{Ops: []content.Operator{
				{Name: content.OpPushGraphicsState},
				{Name: content.OpPopGraphicsState},
			}},
		},
	}
	rm1 := pdf.NewResourceManager(w1)
	dict, err := donor.Encode(rm1)
	if err != nil {
		t.Fatal(err)
	}
	rm1.Close()
	w1.Put(parentRef, pdf.Dict{"Type": pdf.Name("Pages")})
	w1.Close()
	decoded, err := Decode(pdf.NewExtractor(w1), nil, dict, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Contents) == 0 {
		t.Skip("no segments produced")
	}
	src := decoded.Contents[0] // *Source

	// Build a fresh page that mixes a self-contained in-memory segment
	// with the Source part (which has its own q/Q balance).  Each
	// segment validates independently; nothing in the in-memory part
	// depends on state from the Source.
	w2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	parentRef2 := w2.Alloc()
	mixed := &Page{
		Parent:   parentRef2,
		MediaBox: &pdf.Rectangle{URx: 100, URy: 100},
		Resources: &content.Resources{
			SingleUse: true,
		},
		Contents: []Segment{
			&content.Operators{Ops: []content.Operator{
				{Name: content.OpPushGraphicsState},
				{Name: content.OpPopGraphicsState},
			}},
			src,
		},
	}
	rm2 := pdf.NewResourceManager(w2)
	if _, err := mixed.Encode(rm2); err != nil {
		t.Fatalf("mixed page encode: %v", err)
	}
}

// brokenSegment is a [Segment] whose RawBytes always reports a
// pdf.MalformedFileError, modelling a corrupt-filter stream object.
// It is never embedded back to a file, only iterated, so [Embed] is a stub.
type brokenSegment struct{}

func (brokenSegment) RawBytes() (io.ReadCloser, error) {
	return nil, &pdf.MalformedFileError{Err: errors.New("synthetic corrupt segment")}
}

func (brokenSegment) Embed(*pdf.EmbedHelper) (pdf.Native, error) {
	return nil, errors.New("brokenSegment.Embed should not be called")
}

// TestIter_SkipsMalformedSegment confirms the permissive-reader policy:
// when one of a page's content segments fails to decode (e.g. corrupt
// FlateDecode), the page iterator must skip that segment and continue
// with the next one rather than abandoning the rest of the page.
func TestIter_SkipsMalformedSegment(t *testing.T) {
	p := &Page{
		MediaBox:  &pdf.Rectangle{URx: 100, URy: 100},
		Resources: &content.Resources{SingleUse: true},
		Contents: []Segment{
			&content.Operators{Ops: []content.Operator{
				{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(1), pdf.Integer(2)}},
			}},
			brokenSegment{},
			&content.Operators{Ops: []content.Operator{
				{Name: content.OpLineTo, Args: []pdf.Object{pdf.Integer(3), pdf.Integer(4)}},
			}},
		},
	}

	var ops []content.Operator
	it := p.NewIter()
	for name, args := range it.All() {
		ops = append(ops, content.Operator{Name: name, Args: slices.Clone(args)})
	}
	if err := it.Err(); err != nil {
		t.Fatalf("iter: %v", err)
	}
	// The middle (malformed) segment is dropped; segments 0 and 2 are
	// concatenated, so the LineTo from segment 2 continues the path.
	// (After Phase D the raw iter yields only what the scanner saw;
	// any closer for an open path is the consumer's responsibility.)

	want := []content.Operator{
		{Name: content.OpMoveTo, Args: []pdf.Object{pdf.Integer(1), pdf.Integer(2)}},
		{Name: content.OpLineTo, Args: []pdf.Object{pdf.Integer(3), pdf.Integer(4)}},
	}
	if len(ops) != len(want) {
		t.Fatalf("op count: got %d, want %d (%v)", len(ops), len(want), ops)
	}
	for i := range want {
		if !ops[i].Equal(want[i]) {
			t.Errorf("op %d: got %+v, want %+v", i, ops[i], want[i])
		}
	}
}
