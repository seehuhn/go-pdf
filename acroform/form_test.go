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

package acroform

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func textField(name string) *FieldTx { return NewTextField(name) }

func TestEncodeInvalidAlign(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	f := NewTextField("f")
	f.Align = pdf.TextAlign(99)
	form := &InteractiveForm{Fields: []TreeNode{f}}

	if _, err := form.Encode(rm); err == nil {
		t.Error("expected error for out-of-range alignment, got nil")
	}
}

func TestEncodeVersionGating(t *testing.T) {
	// the XFA array form requires PDF 1.6; encoding it to a PDF 1.4 file must fail.
	w, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm := pdf.NewResourceManager(w)

	form := &InteractiveForm{
		Fields: []TreeNode{textField("f")},
		XFA:    pdf.Array{pdf.String("x")},
	}

	_, err := form.Encode(rm)
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected version error, got %v", err)
	}
}

func TestEncodeXFAStreamForm(t *testing.T) {
	// the XFA stream form is valid from PDF 1.5, whereas the array form
	// requires PDF 1.6, so a non-array XFA value must encode at PDF 1.5.
	w, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)

	form := &InteractiveForm{
		Fields: []TreeNode{textField("f")},
		XFA:    rm.Out.Alloc(), // reference to a stream
	}

	if _, err := form.Encode(rm); err != nil {
		t.Errorf("unexpected error encoding XFA stream form at PDF 1.5: %v", err)
	}
}

func TestEncodeVersionGatingEntries(t *testing.T) {
	tests := []struct {
		name    string
		version pdf.Version
		build   func(rm *pdf.ResourceManager) *InteractiveForm
	}{
		{"form requires 1.2", pdf.V1_1, func(rm *pdf.ResourceManager) *InteractiveForm {
			return &InteractiveForm{Fields: []TreeNode{textField("f")}}
		}},
		{"SigFlags requires 1.3", pdf.V1_2, func(rm *pdf.ResourceManager) *InteractiveForm {
			return &InteractiveForm{
				Fields:   []TreeNode{textField("f")},
				SigFlags: SignaturesExist,
			}
		}},
		{"CO requires 1.3", pdf.V1_2, func(rm *pdf.ResourceManager) *InteractiveForm {
			f := textField("f")
			return &InteractiveForm{
				Fields:           []TreeNode{f},
				CalculationOrder: []Field{f},
			}
		}},
		{"XFA array requires 1.6", pdf.V1_5, func(rm *pdf.ResourceManager) *InteractiveForm {
			return &InteractiveForm{
				Fields: []TreeNode{textField("f")},
				XFA:    pdf.Array{pdf.String("template"), pdf.String("<xdp/>")},
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(tc.version, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := tc.build(rm).Encode(rm); !pdf.IsWrongVersion(err) {
				t.Errorf("expected version error, got %v", err)
			}
		})
	}
}
