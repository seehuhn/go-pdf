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

package navnode

import (
	"bytes"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

var testCases = []struct {
	name  string
	nodes []*Node
}{
	{
		name:  "empty",
		nodes: nil,
	},
	{
		name: "single_node_no_actions",
		nodes: []*Node{
			{},
		},
	},
	{
		name: "single_node_with_dur",
		nodes: []*Node{
			{Dur: 5.0},
		},
	},
	{
		name: "single_node_with_action",
		nodes: []*Node{
			{NA: &action.URI{URI: "https://example.com"}},
		},
	},
	{
		name: "multiple_nodes",
		nodes: []*Node{
			{Dur: 2.0},
			{Dur: 3.0},
			{Dur: 4.0},
		},
	},
	{
		name: "complex",
		nodes: []*Node{
			{
				NA:  &action.URI{URI: "https://example.com/1"},
				PA:  &action.URI{URI: "https://example.com/back"},
				Dur: 5.0,
			},
			{
				NA: &action.URI{URI: "https://example.com/2"},
			},
		},
	},
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, pdf.V1_5, tc.nodes)
		})
	}
}

func roundTripTest(t *testing.T, version pdf.Version, nodes1 []*Node) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, version, nil)
	rm := pdf.NewResourceManager(w)

	encoded, err := Encode(rm, nodes1)
	if pdf.IsWrongVersion(err) {
		t.Skip("version not supported")
	} else if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}

	// store encoded reference in a dummy object so we can retrieve it
	if encoded != nil {
		if err := w.Put(w.Alloc(), pdf.Dict{"NavNodes": encoded}); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	nodes2, err := Decode(pdf.CursorAt(x, nil), encoded, false)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// compare using action type equality
	opts := []cmp.Option{
		cmp.Comparer(func(a, b pdf.Action) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return a.ActionType() == b.ActionType()
			// TODO: deeper comparison when action.Equal exists
		}),
	}
	if diff := cmp.Diff(nodes1, nodes2, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range testCases {
		if len(tc.nodes) == 0 {
			continue
		}

		w, buf := memfile.NewPDFWriter(f, pdf.V1_5, opt)

		rm := pdf.NewResourceManager(w)
		encoded, err := Encode(rm, tc.nodes)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:N"] = encoded

		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:N"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		nodes, err := Decode(pdf.CursorAt(x, nil), obj, false)
		if err != nil {
			t.Skip("malformed navigation nodes")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		roundTripTest(t, version, nodes)
	})
}

// TestDecodeDanglingReference checks that a reference to a free object reads
// the same as an absent entry.
func TestDecodeDanglingReference(t *testing.T) {
	dangling := pdf.NewReference(99, 0)
	decode := func(t *testing.T, extras pdf.Dict) []*Node {
		t.Helper()
		w, _ := memfile.NewPDFWriter(t, pdf.V1_5, nil)
		ref := w.Alloc()
		dict := pdf.Dict{"Type": pdf.Name("NavNode")}
		maps.Copy(dict, extras)
		if err := w.Put(ref, dict); err != nil {
			t.Fatal(err)
		}
		x := pdf.NewExtractor(w)
		nodes, err := Decode(pdf.CursorAt(x, nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		return nodes
	}

	want := decode(t, nil)
	for _, key := range []pdf.Name{"NA", "PA", "Dur", "Next", "Prev"} {
		t.Run(string(key), func(t *testing.T) {
			got := decode(t, pdf.Dict{key: dangling})
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("dangling reference differs from absent entry (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDecodeWrongType checks that wrong-type entries read the same as absent
// entries, and that a malformed Next link ends the list.
func TestDecodeWrongType(t *testing.T) {
	decode := func(t *testing.T, extras pdf.Dict) []*Node {
		t.Helper()
		w, _ := memfile.NewPDFWriter(t, pdf.V1_5, nil)
		ref := w.Alloc()
		dict := pdf.Dict{"Type": pdf.Name("NavNode")}
		maps.Copy(dict, extras)
		if err := w.Put(ref, dict); err != nil {
			t.Fatal(err)
		}
		x := pdf.NewExtractor(w)
		nodes, err := Decode(pdf.CursorAt(x, nil), ref, false)
		if err != nil {
			t.Fatal(err)
		}
		return nodes
	}

	want := decode(t, nil)
	for _, key := range []pdf.Name{"NA", "PA", "Dur", "Next", "Prev"} {
		t.Run(string(key), func(t *testing.T) {
			got := decode(t, pdf.Dict{key: pdf.Name("wrong")})
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("wrong type differs from absent entry (-want +got):\n%s", diff)
			}
		})
	}
}

// TestDecodeLongChain checks that a list longer than the nesting depth
// limit of the cursor path is read in full.
func TestDecodeLongChain(t *testing.T) {
	n := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(t, pdf.V1_5, nil)
	refs := make([]pdf.Reference, n)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		dict := pdf.Dict{"Type": pdf.Name("NavNode"), "Dur": pdf.Integer(i + 1)}
		if i+1 < n {
			dict["Next"] = refs[i+1]
		}
		if err := w.Put(ref, dict); err != nil {
			t.Fatal(err)
		}
	}

	nodes, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Decode(refs[0], Decode)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != n {
		t.Fatalf("got %d nodes, want %d", len(nodes), n)
	}
	for i, node := range nodes {
		if node.Dur != float64(i+1) {
			t.Errorf("node %d has Dur %g, want %d", i, node.Dur, i+1)
		}
	}
}

// TestDecodeCycle checks that a Next link back to a node already read ends
// the list, so that each node is read once.
func TestDecodeCycle(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_5, nil)
	a, b, c := w.Alloc(), w.Alloc(), w.Alloc()
	for ref, dict := range map[pdf.Reference]pdf.Dict{
		a: {"Dur": pdf.Integer(1), "Next": b},
		b: {"Dur": pdf.Integer(2), "Next": c},
		c: {"Dur": pdf.Integer(3), "Next": b},
	} {
		if err := w.Put(ref, dict); err != nil {
			t.Fatal(err)
		}
	}

	nodes, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Decode(a, Decode)
	if err != nil {
		t.Fatal(err)
	}
	want := []*Node{{Dur: 1}, {Dur: 2}, {Dur: 3}}
	if diff := cmp.Diff(want, nodes); diff != "" {
		t.Errorf("unexpected nodes (-want +got):\n%s", diff)
	}
}
