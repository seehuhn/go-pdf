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

package appearance

import (
	"bytes"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func makeTestAppearance(gray float64) *form.Form {
	b := builder.New(content.Form, nil, pdf.V2_0)
	b.SetFillColor(color.DeviceGray(gray))
	b.Rectangle(0, 0, 24, 24)
	b.Fill()
	return &form.Form{
		Content: &content.Operators{Ops: b.Stream},
		Res:     b.Resources,
		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: 24, URy: 24},
		Matrix:  matrix.Identity,
	}
}

var (
	appA = makeTestAppearance(0.25)
	appB = makeTestAppearance(0.5)
	appC = makeTestAppearance(0.75)
)

type testCase struct {
	name    string
	version pdf.Version
	data    *Dict
}

var testCases = []testCase{
	{
		name:    "streams/V1.7",
		version: pdf.V1_7,
		data: &Dict{
			Normal:   appA,
			RollOver: appB,
			Down:     appC,
		},
	},
	{
		name:    "streams/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:   appA,
			RollOver: appB,
			Down:     appC,
		},
	},
	{
		name:    "single/V1.7",
		version: pdf.V1_7,
		data: &Dict{
			Normal:    appA,
			RollOver:  appB,
			Down:      appC,
			SingleUse: true,
		},
	},
	{
		name:    "single/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:    appA,
			RollOver:  appB,
			Down:      appC,
			SingleUse: true,
		},
	},
	{
		name:    "maps/V1.7",
		version: pdf.V1_7,
		data: &Dict{
			NormalMap: map[pdf.Name]*form.Form{
				"On":  appA,
				"Off": appB,
			},
			RollOverMap: map[pdf.Name]*form.Form{
				"On":  appB,
				"Off": appC,
			},
			DownMap: map[pdf.Name]*form.Form{
				"On":  appC,
				"Off": appA,
			},
		},
	},
	{
		name:    "maps/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			NormalMap: map[pdf.Name]*form.Form{
				"On":  appA,
				"Off": appB,
			},
			RollOverMap: map[pdf.Name]*form.Form{
				"On":  appB,
				"Off": appC,
			},
			DownMap: map[pdf.Name]*form.Form{
				"On":  appC,
				"Off": appA,
			},
		},
	},
	{
		// an annotation which looks the same at rest, under the pointer and
		// while pressed: reading fills the rollover and down appearances in
		// with the normal one, writing leaves them out again
		name:    "normalOnly/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:   appA,
			RollOver: appA,
			Down:     appA,
		},
	},
	{
		name:    "normalMapOnly/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			NormalMap:   normalStates,
			RollOverMap: normalStates,
			DownMap:     normalStates,
		},
	},
	{
		// a rollover of its own, with the down appearance left to the normal
		// one
		name:    "normalAndRollover/V2.0",
		version: pdf.V2_0,
		data: &Dict{
			Normal:   appA,
			RollOver: appB,
			Down:     appA,
		},
	},
}

// normalStates is shared between the entries of the appearance dictionary
// above, the way reading an appearance dictionary with a single /N entry
// shares one map between them.
var normalStates = map[pdf.Name]*form.Form{
	"On":  appA,
	"Off": appB,
}

// TestExtractDefaults checks that entries a file leaves out are filled in with
// the normal appearance, and that the result shares the normal appearance
// rather than holding a second copy of it.  The sharing is what lets Embed
// leave the entries out again, and what tells a rollover the file asks for
// from one substituted here.
func TestExtractDefaults(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	normalRef, err := rm.Embed(appA)
	if err != nil {
		t.Fatal(err)
	}
	rollOverRef, err := rm.Embed(appB)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name         string
		dict         pdf.Dict
		wantRollOver bool // rollover differs from the normal appearance
	}{
		{
			name: "absent",
			dict: pdf.Dict{"N": normalRef},
		},
		{
			name: "malformed",
			dict: pdf.Dict{"N": normalRef, "R": pdf.Integer(42)},
		},
		{
			// a per-state map naming no appearance we can use is as good as
			// no entry at all, and must default to the normal appearance the
			// same way; leaving an empty map behind would also make the
			// annotation require an /AS entry it has no use for
			name: "emptyStates",
			dict: pdf.Dict{"N": normalRef, "R": pdf.Dict{}},
		},
		{
			name: "malformedStates",
			dict: pdf.Dict{"N": normalRef, "R": pdf.Dict{"On": pdf.Integer(42)}},
		},
		{
			// a state with an empty name cannot be selected by an /AS entry,
			// so it names no appearance we can use either
			name: "emptyStateName",
			dict: pdf.Dict{"N": normalRef, "R": pdf.Dict{"": rollOverRef}},
		},
		{
			// the same appearance stream under both keys is the same
			// appearance, not a rollover of its own
			name: "sameStream",
			dict: pdf.Dict{"N": normalRef, "R": normalRef},
		},
		{
			name:         "ownRollover",
			dict:         pdf.Dict{"N": normalRef, "R": rollOverRef},
			wantRollOver: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref := w.Alloc()
			if err := w.Put(ref, tc.dict); err != nil {
				t.Fatal(err)
			}

			x := pdf.NewExtractor(w)
			d, err := pdf.Decode(pdf.CursorAt(x, nil), ref, ExtractDict)
			if err != nil {
				t.Fatal(err)
			}

			if d.Normal == nil {
				t.Fatal("missing normal appearance")
			}
			if got := d.RollOver != d.Normal; got != tc.wantRollOver {
				t.Errorf("rollover differs from normal = %t, want %t", got, tc.wantRollOver)
			}
			if d.Down != d.Normal {
				t.Error("down appearance was not filled in with the normal one")
			}
			// none of these files names an appearance state, so none of them
			// makes the annotation need an /AS entry
			if d.HasDicts() {
				t.Error("a per-state map was left behind")
			}
		})
	}
}

// TestExtractDropsEmptyStateName checks that a state with an empty name is
// left out of the map, while the other states of the same entry are kept.
func TestExtractDropsEmptyStateName(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	onRef, err := rm.Embed(appA)
	if err != nil {
		t.Fatal(err)
	}
	unnamedRef, err := rm.Embed(appB)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	ref := w.Alloc()
	dict := pdf.Dict{"N": pdf.Dict{"On": onRef, "": unnamedRef}}
	if err := w.Put(ref, dict); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	d, err := pdf.Decode(pdf.CursorAt(x, nil), ref, ExtractDict)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := d.NormalMap[""]; ok {
		t.Error("state with an empty name was kept")
	}
	if d.NormalMap["On"] == nil {
		t.Error("named state was lost")
	}
}

// TestEmbedOmitsRepeats checks that an appearance which only repeats the
// normal one is left out of the PDF dictionary, and that one of its own is
// written.
func TestEmbedOmitsRepeats(t *testing.T) {
	otherStates := maps.Clone(normalStates)
	changedStates := maps.Clone(normalStates)
	changedStates["On"] = appC
	extraStates := maps.Clone(normalStates)
	extraStates["Extra"] = appA

	cases := []struct {
		name string
		data *Dict
		want bool // an /R entry is written
	}{
		{
			name: "nilShorthand",
			data: &Dict{Normal: appA, SingleUse: true},
		},
		{
			name: "sameForm",
			data: &Dict{Normal: appA, RollOver: appA, SingleUse: true},
		},
		{
			// equal content is not the same appearance stream
			name: "equalForm",
			data: &Dict{Normal: appA, RollOver: makeTestAppearance(0.25), SingleUse: true},
			want: true,
		},
		{
			name: "otherForm",
			data: &Dict{Normal: appA, RollOver: appB, SingleUse: true},
			want: true,
		},
		{
			// a map of its own listing the same forms for the same states
			// shows the same thing
			name: "sameStates",
			data: &Dict{NormalMap: normalStates, RollOverMap: otherStates, SingleUse: true},
		},
		{
			name: "changedState",
			data: &Dict{NormalMap: normalStates, RollOverMap: changedStates, SingleUse: true},
			want: true,
		},
		{
			name: "extraState",
			data: &Dict{NormalMap: normalStates, RollOverMap: extraStates, SingleUse: true},
			want: true,
		},
		{
			name: "mapAndForm",
			data: &Dict{NormalMap: normalStates, RollOver: appA, SingleUse: true},
			want: true,
		},
		{
			// an empty map names no appearance, so it is the shorthand for the
			// normal appearance just like a nil one
			name: "emptyStates",
			data: &Dict{Normal: appA, RollOverMap: map[pdf.Name]*form.Form{}, SingleUse: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			obj, err := rm.Embed(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if err := rm.Close(); err != nil {
				t.Fatal(err)
			}

			dict, ok := obj.(pdf.Dict)
			if !ok {
				t.Fatalf("expected a dictionary, got %T", obj)
			}
			if dict["N"] == nil {
				t.Error("missing N entry")
			}
			if _, ok := dict["R"]; ok != tc.want {
				t.Errorf("R entry written = %t, want %t", ok, tc.want)
			}
			if _, ok := dict["D"]; ok {
				t.Error("D entry written for a repeated appearance")
			}
		})
	}
}

// TestEmbedRejectsMissingNormal checks that a dictionary without a normal
// appearance is refused, rather than written as a file we cannot read back.
func TestEmbedRejectsMissingNormal(t *testing.T) {
	cases := []struct {
		name string
		data *Dict
	}{
		{name: "nil", data: &Dict{}},
		{name: "emptyMap", data: &Dict{NormalMap: map[pdf.Name]*form.Form{}}},
		{
			name: "emptyMapWithRollOver",
			data: &Dict{NormalMap: map[pdf.Name]*form.Form{}, RollOver: appA},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(tc.data); err == nil {
				t.Error("embed accepted a dictionary without a normal appearance")
			}
		})
	}
}

func roundTripTest(t *testing.T, version pdf.Version, data *Dict) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}
	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.Decode(pdf.CursorAt(x, nil), ref, ExtractDict)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			roundTripTest(t, tc.version, tc.data)
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		ref, err := rm.Embed(tc.data)
		if err != nil {
			continue
		}
		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = ref
		err = w.Close()
		if err != nil {
			continue
		}

		f.Add(buf.Data)
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, _ := pdf.Decode(pdf.CursorAt(x, nil), objPDF, ExtractDict)
		if objGo == nil {
			t.Skip("no appearance dictionary")
		}

		roundTripTest(t, pdf.GetVersion(r), objGo)
	})
}

// TestAnyState checks the choice of appearance state for a dictionary which
// names none: the smallest name wins, and the normal appearance decides where
// it has states of its own.
func TestAnyState(t *testing.T) {
	cases := []struct {
		name string
		dict *Dict
		want pdf.Name // "" if the dictionary does not select by state
	}{
		{name: "nil"},
		{name: "noStates", dict: &Dict{Normal: appA, RollOver: appB}},
		{name: "emptyMap", dict: &Dict{NormalMap: map[pdf.Name]*form.Form{}}},
		{
			// an empty name cannot fill an AS entry, so it is passed over
			name: "emptyStateName",
			dict: &Dict{NormalMap: map[pdf.Name]*form.Form{"": appA}},
		},
		{
			name: "smallestName",
			dict: &Dict{NormalMap: map[pdf.Name]*form.Form{
				"On": appA, "Off": appB, "Mixed": appC,
			}},
			want: "Mixed",
		},
		{
			// the normal appearance decides where it has states
			name: "normalWins",
			dict: &Dict{
				NormalMap:   map[pdf.Name]*form.Form{"On": appA},
				RollOverMap: map[pdf.Name]*form.Form{"Off": appB},
			},
			want: "On",
		},
		{
			// ... and the other entries only get a say where it does not
			name: "rollOverDecides",
			dict: &Dict{
				Normal:      appA,
				RollOverMap: map[pdf.Name]*form.Form{"Off": appB},
			},
			want: "Off",
		},
		{
			name: "downDecides",
			dict: &Dict{
				Normal:  appA,
				DownMap: map[pdf.Name]*form.Form{"Off": appB},
			},
			want: "Off",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if state := tc.dict.AnyState(); state != tc.want {
				t.Errorf("appearance state = %q, want %q", state, tc.want)
			}
		})
	}
}

// TestEmbedRejectsEmptyStateName checks that a state with an empty name is
// refused rather than written out.  Reading drops such a state, since an AS
// entry cannot name it, so writing one would lose it silently.
func TestEmbedRejectsEmptyStateName(t *testing.T) {
	cases := map[string]*Dict{
		"normal":   {NormalMap: map[pdf.Name]*form.Form{"": appA, "On": appB}},
		"rollOver": {Normal: appA, RollOverMap: map[pdf.Name]*form.Form{"": appB}},
		"down":     {Normal: appA, DownMap: map[pdf.Name]*form.Form{"": appB}},
	}

	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			if _, err := rm.Embed(d); err == nil {
				t.Error("expected an error for the empty state name")
			}
		})
	}
}

// TestClone checks that a clone can be changed without disturbing the
// original, while still sharing its appearance streams.
func TestClone(t *testing.T) {
	if (*Dict)(nil).Clone() != nil {
		t.Error("cloning a nil dictionary gave a dictionary")
	}

	// the shape reading a file with a single /N sub-dictionary produces: one
	// map behind all three entries
	orig := &Dict{
		NormalMap:   normalStates,
		RollOverMap: normalStates,
		DownMap:     normalStates,
		SingleUse:   true,
	}
	clone := orig.Clone()

	if !clone.SingleUse {
		t.Error("SingleUse was not carried over")
	}
	if clone.NormalMap["On"] != appA {
		t.Error("the appearance streams were not shared with the original")
	}

	// changing the clone must leave the original alone, in every entry
	clone.NormalMap["On"] = appC
	delete(clone.RollOverMap, "Off")
	clone.DownMap["Extra"] = appC
	if diff := cmp.Diff(normalStates, orig.NormalMap); diff != "" {
		t.Errorf("the original changed with the clone (-want +got):\n%s", diff)
	}
	if len(orig.RollOverMap) != len(normalStates) || len(orig.DownMap) != len(normalStates) {
		t.Error("the original changed with the clone")
	}
}

// TestCloneKeepsRepeats checks that the entries of a clone still count as
// repeating the normal appearance.  Cloning splits the one map reading a file
// shares between the entries, so an implementation comparing the maps by
// identity would start writing /R and /D entries which only repeat /N.
func TestCloneKeepsRepeats(t *testing.T) {
	orig := &Dict{
		NormalMap:   normalStates,
		RollOverMap: normalStates,
		DownMap:     normalStates,
		SingleUse:   true,
	}

	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(orig.Clone())
	if err != nil {
		t.Fatal(err)
	}
	dict, ok := obj.(pdf.Dict)
	if !ok {
		t.Fatalf("expected a dictionary, got %T", obj)
	}
	if _, ok := dict["R"]; ok {
		t.Error("R entry written for a repeated appearance")
	}
	if _, ok := dict["D"]; ok {
		t.Error("D entry written for a repeated appearance")
	}
}
