// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

// TestGoToEFileOptional checks that a GoToE action reads without F where T
// locates the file, that a malformed F or T fails the action even where the
// other entry is present, and that an action with nothing to locate the file
// is rejected on both read and write.
func TestGoToEFileOptional(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	dest := pdf.Array{pdf.Integer(0), pdf.Name("Fit")}
	target := pdf.Dict{"R": pdf.Name("P")}
	fileSpec := pdf.Dict{"F": pdf.String("a.pdf")}

	a, err := Decode(c, pdf.Dict{"S": pdf.Name("GoToE"), "D": dest, "T": target}, true)
	if err != nil {
		t.Fatal(err)
	}
	if f := a.(*GoToE).F; f != nil {
		t.Errorf("F = %v, want nil", f)
	}

	invalid := map[string]pdf.Dict{
		"without F and T":  {"S": pdf.Name("GoToE"), "D": dest},
		"with bad F":       {"S": pdf.Name("GoToE"), "D": dest, "F": pdf.Integer(5)},
		"with T and bad F": {"S": pdf.Name("GoToE"), "D": dest, "T": target, "F": pdf.Integer(5)},
		"with F and bad T": {"S": pdf.Name("GoToE"), "D": dest, "F": fileSpec, "T": pdf.Integer(5)},
	}
	for name, dict := range invalid {
		if _, err := Decode(c, dict, true); !pdf.IsMalformed(err) {
			t.Errorf("%s: got %v, want malformed error", name, err)
		}
	}

	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	a = &GoToE{D: &destination.Fit{Page: pdf.Integer(0)}}
	if _, err := a.Encode(rm); err == nil {
		t.Error("encode without file succeeded")
	}
}
