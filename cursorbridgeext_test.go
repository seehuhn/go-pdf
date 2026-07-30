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

package pdf_test

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// The decoders below model the situation [pdf.Cursor.AtRef] is for: an outer
// decoder which inspects the resolved object before handing it to a delegate,
// and a delegate whose results callers compare by pointer.  The two return
// different Go types, so they occupy separate slots in the extractor cache,
// as oc.ExtractConditional and oc.ExtractGroup do.

type atRefLeaf struct {
	name string
}

type atRefHolder struct {
	leaf *atRefLeaf
}

func decodeAtRefLeaf(c pdf.Cursor, obj pdf.Object, _ bool) (*atRefLeaf, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	name, err := c.Name(dict["Name"])
	if err != nil {
		return nil, err
	}
	return &atRefLeaf{name: string(name)}, nil
}

// decodeAtRefHolder delegates the way a decoder should: it steps back to the
// reference it was called for before handing the object on.
func decodeAtRefHolder(c pdf.Cursor, obj pdf.Object, isDirect bool) (*atRefHolder, error) {
	if _, err := c.Dict(obj); err != nil { // inspect the resolved object first
		return nil, err
	}
	c, obj = c.AtRef(obj, isDirect)
	leaf, err := pdf.Decode(c, obj, decodeAtRefLeaf)
	if err != nil {
		return nil, err
	}
	return &atRefHolder{leaf: leaf}, nil
}

// decodeAtRefResolved delegates the way AtRef exists to prevent: it passes the
// resolved object on, so the delegate runs under no reference.
func decodeAtRefResolved(c pdf.Cursor, obj pdf.Object, _ bool) (*atRefHolder, error) {
	leaf, err := pdf.Decode(c, obj, decodeAtRefLeaf)
	if err != nil {
		return nil, err
	}
	return &atRefHolder{leaf: leaf}, nil
}

// decodeBothWays decodes ref through a fresh extractor twice, once with the
// delegate alone and once through the given outer decoder, and returns the two
// leaves.  outerFirst chooses which runs first: a reader can meet an object
// either way round, and the shared value must not depend on which.  The
// extractor is fresh so that a value cached earlier cannot mask what the
// decoders did.
func decodeBothWays(t *testing.T, r pdf.Getter, ref pdf.Reference, outerFirst bool,
	outer func(pdf.Cursor, pdf.Object, bool) (*atRefHolder, error)) (*atRefLeaf, *atRefLeaf) {
	t.Helper()

	x := pdf.NewExtractor(r)
	viaOuter := func() *atRefLeaf {
		holder, err := pdf.Decode(pdf.CursorAt(x, nil), ref, outer)
		if err != nil {
			t.Fatal(err)
		}
		return holder.leaf
	}
	viaDelegate := func() *atRefLeaf {
		leaf, err := pdf.Decode(pdf.CursorAt(x, nil), ref, decodeAtRefLeaf)
		if err != nil {
			t.Fatal(err)
		}
		return leaf
	}

	if outerFirst {
		second := viaOuter()
		return viaDelegate(), second
	}
	return viaDelegate(), viaOuter()
}

// TestCursorAtRefOneValuePerReference checks the guarantee AtRef exists for:
// an object reached through a delegating decoder is the same Go value as one
// decoded with the delegate directly, so callers can compare by pointer.
func TestCursorAtRefOneValuePerReference(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	ref := w.Alloc()
	if err := w.Put(ref, pdf.Dict{"Name": pdf.Name("leaf")}); err != nil {
		t.Fatal(err)
	}

	for _, order := range []struct {
		name       string
		outerFirst bool
	}{
		{name: "delegate first"},
		{name: "outer first", outerFirst: true},
	} {
		t.Run(order.name, func(t *testing.T) {
			direct, viaAtRef := decodeBothWays(t, w, ref, order.outerFirst, decodeAtRefHolder)
			if viaAtRef != direct {
				t.Error("delegating through AtRef gave a second value for one reference")
			}

			// Without AtRef the delegate decodes the resolved dictionary
			// afresh and nothing is cached under the reference.  This is the
			// failure the check above rules out; testing for it here keeps
			// that check from passing for some unrelated reason.
			direct, viaResolved := decodeBothWays(t, w, ref, order.outerFirst, decodeAtRefResolved)
			if viaResolved == direct {
				t.Error("delegating without AtRef shared a value, so the check above proves nothing")
			}
		})
	}
}

// TestCursorAtRefPassThrough checks the two cases in which there is no
// reference to step back to: a direct object, whose enclosing reference on the
// path is not its own, and a cursor built by hand with no path at all.
func TestCursorAtRefPassThrough(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	containerRef := w.Alloc()
	if err := w.Put(containerRef, pdf.Dict{"Name": pdf.Name("container")}); err != nil {
		t.Fatal(err)
	}

	// a dictionary written inline inside the object containerRef, as [pdf.Decode]
	// would present it
	t.Run("direct object", func(t *testing.T) {
		c := pdf.CursorAt(pdf.NewExtractor(w), &pdf.CycleCheck{Ref: containerRef})
		holder, err := decodeAtRefHolder(c, pdf.Dict{"Name": pdf.Name("inline")}, true)
		if err != nil {
			t.Fatal(err)
		}
		if holder.leaf.name != "inline" {
			t.Errorf("decoded %q, want \"inline\": a direct object has no reference of its own",
				holder.leaf.name)
		}
	})

	// a decoder called by hand rather than through [pdf.Decode] can be handed a
	// cursor with no path together with isDirect false
	t.Run("cursor with no path", func(t *testing.T) {
		c := pdf.CursorAt(pdf.NewExtractor(w), nil)
		holder, err := decodeAtRefHolder(c, containerRef, false)
		if err != nil {
			t.Fatal(err)
		}
		if holder.leaf.name != "container" {
			t.Errorf("decoded %q, want \"container\"", holder.leaf.name)
		}
	})
}

// decodeAtRefLink and decodeAtRefLinkLeaf are a delegating decoder pair whose
// delegate follows a link to the next object, so that a cycle in the file runs
// back through AtRef.
func decodeAtRefLink(c pdf.Cursor, obj pdf.Object, isDirect bool) (*atRefHolder, error) {
	if _, err := c.Dict(obj); err != nil {
		return nil, err
	}
	c, obj = c.AtRef(obj, isDirect)
	leaf, err := pdf.Decode(c, obj, decodeAtRefLinkLeaf)
	if err != nil {
		return nil, err
	}
	return &atRefHolder{leaf: leaf}, nil
}

func decodeAtRefLinkLeaf(c pdf.Cursor, obj pdf.Object, _ bool) (*atRefLeaf, error) {
	dict, err := c.Dict(obj)
	if err != nil {
		return nil, err
	}
	if next := dict["Next"]; next != nil {
		if _, err := pdf.Decode(c, next, decodeAtRefLink); err != nil {
			return nil, err
		}
	}
	return &atRefLeaf{}, nil
}

// TestCursorAtRefCycle checks that stepping back to a reference and decoding it
// again leaves cycle detection working, so that a cycle running through a
// delegating decoder terminates with an error rather than recursing forever.
func TestCursorAtRefCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	refA, refB := w.Alloc(), w.Alloc()
	if err := w.Put(refA, pdf.Dict{"Next": refB}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, pdf.Dict{"Next": refA}); err != nil {
		t.Fatal(err)
	}

	c := pdf.CursorAt(pdf.NewExtractor(w), nil)
	if _, err := pdf.Decode(c, refA, decodeAtRefLink); err == nil {
		t.Error("no error for a cycle running through a delegating decoder")
	}
}
