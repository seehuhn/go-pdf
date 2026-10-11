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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
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

	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	dstOuter, err := pdf.CursorAt(dstX, nil).Dict(dstRef)
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
	dstInner, err := pdf.CursorAt(dstX, nil).Dict(dstInnerRef)
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
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	got, err := pdf.CursorAt(dstX, nil).Dict(refA)
	if err != nil {
		t.Fatalf("dst GetDict: %v", err)
	}
	if got["X"] != pdf.Integer(42) {
		t.Errorf("dst dict.X = %v, want 42", got["X"])
	}
}

// TestObjectAs verifies that Object.As runs a typed extractor against
// the wrapped value when the Object has a source extractor, and
// returns an error when called on an Object built via New.
func TestObjectAs(t *testing.T) {
	// Build source PDF with a dict containing a known integer.
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	srcRef := src.Alloc()
	if err := src.Put(srcRef, pdf.Dict{"N": pdf.Integer(7)}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	src.GetMeta().Trailer["Quir:E"] = srcRef
	if err := src.Close(); err != nil {
		t.Fatalf("src.Close: %v", err)
	}

	// extract reads a dict and returns its /N value.
	extract := func(c pdf.Cursor, obj pdf.Object, _ bool) (int, error) {
		dict, err := c.Dict(obj)
		if err != nil {
			return 0, err
		}
		n, _ := dict["N"].(pdf.Integer)
		return int(n), nil
	}

	// success: file-backed Object.
	srcX := pdf.NewExtractor(src)
	fileBacked := Extract(srcX, srcRef)
	got, err := fileBacked.As(nil, extract)
	if err != nil {
		t.Fatalf("As(file-backed): %v", err)
	}
	if got != 7 {
		t.Errorf("As(file-backed) = %d, want 7", got)
	}

	// error: in-memory Object has no source.
	inMemory := Direct(pdf.Dict{"N": pdf.Integer(99)})
	_, err = inMemory.As(nil, extract)
	if err == nil {
		t.Error("As(in-memory): expected error, got nil")
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

// TestEqualCycle checks that values containing reference cycles through
// dictionaries compare by their structure: a cycle equals itself, and equals
// a differently shaped cycle which unfolds to the same infinite value.
func TestEqualCycle(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	aRef, bRef, cRef, dRef := src.Alloc(), src.Alloc(), src.Alloc(), src.Alloc()
	puts := map[pdf.Reference]pdf.Object{
		aRef: pdf.Dict{"K": bRef}, // a and b point at each other
		bRef: pdf.Dict{"K": aRef},
		cRef: pdf.Dict{"K": cRef},                     // c points at itself
		dRef: pdf.Dict{"K": cRef, "X": pdf.Name("x")}, // d differs from c
	}
	for ref, obj := range puts {
		if err := src.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}
	srcX := pdf.NewExtractor(src)

	a := Extract(srcX, aRef)
	if !a.Equal(a) {
		t.Error("a.Equal(a) = false")
	}
	if !a.Equal(Extract(srcX, cRef)) {
		t.Error("a.Equal(c) = false")
	}
	if a.Equal(Extract(srcX, dRef)) {
		t.Error("a.Equal(d) = true")
	}
	if a.Equal(Direct(pdf.Dict{"K": pdf.Name("anything")})) {
		t.Error("a.Equal(direct) = true")
	}
}

// TestEqualMisalignedCycles checks two cyclic values whose references never
// line up: one side has a reference at every second level of nesting, the
// other only at the odd levels.  Both unfold to the same infinitely nested
// array, and the comparison must still terminate.
func TestEqualMisalignedCycles(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	aRef, bRef := src.Alloc(), src.Alloc()
	if err := src.Put(aRef, pdf.Array{pdf.Array{aRef}}); err != nil {
		t.Fatal(err)
	}
	if err := src.Put(bRef, pdf.Array{pdf.Array{bRef}}); err != nil {
		t.Fatal(err)
	}
	srcX := pdf.NewExtractor(src)

	a := Extract(srcX, aRef)
	b := Extract(srcX, pdf.Array{bRef})
	if !a.Equal(b) {
		t.Error("a.Equal(b) = false")
	}
}

// TestEqualCoprimeCycles compares two reference cycles whose lengths are
// coprime, so that a comparison which walks pairs of positions until a
// pair repeats would need the product of the two lengths in steps.
func TestEqualCoprimeCycles(t *testing.T) {
	const p, q = 10007, 10009

	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	cycle := func(n int) pdf.Reference {
		refs := make([]pdf.Reference, n)
		for i := range refs {
			refs[i] = src.Alloc()
		}
		for i, ref := range refs {
			if err := src.Put(ref, pdf.Array{refs[(i+1)%n]}); err != nil {
				t.Fatal(err)
			}
		}
		return refs[0]
	}
	aRef := cycle(p)
	bRef := cycle(q)
	srcX := pdf.NewExtractor(src)

	a := Extract(srcX, aRef)
	b := Extract(srcX, bRef)
	if !a.Equal(b) {
		t.Error("a.Equal(b) = false")
	}
}

// TestEqualReferenceLoop checks that a chain of references which loops back
// on itself counts as null, which is what [pdf.Copier] writes for it.
func TestEqualReferenceLoop(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	aRef, bRef := src.Alloc(), src.Alloc()
	if err := src.Put(aRef, bRef); err != nil {
		t.Fatal(err)
	}
	if err := src.Put(bRef, aRef); err != nil {
		t.Fatal(err)
	}
	srcX := pdf.NewExtractor(src)

	o := Extract(srcX, pdf.Dict{"K": aRef, "N": pdf.Integer(1)})
	if !o.Equal(o) {
		t.Error("o.Equal(o) = false")
	}
	if !o.Equal(Direct(pdf.Dict{"N": pdf.Integer(1)})) {
		t.Error("reference loop does not compare as null")
	}
}

// TestEqualDeepChain checks that a long chain of indirect dictionaries does
// not exhaust the Go stack, and that it is compared all the way down.
func TestEqualDeepChain(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	const n = 10000
	build := func(leaf pdf.Name) pdf.Reference {
		refs := make([]pdf.Reference, n)
		for i := range refs {
			refs[i] = src.Alloc()
		}
		for i := range n - 1 {
			if err := src.Put(refs[i], pdf.Dict{"K": refs[i+1]}); err != nil {
				t.Fatal(err)
			}
		}
		if err := src.Put(refs[n-1], pdf.Dict{"K": leaf}); err != nil {
			t.Fatal(err)
		}
		return refs[0]
	}
	first := build("leaf")
	same := build("leaf")
	different := build("other")
	srcX := pdf.NewExtractor(src)

	deep := Extract(srcX, first)
	if !deep.Equal(Extract(srcX, same)) {
		t.Error("deep.Equal(same) = false")
	}
	if deep.Equal(Extract(srcX, different)) {
		t.Error("deep.Equal(different) = true")
	}
}

// TestEqualShared checks that an object reached along many paths is compared
// only once: each level of the chain refers to the next level twice, so the
// number of paths doubles at every level.
func TestEqualShared(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	const n = 100
	refs := make([]pdf.Reference, n)
	for i := range refs {
		refs[i] = src.Alloc()
	}
	for i := range n - 1 {
		if err := src.Put(refs[i], pdf.Array{refs[i+1], refs[i+1]}); err != nil {
			t.Fatal(err)
		}
	}
	if err := src.Put(refs[n-1], pdf.Integer(0)); err != nil {
		t.Fatal(err)
	}
	srcX := pdf.NewExtractor(src)

	o := Extract(srcX, refs[0])
	if !o.Equal(o) {
		t.Error("o.Equal(o) = false")
	}
}

// unreadableGetter wraps a [pdf.Getter] and makes the object bad unreadable,
// as if its bytes in the file were damaged.
type unreadableGetter struct {
	pdf.Getter
	bad pdf.Reference
}

func (g unreadableGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.bad {
		return nil, &pdf.MalformedFileError{Err: errors.New("damaged object")}
	}
	return g.Getter.Get(ref, canObjStm)
}

// TestEqualUnreadable checks that a reference to an unreadable object
// compares as null (PDF 2.0, 7.3.10), so that an Object agrees with itself
// and with the copy [Object.Embed] writes, where the Copier puts null in
// place of the unreadable object.
func TestEqualUnreadable(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	bad := src.Alloc()
	if err := src.Put(bad, pdf.Integer(1)); err != nil {
		t.Fatal(err)
	}
	srcX := pdf.NewExtractor(unreadableGetter{Getter: src, bad: bad})

	o := Extract(srcX, pdf.Dict{
		"A": pdf.Array{pdf.Dict{"K": bad}},
		"B": pdf.Integer(2),
	})
	if !o.Equal(o) {
		t.Error("o.Equal(o) = false")
	}
	want := Direct(pdf.Dict{"A": pdf.Array{pdf.Dict{}}, "B": pdf.Integer(2)})
	if !o.Equal(want) {
		t.Error("unreadable object does not compare as null")
	}

	dst, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(dst)
	native, err := rm.Embed(o)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	copied := Extract(pdf.NewExtractor(dst), native)
	if !o.Equal(copied) {
		t.Error("round trip changed the value")
	}
}

// failingGetter wraps a [pdf.Getter] and fails to read the object bad with
// an I/O error, which is not a sign of a malformed file.
type failingGetter struct {
	pdf.Getter
	bad pdf.Reference
}

func (g failingGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.bad {
		return nil, errors.New("I/O error")
	}
	return g.Getter.Get(ref, canObjStm)
}

// TestEqualReadError checks that a value which cannot be read because of an
// I/O error compares unequal to everything, on either side of the comparison.
func TestEqualReadError(t *testing.T) {
	src, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	bad := src.Alloc()
	if err := src.Put(bad, pdf.Integer(1)); err != nil {
		t.Fatal(err)
	}
	srcX := pdf.NewExtractor(failingGetter{Getter: src, bad: bad})

	o := Extract(srcX, pdf.Array{bad})
	other := Direct(pdf.Array{pdf.Integer(1)})
	if o.Equal(other) {
		t.Error("o.Equal(other) = true")
	}
	if other.Equal(o) {
		t.Error("other.Equal(o) = true")
	}
}

// TestEqualArrayLength checks that arrays of different lengths are unequal.
func TestEqualArrayLength(t *testing.T) {
	a := Direct(pdf.Array{pdf.Integer(1)})
	b := Direct(pdf.Array{pdf.Integer(1), pdf.Integer(2)})
	if a.Equal(b) {
		t.Error("a.Equal(b) = true")
	}
}

func TestAsDirectDict(t *testing.T) {
	dict := pdf.Dict{"A": pdf.Integer(1)}
	if diff := cmp.Diff(dict, Direct(dict).AsDirectDict()); diff != "" {
		t.Errorf("direct dict (-want +got):\n%s", diff)
	}
	if got := Direct(pdf.Dict{"A": pdf.NewReference(1, 0)}).AsDirectDict(); got != nil {
		t.Errorf("dict with reference: got %v, want nil", got)
	}
	if got := Direct(pdf.Integer(1)).AsDirectDict(); got != nil {
		t.Errorf("integer: got %v, want nil", got)
	}
}
