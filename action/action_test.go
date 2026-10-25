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
	"errors"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/destination"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/internal/limits"
	"seehuhn.de/go/pdf/sound"
)

func TestActionListEncode_Empty(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
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
	decoded, err := Decode(pdf.CursorAt(x, nil), obj, false)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	goTo := decoded.(*GoTo)
	if len(goTo.Next) != 2 {
		t.Fatalf("expected 2 next actions, got %d", len(goTo.Next))
	}
}

func TestNewWindowMode(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	defer w.Close()
	rm := pdf.NewResourceManager(w)
	x := pdf.NewExtractor(w)

	tests := []struct {
		name   string
		action pdf.Action
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
			decoded, err := Decode(pdf.CursorAt(x, nil), obj, false)
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

// TestDecodeActionListNextCycleSelf checks that a URI action with a /Next
// entry pointing back at itself fails with a cycle error, instead of
// recursing until the goroutine stack is exhausted.
func TestDecodeActionListNextCycleSelf(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/"),
		"Next": ref,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(ref, Decode); !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("got %v, want cycle error", err)
	}
}

// TestDecodeActionListNextCycleMutual checks that two URI actions with
// /Next entries pointing at each other fail with a cycle error, whichever
// of them is read first.
func TestDecodeActionListNextCycleMutual(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refA := w.Alloc()
	refB := w.Alloc()
	if err := w.Put(refA, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/a"),
		"Next": refB,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/b"),
		"Next": refA,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	for _, order := range [][]pdf.Reference{{refA, refB}, {refB, refA}} {
		c := pdf.CursorAt(pdf.NewExtractor(w), nil)
		for _, ref := range order {
			if _, err := c.Decode(ref, Decode); !errors.Is(err, pdf.ErrCycle) {
				t.Errorf("order %v, %v: got %v, want cycle error", order, ref, err)
			}
		}
	}
}

// TestDecodeActionListNextCycleOtherTypes checks that a /Next cycle is
// detected for action types other than URI, whichever action of the cycle
// is read first.
func TestDecodeActionListNextCycleOtherTypes(t *testing.T) {
	spec := pdf.Dict{"Type": pdf.Name("Filespec"), "FS": pdf.Name("URL"), "F": pdf.String("a.pdf")}
	bases := map[string]pdf.Dict{
		"Named":      {"S": pdf.Name("Named"), "N": pdf.Name("NextPage")},
		"Hide":       {"S": pdf.Name("Hide"), "T": pdf.String("btn")},
		"Launch":     {"S": pdf.Name("Launch"), "F": spec},
		"JavaScript": {"S": pdf.Name("JavaScript"), "JS": pdf.String("1;")},
		"SubmitForm": {"S": pdf.Name("SubmitForm"), "F": pdf.String("https://example.com/")},
		"Rendition":  {"S": pdf.Name("Rendition"), "AN": pdf.NewReference(99, 0), "OP": pdf.Integer(1)},
	}

	for name, base := range bases {
		t.Run(name+" self", func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			ref := w.Alloc()
			dict := maps.Clone(base)
			dict["Next"] = ref
			if err := w.Put(ref, dict); err != nil {
				t.Fatal(err)
			}
			if _, err := pdf.CursorAt(pdf.NewExtractor(w), nil).Decode(ref, Decode); !errors.Is(err, pdf.ErrCycle) {
				t.Errorf("got %v, want cycle error", err)
			}
		})

		t.Run(name+" mutual with Named", func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			refA := w.Alloc()
			refB := w.Alloc()
			dictA := maps.Clone(base)
			dictA["Next"] = refB
			dictB := maps.Clone(bases["Named"])
			dictB["Next"] = refA
			if err := w.Put(refA, dictA); err != nil {
				t.Fatal(err)
			}
			if err := w.Put(refB, dictB); err != nil {
				t.Fatal(err)
			}
			for _, order := range [][]pdf.Reference{{refA, refB}, {refB, refA}} {
				c := pdf.CursorAt(pdf.NewExtractor(w), nil)
				for _, ref := range order {
					if _, err := c.Decode(ref, Decode); !errors.Is(err, pdf.ErrCycle) {
						t.Errorf("order %v, %v: got %v, want cycle error", order, ref, err)
					}
				}
			}
		})
	}
}

// TestDecodeActionListNextCycleInlineDict checks that an inline (non-ref)
// /Next dictionary whose own /Next references the parent action is
// cycle-protected.
func TestDecodeActionListNextCycleInlineDict(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{
		"S":   pdf.Name("URI"),
		"URI": pdf.String("https://example.com/"),
		"Next": pdf.Dict{
			"S":    pdf.Name("URI"),
			"URI":  pdf.String("https://example.com/inner"),
			"Next": ref,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(ref, Decode); !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("got %v, want cycle error", err)
	}
}

// TestDecodeActionListNextCycleDeep checks that a /Next chain that loops
// between two refs neither of which is the entry ref (A → B → C → B) is
// detected. This is the case the path-extending pdf.Cursor.Decode wrapper
// handles: a cycle check that only catches refs already on the entry
// path would recurse forever between B and C.
func TestDecodeActionListNextCycleDeep(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refA := w.Alloc()
	refB := w.Alloc()
	refC := w.Alloc()
	if err := w.Put(refA, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/a"),
		"Next": refB,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refB, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/b"),
		"Next": refC,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Put(refC, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/c"),
		"Next": refB,
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(refA, Decode); !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("got %v, want cycle error", err)
	}
}

// TestDecodeActionListNextCycleArray checks that a /Next entry containing
// an array of action references is also cycle-protected.
func TestDecodeActionListNextCycleArray(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{
		"S":    pdf.Name("URI"),
		"URI":  pdf.String("https://example.com/"),
		"Next": pdf.Array{ref},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(ref, Decode); !errors.Is(err, pdf.ErrCycle) {
		t.Errorf("got %v, want cycle error", err)
	}
}

// TestDecodeActionListNextDeepChainBounded guards against a stack-overflow
// DoS: a /Next chain of distinct URI actions is acyclic, so the cycle guard
// never trips, yet recursing one frame per level would exhaust the Go stack.
// The Decode depth cap must fail the action rather than crash.
func TestDecodeActionListNextDeepChainBounded(t *testing.T) {
	depth := limits.MaxExtractDepth + 10
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	refs := make([]pdf.Reference, depth)
	for i := range refs {
		refs[i] = w.Alloc()
	}
	for i, ref := range refs {
		d := pdf.Dict{"S": pdf.Name("URI"), "URI": pdf.String("https://example.com/")}
		if i+1 < depth {
			d["Next"] = refs[i+1]
		}
		if err := w.Put(ref, d); err != nil {
			t.Fatal(err)
		}
	}

	x := pdf.NewExtractor(w)
	if _, err := pdf.CursorAt(x, nil).Decode(refs[0], Decode); !errors.Is(err, pdf.ErrDepth) {
		t.Errorf("got %v, want depth error", err)
	}
}

// TestNextInvalidIsAbsent checks that a /Next entry of the wrong type or
// pointing at a nonexistent object reads like an absent entry, for every
// action type.
func TestNextInvalidIsAbsent(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	spec := pdf.Dict{"Type": pdf.Name("Filespec"), "FS": pdf.Name("URL"), "F": pdf.String("a.pdf")}
	bases := []pdf.Dict{
		{"S": pdf.Name("URI"), "URI": pdf.String("https://example.com/")},
		{"S": pdf.Name("Named"), "N": pdf.Name("NextPage")},
		{"S": pdf.Name("GoTo"), "D": pdf.String("dest")},
		{"S": pdf.Name("GoToR"), "F": spec, "D": pdf.String("dest")},
		{"S": pdf.Name("Launch"), "F": spec},
		{"S": pdf.Name("Hide"), "T": pdf.String("btn")},
		{"S": pdf.Name("JavaScript"), "JS": pdf.String("1;")},
		{"S": pdf.Name("ResetForm")},
		{"S": pdf.Name("Thread"), "D": pdf.Integer(0)},
	}
	invalid := map[string]pdf.Object{
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(5),
		"dangling": pdf.NewReference(99, 0),
		"array":    pdf.Array{pdf.Integer(1), pdf.NewReference(99, 0)},
	}
	for _, base := range bases {
		for name, bad := range invalid {
			t.Run(string(base["S"].(pdf.Name))+"/"+name, func(t *testing.T) {
				dict := maps.Clone(base)
				dict["Next"] = bad
				want, err := Decode(c, base, true)
				if err != nil {
					t.Fatal(err)
				}
				got, err := Decode(c, dict, true)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("invalid Next read differently from absent entry (-absent +got):\n%s", diff)
				}
			})
		}
	}
}

// TestInvalidEntryIsAbsent checks that a reference to a nonexistent object,
// and a value of the wrong type, are read like an absent entry.
func TestInvalidEntryIsAbsent(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	soundRef, err := rm.Embed(testSound)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	invalid := map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(5),
	}
	// an integer is a valid value for the other entries
	integerInvalid := map[pdf.Name]bool{
		"NewWindow": true, "H": true, "PreserveRB": true,
		"ShowControls": true, "Synchronous": true,
		"IsMap": true, "Repeat": true, "Mix": true,
		"Fields": true, "State": true, "FWScale": true, "FWPosition": true,
	}
	c := pdf.NewCursor(w)
	spec := pdf.Dict{"Type": pdf.Name("Filespec"), "FS": pdf.Name("URL"), "F": pdf.String("a.pdf")}

	cases := []struct {
		name string
		base pdf.Dict
		keys []pdf.Name
	}{
		{
			"GoToR",
			pdf.Dict{"S": pdf.Name("GoToR"), "F": spec, "D": pdf.String("dest")},
			[]pdf.Name{"NewWindow"},
		},
		{
			"GoToE",
			pdf.Dict{"S": pdf.Name("GoToE"), "F": spec, "D": pdf.String("dest")},
			[]pdf.Name{"NewWindow"},
		},
		{
			"Launch",
			pdf.Dict{"S": pdf.Name("Launch"), "F": spec},
			[]pdf.Name{"NewWindow"},
		},
		{
			"URI",
			pdf.Dict{"S": pdf.Name("URI"), "URI": pdf.String("https://example.com/")},
			[]pdf.Name{"IsMap"},
		},
		{
			"Sound",
			pdf.Dict{"S": pdf.Name("Sound"), "Sound": soundRef},
			[]pdf.Name{"Volume", "Synchronous", "Repeat", "Mix"},
		},
		{
			"Hide",
			pdf.Dict{"S": pdf.Name("Hide"), "T": pdf.String("field")},
			[]pdf.Name{"H"},
		},
		{
			"SubmitForm",
			pdf.Dict{"S": pdf.Name("SubmitForm"), "F": pdf.String("https://example.com/")},
			[]pdf.Name{"Fields", "Flags"},
		},
		{
			"ResetForm",
			pdf.Dict{"S": pdf.Name("ResetForm")},
			[]pdf.Name{"Fields", "Flags"},
		},
		{
			"SetOCGState",
			pdf.Dict{"S": pdf.Name("SetOCGState")},
			[]pdf.Name{"State", "PreserveRB"},
		},
		{
			"Movie",
			pdf.Dict{"S": pdf.Name("Movie"), "T": pdf.String("movie")},
			[]pdf.Name{
				"Start", "Duration", "Rate", "Volume", "ShowControls",
				"Synchronous", "FWScale", "FWPosition",
			},
		},
		{
			"Rendition with OP",
			pdf.Dict{"S": pdf.Name("Rendition"), "AN": pdf.NewReference(5, 0), "OP": pdf.Integer(1)},
			[]pdf.Name{"JS"},
		},
		{
			"Rendition with JS",
			pdf.Dict{"S": pdf.Name("Rendition"), "AN": pdf.NewReference(5, 0), "JS": pdf.String("x")},
			[]pdf.Name{"OP"},
		},
	}
	for _, tc := range cases {
		for _, key := range tc.keys {
			for variant, value := range invalid {
				if variant == "integer" && !integerInvalid[key] {
					continue
				}
				t.Run(tc.name+" "+string(key)+" "+variant, func(t *testing.T) {
					absent := maps.Clone(tc.base)
					withInvalid := maps.Clone(tc.base)
					withInvalid[key] = value

					want, err := Decode(c, absent, true)
					if err != nil {
						t.Fatal(err)
					}
					got, err := Decode(c, withInvalid, true)
					if err != nil {
						t.Fatal(err)
					}
					if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(sound.Sound{}, "Data")); diff != "" {
						t.Errorf("%s value read differently from absent entry (-absent +got):\n%s", variant, diff)
					}
				})
			}
		}
	}
}

// TestDanglingRequiredEntry checks that a reference to a nonexistent object,
// and a value of the wrong type, do not satisfy a required entry.
func TestDanglingRequiredEntry(t *testing.T) {
	dangling := pdf.NewReference(99, 0)
	c := pdf.NewCursor(mock.Getter)

	cases := []struct {
		name string
		dict pdf.Dict
	}{
		{"JavaScript JS", pdf.Dict{"S": pdf.Name("JavaScript"), "JS": dangling}},
		{"JavaScript JS name", pdf.Dict{"S": pdf.Name("JavaScript"), "JS": pdf.Name("x")}},
		{"Hide T", pdf.Dict{"S": pdf.Name("Hide"), "T": dangling}},
		{"Hide T name", pdf.Dict{"S": pdf.Name("Hide"), "T": pdf.Name("x")}},
		{"Hide T integer", pdf.Dict{"S": pdf.Name("Hide"), "T": pdf.Integer(1)}},
		{"SubmitForm F", pdf.Dict{"S": pdf.Name("SubmitForm"), "F": dangling}},
		{"SubmitForm F name", pdf.Dict{"S": pdf.Name("SubmitForm"), "F": pdf.Name("x")}},
		{"SubmitForm F integer", pdf.Dict{"S": pdf.Name("SubmitForm"), "F": pdf.Integer(1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Decode(c, tc.dict, true); err == nil {
				t.Error("missing error")
			}
		})
	}
}

// TestNextArrayKeepsUsableElements checks that null, dangling and malformed
// elements of a /Next array are skipped while the other elements are kept.
func TestNextArrayKeepsUsableElements(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	good := pdf.Dict{"S": pdf.Name("Named"), "N": pdf.Name("NextPage")}
	dict := pdf.Dict{
		"S":   pdf.Name("URI"),
		"URI": pdf.String("https://example.com/"),
		"Next": pdf.Array{
			nil,
			pdf.NewReference(99, 0),
			pdf.Integer(3),
			pdf.Dict{"S": pdf.Name("NoSuchAction")},
			good,
		},
	}
	got, err := Decode(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want := &URI{URI: "https://example.com/", Next: ActionList{&Named{N: "NextPage"}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected action (-want +got):\n%s", diff)
	}

	// an array without usable elements reads like an absent entry
	dict["Next"] = pdf.Array{nil, pdf.NewReference(99, 0)}
	got, err = Decode(c, dict, true)
	if err != nil {
		t.Fatal(err)
	}
	want = &URI{URI: "https://example.com/"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected action (-want +got):\n%s", diff)
	}
}
