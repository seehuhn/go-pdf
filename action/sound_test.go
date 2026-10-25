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
