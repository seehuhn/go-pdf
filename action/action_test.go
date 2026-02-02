// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestActionListEncode_Empty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	var al ActionList
	obj, err := al.Encode(rm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj != nil {
		t.Errorf("expected nil for empty ActionList, got %v", obj)
	}
}

func TestActionListMultipleActions(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)

	// create multiple parallel actions
	action := &GoTo{
		Dest: &destination.Fit{Page: pdf.Reference(1)},
		Next: ActionList{
			&URI{URI: "https://example.com"},
			&Named{N: "NextPage"},
		},
	}

	// encode
	obj, err := action.Encode(rm)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	dict := obj.(pdf.Dict)
	nextArr, ok := dict["Next"].(pdf.Array)
	if !ok {
		t.Fatalf("expected Next to be array, got %T", dict["Next"])
	}

	if len(nextArr) != 2 {
		t.Errorf("expected 2 actions in Next array, got %d", len(nextArr))
	}

	// decode and verify
	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, obj)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goTo := decoded.(*GoTo)
	if len(goTo.Next) != 2 {
		t.Fatalf("expected 2 next actions, got %d", len(goTo.Next))
	}
}

func TestNewWindowMode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)
	x := pdf.NewExtractor(w)

	tests := []struct {
		name   string
		action Action
		mode   NewWindowMode
	}{
		{
			name: "GoToR default",
			action: &GoToR{
				F:         &file.Specification{FileName: "other.pdf"},
				D:         &destination.Fit{Page: pdf.Integer(0)},
				NewWindow: NewWindowDefault,
			},
			mode: NewWindowDefault,
		},
		{
			name: "GoToR replace",
			action: &GoToR{
				F:         &file.Specification{FileName: "other.pdf"},
				D:         &destination.Fit{Page: pdf.Integer(0)},
				NewWindow: NewWindowReplace,
			},
			mode: NewWindowReplace,
		},
		{
			name: "GoToR new",
			action: &GoToR{
				F:         &file.Specification{FileName: "other.pdf"},
				D:         &destination.Fit{Page: pdf.Integer(0)},
				NewWindow: NewWindowNew,
			},
			mode: NewWindowNew,
		},
		{
			name: "GoToE default",
			action: &GoToE{
				F:         &file.Specification{FileName: "embedded.pdf"},
				D:         &destination.Fit{Page: pdf.Integer(0)},
				NewWindow: NewWindowDefault,
			},
			mode: NewWindowDefault,
		},
		{
			name: "GoToE new",
			action: &GoToE{
				F:         &file.Specification{FileName: "embedded.pdf"},
				D:         &destination.Fit{Page: pdf.Integer(0)},
				NewWindow: NewWindowNew,
			},
			mode: NewWindowNew,
		},
		{
			name: "Launch default",
			action: &Launch{
				F:         &file.Specification{FileName: "app.exe"},
				NewWindow: NewWindowDefault,
			},
			mode: NewWindowDefault,
		},
		{
			name: "Launch replace",
			action: &Launch{
				F:         &file.Specification{FileName: "app.exe"},
				NewWindow: NewWindowReplace,
			},
			mode: NewWindowReplace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, err := tt.action.Encode(rm)
			if err != nil {
				t.Fatalf("encode error: %v", err)
			}

			dict := obj.(pdf.Dict)

			// verify encoding
			if tt.mode == NewWindowDefault {
				if dict["NewWindow"] != nil {
					t.Errorf("expected NewWindow to be omitted for default, got %v", dict["NewWindow"])
				}
			} else {
				expected := pdf.Boolean(tt.mode == NewWindowNew)
				if dict["NewWindow"] != expected {
					t.Errorf("NewWindow = %v, want %v", dict["NewWindow"], expected)
				}
			}

			// decode and verify
			decoded, err := Decode(x, obj)
			if err != nil {
				t.Fatalf("decode error: %v", err)
			}

			var decodedMode NewWindowMode
			switch a := decoded.(type) {
			case *GoToR:
				decodedMode = a.NewWindow
			case *GoToE:
				decodedMode = a.NewWindow
			case *Launch:
				decodedMode = a.NewWindow
			default:
				t.Fatalf("unexpected action type: %T", decoded)
			}

			if decodedMode != tt.mode {
				t.Errorf("decoded NewWindow = %v, want %v", decodedMode, tt.mode)
			}
		})
	}
}
