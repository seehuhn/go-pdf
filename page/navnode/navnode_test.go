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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
	"seehuhn.de/go/pdf/internal/debug/memfile"
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

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	encoded, err := Encode(rm, nodes1)
	var versionError *pdf.VersionError
	if errors.As(err, &versionError) {
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
	nodes2, err := Decode(x, encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// compare using action type equality
	opts := []cmp.Option{
		cmp.Comparer(func(a, b action.Action) bool {
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

		w, buf := memfile.NewPDFWriter(pdf.V1_5, opt)
		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

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
		r, err := pdf.NewReader(bytes.NewReader(fileData), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}

		obj := r.GetMeta().Trailer["Quir:N"]
		if obj == nil {
			t.Skip("missing object")
		}

		x := pdf.NewExtractor(r)
		nodes, err := Decode(x, obj)
		if err != nil {
			t.Skip("malformed navigation nodes")
		}

		roundTripTest(t, pdf.GetVersion(r), nodes)
	})
}
