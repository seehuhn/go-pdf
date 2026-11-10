// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package action

import (
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestSoundVolumeInvalid checks that a Volume which is out of range, of the
// wrong type, or a reference to a nonexistent object reads like an absent
// entry, and that the result can be written back.
func TestSoundVolumeInvalid(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	encoded, err := (&Sound{Sound: testSound, Volume: 0.5}).Encode(rm)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	base := encoded.(pdf.Dict)

	x := pdf.NewExtractor(w)
	absent := maps.Clone(base)
	delete(absent, "Volume")
	a, err := Decode(pdf.CursorAt(x, nil), absent, true)
	if err != nil {
		t.Fatal(err)
	}
	want := a.(*Sound).Volume

	invalid := map[string]pdf.Object{
		"too large": pdf.Number(1.5),
		"too small": pdf.Number(-1.5),
		"huge":      pdf.Number(100),
		"name":      pdf.Name("x"),
		"dangling":  pdf.NewReference(99, 0),
	}
	for name, value := range invalid {
		t.Run(name, func(t *testing.T) {
			dict := maps.Clone(base)
			dict["Volume"] = value
			a, err := Decode(pdf.CursorAt(x, nil), dict, true)
			if err != nil {
				t.Fatal(err)
			}
			if got := a.(*Sound).Volume; got != want {
				t.Errorf("Volume %v read as %g, want %g", value, got, want)
			}
			if _, err := a.Encode(rm); err != nil {
				t.Errorf("re-encode failed: %v", err)
			}
		})
	}
}
