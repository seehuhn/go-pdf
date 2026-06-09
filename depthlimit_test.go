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
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/limits"
)

// deepExtract recursively follows the /Next reference of each dict via
// ExtractorGet, so the call stack grows one frame per indirect-reference
// level — the shape every real recursive extractor (color spaces, functions,
// actions, fields, ...) has.  It returns the chain length it managed to walk.
func deepExtract(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (int, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return 0, err
	}
	if dict["Next"] == nil {
		return 0, nil
	}
	n, err := pdf.ExtractorGet(x, path, dict["Next"], deepExtract)
	if err != nil {
		return 0, err
	}
	return n + 1, nil
}

// TestExtractorGetDepthBounded guards against a stack-overflow DoS: a chain of
// distinct indirect objects is acyclic, so the cycle guard never trips, yet a
// recursive extractor following it would exhaust the Go call stack.  The depth
// cap in ExtractorGet must turn this into a malformed-file error rather than a
// crash.
func TestExtractorGetDepthBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// build /Next chain: refs[i] -> refs[i+1], last one has no /Next
	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		node := pdf.Dict{}
		if i+1 < depth {
			node["Next"] = refs[i+1]
		}
		if err := w.Put(ref, node); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	_, err := pdf.ExtractorGet(x, nil, refs[0], deepExtract)
	if !pdf.IsMalformed(err) {
		t.Errorf("err = %v, want malformed", err)
	}
	if !errors.Is(err, pdf.ErrDepth) {
		t.Errorf("err = %v, want ErrDepth", err)
	}
}
