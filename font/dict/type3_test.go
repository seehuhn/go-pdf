// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package dict

import (
	"testing"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// A resource dictionary shared between glyphs and the font is written to the
// file only once.
func TestType3SharedResources(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	res := &content.Resources{}
	glyph := func() *CharProc {
		return &CharProc{
			Content: &content.Operators{Ops: []content.Operator{
				{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(0)}},
			}},
			Resources: res,
		}
	}
	d := &Type3{
		Descriptor: &font.Descriptor{},
		Encoding:   encodeAt(0x41, 0x42),
		CharProcs:  map[pdf.Name]*CharProc{"A": glyph(), "B": glyph()},
		FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		Resources:  res,
	}
	d.Width[0x41] = 500
	d.Width[0x42] = 500

	fontObj, err := rm.Embed(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	resolve := func(obj pdf.Object) pdf.Native {
		t.Helper()
		native, err := pdf.Resolve(w, obj)
		if err != nil {
			t.Fatal(err)
		}
		return native
	}
	fontDict := resolve(fontObj).(pdf.Dict)
	charProcs := resolve(fontDict["CharProcs"]).(pdf.Dict)
	want := fontDict["Resources"]
	if _, isRef := want.(pdf.Reference); !isRef {
		t.Fatalf("font Resources = %v, want an indirect reference", want)
	}
	for _, name := range []pdf.Name{"A", "B"} {
		stm := resolve(charProcs[name]).(*pdf.Stream)
		if got := stm.Dict["Resources"]; got != want {
			t.Errorf("glyph %s Resources = %v, want %v", name, got, want)
		}
	}
}

// Font-level resources need PDF 1.2, per-glyph resources need PDF 2.0.
func TestType3ResourcesVersion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		version  pdf.Version
		fontRes  bool
		glyphRes bool
		wantErr  bool
	}{
		{"font-1.1", pdf.V1_1, true, false, true},
		{"font-1.2", pdf.V1_2, true, false, false},
		{"glyph-1.7", pdf.V1_7, false, true, true},
		{"glyph-2.0", pdf.V2_0, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, tc.version, nil)
			rm := pdf.NewResourceManager(w)

			cp := &CharProc{
				Content: &content.Operators{Ops: []content.Operator{
					{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(0)}},
				}},
			}
			d := &Type3{
				Name:       "F",
				Descriptor: &font.Descriptor{},
				Encoding:   encodeAt(0x41),
				CharProcs:  map[pdf.Name]*CharProc{"A": cp},
				FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
			}
			d.Width[0x41] = 500
			if tc.fontRes {
				d.Resources = &content.Resources{}
			}
			if tc.glyphRes {
				cp.Resources = &content.Resources{}
			}

			_, err := rm.Embed(d)
			if gotErr := err != nil; gotErr != tc.wantErr {
				t.Errorf("error = %v, want error: %t", err, tc.wantErr)
			}
		})
	}
}

// Glyph content uses the output file's name syntax: names which need #xx
// escapes cannot be written before PDF 1.2.
func TestType3GlyphNameVersion(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_1, pdf.V1_2} {
		w, _ := memfile.NewPDFWriter(t, v, nil)
		rm := pdf.NewResourceManager(w)

		cp := &CharProc{
			Content: &content.Operators{Ops: []content.Operator{
				{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(0)}},
				{Name: content.OpXObject, Args: []pdf.Object{pdf.Name("X 1")}},
			}},
		}
		d := &Type3{
			Name:       "F",
			Descriptor: &font.Descriptor{},
			Encoding:   encodeAt(0x41),
			CharProcs:  map[pdf.Name]*CharProc{"A": cp},
			FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		}
		d.Width[0x41] = 500

		_, err := rm.Embed(d)
		if hasErr := err != nil; hasErr != (v < pdf.V1_2) {
			t.Errorf("PDF %s: unexpected result, err = %v", v, err)
		}
	}
}

// A glyph procedure must start with a well-formed d0 or d1 operator.
func TestType3InvalidGlyph(t *testing.T) {
	op := func(name content.OpName, args ...pdf.Object) content.Operator {
		return content.Operator{Name: name, Args: args}
	}
	cases := map[string]content.Stream{
		"nil":   nil,
		"empty": &content.Operators{},
		"no metrics": &content.Operators{Ops: []content.Operator{
			op(content.OpMoveTo, pdf.Integer(0), pdf.Integer(0)),
		}},
		"wy": &content.Operators{Ops: []content.Operator{
			op(content.OpType3ColoredGlyph, pdf.Integer(500), pdf.Integer(1)),
		}},
		"short d1": &content.Operators{Ops: []content.Operator{
			op(content.OpType3UncoloredGlyph, pdf.Integer(500), pdf.Integer(0)),
		}},
	}
	for name, stm := range cases {
		t.Run(name, func(t *testing.T) {
			w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
			rm := pdf.NewResourceManager(w)
			d := &Type3{
				Descriptor: &font.Descriptor{},
				Encoding:   encodeAt(0x41),
				CharProcs:  map[pdf.Name]*CharProc{"A": {Content: stm}},
				FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
			}
			if _, err := rm.Embed(d); err == nil {
				t.Error("invalid glyph procedure was accepted")
			}
		})
	}
}

// A glyph procedure shared by several glyph names, and by several fonts, is
// written to the file only once.
func TestType3SharedCharProc(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w)

	cp := &CharProc{
		Content: &content.Operators{Ops: []content.Operator{
			{Name: content.OpType3ColoredGlyph, Args: []pdf.Object{pdf.Integer(500), pdf.Integer(0)}},
		}},
	}
	newFont := func(charProcs map[pdf.Name]*CharProc) *Type3 {
		return &Type3{
			Descriptor: &font.Descriptor{},
			Encoding:   encodeAt(0x41),
			CharProcs:  charProcs,
			FontMatrix: matrix.Matrix{0.001, 0, 0, 0.001, 0, 0},
		}
	}
	var fontObjs []pdf.Native
	for _, d := range []*Type3{
		newFont(map[pdf.Name]*CharProc{"A": cp, "B": cp}),
		newFont(map[pdf.Name]*CharProc{"A": cp}),
	} {
		obj, err := rm.Embed(d)
		if err != nil {
			t.Fatal(err)
		}
		fontObjs = append(fontObjs, obj)
	}
	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}

	var refs []pdf.Object
	for _, fontObj := range fontObjs {
		fontDict, err := pdf.Resolve(w, fontObj)
		if err != nil {
			t.Fatal(err)
		}
		charProcs, err := pdf.Resolve(w, fontDict.(pdf.Dict)["CharProcs"])
		if err != nil {
			t.Fatal(err)
		}
		for _, obj := range charProcs.(pdf.Dict) {
			refs = append(refs, obj)
		}
	}
	if len(refs) != 3 {
		t.Fatalf("got %d glyph procedures, want 3", len(refs))
	}
	for _, ref := range refs[1:] {
		if ref != refs[0] {
			t.Errorf("glyph procedure written more than once: %v", refs)
			break
		}
	}
}
