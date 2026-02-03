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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
	"seehuhn.de/go/pdf/page/transition"
)

var actionTestCases = []Action{
	// GoTo
	&GoTo{Dest: &destination.Fit{Page: pdf.Reference(1)}},
	&GoTo{Dest: &destination.XYZ{Page: pdf.Reference(5), Left: 100, Top: 200, Zoom: 1.5}},

	// GoToR
	&GoToR{
		F: &file.Specification{FileName: "other.pdf", AFRelationship: "Unspecified"},
		D: &destination.Fit{Page: pdf.Integer(0)},
	},
	&GoToR{
		F:         &file.Specification{FileName: "other.pdf", AFRelationship: "Unspecified"},
		D:         &destination.Fit{Page: pdf.Integer(0)},
		NewWindow: NewWindowNew,
	},
	&GoToR{
		F:         &file.Specification{FileName: "other.pdf", AFRelationship: "Unspecified"},
		D:         &destination.Fit{Page: pdf.Integer(0)},
		NewWindow: NewWindowReplace,
	},
	&GoToR{
		F:  &file.Specification{FileName: "other.pdf", AFRelationship: "Unspecified"},
		D:  &destination.Fit{Page: pdf.Integer(0)},
		SD: pdf.Array{pdf.Integer(0), pdf.Name("Fit")},
	},

	// GoToE
	&GoToE{
		F: &file.Specification{FileName: "embedded.pdf", AFRelationship: "Unspecified"},
		D: &destination.Fit{Page: pdf.Integer(0)},
	},
	&GoToE{
		F:         &file.Specification{FileName: "embedded.pdf", AFRelationship: "Unspecified"},
		D:         &destination.Fit{Page: pdf.Integer(0)},
		NewWindow: NewWindowNew,
	},
	&GoToE{
		F: &file.Specification{FileName: "embedded.pdf", AFRelationship: "Unspecified"},
		D: &destination.Fit{Page: pdf.Integer(0)},
		T: &TargetNamedChild{Name: pdf.String("child.pdf"), Next: &TargetParent{}},
	},

	// GoToDp (PDF 2.0)
	&GoToDp{DPart: pdf.Reference(1)},

	// Launch
	&Launch{F: &file.Specification{FileName: "app.exe", AFRelationship: "Unspecified"}},
	&Launch{F: &file.Specification{FileName: "app.exe", AFRelationship: "Unspecified"}, NewWindow: NewWindowNew},
	&Launch{F: &file.Specification{FileName: "app.exe", AFRelationship: "Unspecified"}, NewWindow: NewWindowReplace},

	// Thread
	&Thread{D: pdf.Integer(0)},
	&Thread{D: pdf.Reference(1)},                      // thread by reference
	&Thread{D: pdf.Integer(0), B: pdf.Integer(0)},     // with bead index
	&Thread{D: pdf.Reference(1), B: pdf.Reference(2)}, // thread and bead by reference

	// URI
	&URI{URI: "https://example.com"},
	&URI{URI: "https://example.com", IsMap: true},
	&URI{URI: "mailto:user@example.com"},  // mailto URI
	&URI{URI: "file:///path/to/file.pdf"}, // file URI

	// Sound
	&Sound{Sound: pdf.Reference(1), Volume: 1.0},
	&Sound{Sound: pdf.Reference(1), Volume: 0.5},  // half volume
	&Sound{Sound: pdf.Reference(1), Volume: -1.0}, // negative volume (muted)
	&Sound{Sound: pdf.Reference(1), Volume: 1.0, Synchronous: true},
	&Sound{Sound: pdf.Reference(1), Volume: 1.0, Repeat: true},
	&Sound{Sound: pdf.Reference(1), Volume: 1.0, Mix: true},
	&Sound{Sound: pdf.Reference(1), Volume: 0.75, Synchronous: true, Repeat: true, Mix: true},

	// Movie
	&Movie{T: pdf.String("movie1")},
	&Movie{Annotation: pdf.Reference(1)}, // by annotation reference
	&Movie{T: pdf.String("intro"), Operation: "Play"},
	&Movie{T: pdf.String("video"), Operation: "Stop"},
	&Movie{T: pdf.String("clip"), Operation: "Pause"},
	&Movie{T: pdf.String("movie"), Operation: "Resume"},

	// Hide
	&Hide{T: pdf.String("annotation1"), H: true},
	&Hide{T: pdf.String("annotation1"), H: false},

	// Named
	&Named{N: "NextPage"},
	&Named{N: "PrevPage"},
	&Named{N: "FirstPage"},
	&Named{N: "LastPage"},
	&Named{N: "CustomAction"}, // non-standard action name
	&Named{N: "NextPage", Next: ActionList{&Named{N: "PrevPage"}}},      // with chaining
	&Named{N: "Print", Next: ActionList{&URI{URI: "https://help.com"}}}, // non-standard with chaining

	// SubmitForm
	&SubmitForm{F: pdf.String("http://example.com/submit")},
	&SubmitForm{
		F:      pdf.String("https://example.com/form"),
		Fields: pdf.Array{pdf.String("name"), pdf.String("email")},
	},
	&SubmitForm{
		F:     pdf.String("https://example.com/submit"),
		Flags: 1, // IncludeNoValueFields
	},
	&SubmitForm{
		F:      pdf.String("https://example.com/submit"),
		Fields: pdf.Array{pdf.String("field1")},
		Flags:  4, // ExportFormat
	},

	// ResetForm
	&ResetForm{},
	&ResetForm{Fields: pdf.Array{pdf.String("name"), pdf.String("address")}},
	&ResetForm{Flags: 1}, // Include (reset only listed fields)
	&ResetForm{
		Fields: pdf.Array{pdf.String("signature")},
		Flags:  1,
	},

	// ImportData
	&ImportData{F: &file.Specification{FileName: "data.fdf", AFRelationship: "Unspecified"}},

	// SetOCGState
	&SetOCGState{State: pdf.Array{pdf.Name("ON"), pdf.Reference(1)}, PreserveRB: true},
	&SetOCGState{State: pdf.Array{pdf.Name("OFF"), pdf.Reference(1)}, PreserveRB: false},
	&SetOCGState{State: pdf.Array{pdf.Name("Toggle"), pdf.Reference(1), pdf.Reference(2)}},
	&SetOCGState{
		State: pdf.Array{
			pdf.Name("ON"), pdf.Reference(1), pdf.Reference(2),
			pdf.Name("OFF"), pdf.Reference(3),
		},
		PreserveRB: true,
	},

	// Rendition
	&Rendition{OP: func() optional.UInt { var u optional.UInt; u.Set(0); return u }()},

	// Trans
	&Trans{Trans: &transition.Transition{Style: transition.StyleSplit}},

	// GoTo3DView
	&GoTo3DView{TA: pdf.Reference(1), V: pdf.Name("Default")},

	// JavaScript
	&JavaScript{JS: pdf.String("app.alert('Hello');")},

	// RichMediaExecute (PDF 2.0)
	&RichMediaExecute{TA: pdf.Reference(1)},

	// action chaining
	&GoTo{
		Dest: &destination.Fit{Page: pdf.Reference(1)},
		Next: ActionList{
			&URI{
				URI: "https://example.com",
				Next: ActionList{
					&Named{N: "NextPage"},
				},
			},
		},
	},
	&JavaScript{
		JS:   pdf.String("app.alert('test');"),
		Next: ActionList{&Named{N: "LastPage"}},
	},
	&Hide{
		T:    pdf.String("btn1"),
		H:    true,
		Next: ActionList{&Hide{T: pdf.String("btn2"), H: false}},
	},
}

func testActionRoundTrip(t *testing.T, version pdf.Version, action Action) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	encoded, err := action.Encode(rm)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("encode error: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close error: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := Decode(x, encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if diff := cmp.Diff(decoded, action); diff != "" {
		t.Errorf("round trip failed (-got +want):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	versions := []pdf.Version{pdf.V1_7, pdf.V2_0}

	for _, version := range versions {
		t.Run(version.String(), func(t *testing.T) {
			for _, action := range actionTestCases {
				t.Run(string(action.ActionType()), func(t *testing.T) {
					testActionRoundTrip(t, version, action)
				})
			}
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}

	for _, action := range actionTestCases {
		w, buf := memfile.NewPDFWriter(pdf.V1_7, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		obj, err := action.Encode(rm)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
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

		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		action, err := Decode(x, obj)
		if err != nil {
			t.Skip("malformed action")
		}

		testActionRoundTrip(t, pdf.GetVersion(r), action)
	})
}
