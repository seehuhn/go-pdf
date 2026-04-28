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

package opaque

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestNewEmbedVerbatim checks that an Object built from an in-memory
// value is written to PDF without any reference translation.
func TestNewEmbedVerbatim(t *testing.T) {
	original := pdf.Dict{
		"Quality": pdf.Integer(5),
		"Codec":   pdf.Name("Acme"),
	}
	o := Direct(original)

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	native, err := rm.Embed(o)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if !pdf.Equal(native, original) {
		t.Errorf("verbatim Embed: got %v, want %v", native, original)
	}

	// Direct-built objects compare equal to themselves and to a fresh
	// Direct wrapping the same value.
	if !o.Equal(o) {
		t.Error("o.Equal(o) = false")
	}
	if !o.Equal(Direct(original)) {
		t.Error("o.Equal(Direct(same)) = false")
	}
	if o.Equal(Direct(pdf.Dict{"Quality": pdf.Integer(6)})) {
		t.Error("o.Equal(Direct(different)) = true")
	}

	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
}

// TestExtractRoundTripCrossFile checks that an Object built via Extract
// from a source PDF translates internal references when re-embedded
// into a different PDF.  The source file's reference becomes a fresh
// reference in the destination, with the referenced value copied
// through.
func TestExtractRoundTripCrossFile(t *testing.T) {
	// Build the source: an outer dict whose /Inner entry references an
	// indirect dict in the same file.
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	innerRef := src.Alloc()
	if err := src.Put(innerRef, pdf.Dict{"Y": pdf.Integer(99)}); err != nil {
		t.Fatalf("Put inner: %v", err)
	}
	outer := pdf.Dict{
		"Inner": innerRef,
		"Q":     pdf.Integer(5),
	}
	outerRef := src.Alloc()
	if err := src.Put(outerRef, outer); err != nil {
		t.Fatalf("Put outer: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = outerRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	srcX := pdf.NewExtractor(src)
	o := Extract(srcX, outerRef)

	// Embed into a different file.
	dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)
	native, err := rm.Embed(o)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	dstRef, ok := native.(pdf.Reference)
	if !ok {
		t.Fatalf("Embed returned %T, want pdf.Reference", native)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("dst.Close: %v", err)
	}

	// Verify that the destination's inner reference is a fresh dest
	// reference (not equal to the source's innerRef) and that its
	// payload survived the copy.
	dstX := pdf.NewExtractor(dst)
	dstOuter, err := dstX.GetDict(nil, dstRef)
	if err != nil {
		t.Fatalf("dst GetDict outer: %v", err)
	}
	dstInnerRef, ok := dstOuter["Inner"].(pdf.Reference)
	if !ok {
		t.Fatalf("dst /Inner is %T, want pdf.Reference", dstOuter["Inner"])
	}
	if dstInnerRef == innerRef {
		t.Error("dst /Inner reused the source-file reference; refs should be translated")
	}
	dstInner, err := dstX.GetDict(nil, dstInnerRef)
	if err != nil {
		t.Fatalf("dst GetDict inner: %v", err)
	}
	if dstInner["Y"] != pdf.Integer(99) {
		t.Errorf("inner.Y = %v, want 99", dstInner["Y"])
	}

	// Equal: the round-tripped Object should compare equal to the
	// original, with references resolved on both sides.
	roundTripped := Extract(dstX, dstRef)
	if !o.Equal(roundTripped) {
		t.Error("o.Equal(roundTripped) = false")
	}
}

// TestEmbedDedup verifies that a single *Object embedded multiple times
// against the same ResourceManager produces one indirect copy in the
// destination, with every Embed call returning the same Reference.
//
// This relies on (a) pdf.ResourceManager's Embedder cache, which keys
// by pointer identity, and (b) pdf.Copier's reference translation,
// which produces a Reference for indirect source values.
func TestEmbedDedup(t *testing.T) {
	// Source PDF with one indirect dict.
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	srcRef := src.Alloc()
	if err := src.Put(srcRef, pdf.Dict{"X": pdf.Integer(42)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = srcRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	srcX := pdf.NewExtractor(src)
	o := Extract(srcX, srcRef)

	// Two Embed calls against the same ResourceManager.
	dst, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)

	a, err := rm.Embed(o)
	if err != nil {
		t.Fatalf("first Embed: %v", err)
	}
	b, err := rm.Embed(o)
	if err != nil {
		t.Fatalf("second Embed: %v", err)
	}

	refA, okA := a.(pdf.Reference)
	refB, okB := b.(pdf.Reference)
	if !okA || !okB {
		t.Fatalf("both Embed results should be pdf.Reference, got %T and %T", a, b)
	}
	if refA != refB {
		t.Errorf("dedup failed: first Embed = %v, second Embed = %v", refA, refB)
	}

	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	if err := dst.Close(); err != nil {
		t.Fatalf("dst.Close: %v", err)
	}

	// Sanity: the destination's reference resolves to the expected dict.
	dstX := pdf.NewExtractor(dst)
	got, err := dstX.GetDict(nil, refA)
	if err != nil {
		t.Fatalf("dst GetDict: %v", err)
	}
	if got["X"] != pdf.Integer(42) {
		t.Errorf("dst dict.X = %v, want 42", got["X"])
	}
}

// TestObjectAs verifies that ObjectAs runs a typed extractor against
// the wrapped value when the Object has a source extractor, and
// returns an error when called on an Object built via New.
func TestObjectAs(t *testing.T) {
	// Build source PDF with a dict containing a known integer.
	src, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	if err := memfile.AddBlankPage(src); err != nil {
		t.Fatalf("AddBlankPage: %v", err)
	}
	srcRef := src.Alloc()
	if err := src.Put(srcRef, pdf.Dict{"N": pdf.Integer(7)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = srcRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	// extract reads a dict and returns its /N value.
	extract := func(x *pdf.Extractor, _ *pdf.CycleCheck, obj pdf.Object, _ bool) (int, error) {
		dict, err := x.GetDict(nil, obj)
		if err != nil {
			return 0, err
		}
		n, _ := dict["N"].(pdf.Integer)
		return int(n), nil
	}

	// success: file-backed Object.
	srcX := pdf.NewExtractor(src)
	fileBacked := Extract(srcX, srcRef)
	got, err := ObjectAs(fileBacked, nil, extract)
	if err != nil {
		t.Fatalf("ObjectAs(file-backed): %v", err)
	}
	if got != 7 {
		t.Errorf("ObjectAs(file-backed) = %d, want 7", got)
	}

	// error: in-memory Object has no source.
	inMemory := Direct(pdf.Dict{"N": pdf.Integer(99)})
	_, err = ObjectAs(inMemory, nil, extract)
	if err == nil {
		t.Error("ObjectAs(in-memory): expected error, got nil")
	}
}

func TestEqualNil(t *testing.T) {
	var nilObj *Object
	if !nilObj.Equal(nil) {
		t.Error("nil.Equal(nil) = false")
	}
	o := Direct(pdf.Integer(1))
	if o.Equal(nil) {
		t.Error("o.Equal(nil) = true")
	}
	if nilObj.Equal(o) {
		t.Error("nil.Equal(o) = true")
	}
}
