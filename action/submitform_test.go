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
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// TestSubmitFormTarget checks that the F entry of a SubmitForm action is read
// as a string or a dictionary, and survives a read-write-read cycle.
func TestSubmitFormTarget(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	spec := pdf.Dict{"Type": pdf.Name("Filespec"), "FS": pdf.Name("URL"), "F": pdf.String("https://example.com/")}
	specRef := w.Alloc()
	if err := w.Put(specRef, spec); err != nil {
		t.Fatal(err)
	}
	urlRef := w.Alloc()
	if err := w.Put(urlRef, pdf.String("https://example.com/")); err != nil {
		t.Fatal(err)
	}
	c := pdf.NewCursor(w)

	valid := map[string]struct{ f, want pdf.Object }{
		"string":               {pdf.String("https://example.com/"), pdf.String("https://example.com/")},
		"string reference":     {urlRef, pdf.String("https://example.com/")},
		"dictionary":           {spec, spec},
		"dictionary reference": {specRef, spec},
	}
	for name, tc := range valid {
		t.Run(name, func(t *testing.T) {
			a, err := Decode(c, pdf.Dict{"S": pdf.Name("SubmitForm"), "F": tc.f}, true)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, a.(*SubmitForm).F); diff != "" {
				t.Errorf("wrong F (-want +got):\n%s", diff)
			}
			testActionRoundTrip(t, pdf.V1_7, a)
		})
	}

	invalid := map[string]pdf.Object{
		"null":     nil,
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(1),
		"array":    pdf.Array{pdf.String("https://example.com/")},
		"dangling": pdf.NewReference(99, 0),
	}
	for name, value := range invalid {
		t.Run("invalid "+name, func(t *testing.T) {
			dict := pdf.Dict{"S": pdf.Name("SubmitForm"), "F": value}
			if _, err := Decode(c, dict, true); !pdf.IsMalformed(err) {
				t.Errorf("read: got %v, want malformed error", err)
			}
		})
	}

	// a reference cannot be checked on write
	rm := pdf.NewResourceManager(w)
	for name, value := range invalid {
		if value == nil || name == "dangling" {
			continue
		}
		if _, err := (&SubmitForm{F: value}).Encode(rm); err == nil {
			t.Errorf("%s: encode succeeded", name)
		}
	}
	if _, err := (&SubmitForm{}).Encode(rm); err == nil {
		t.Error("encode without F succeeded")
	}
}
