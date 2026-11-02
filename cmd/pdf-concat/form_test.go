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

package main

import (
	"path/filepath"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/pagetree"
)

func TestRewriteDA(t *testing.T) {
	rename := resRename{
		"Font":       {"Helv": "Helv_2", "A#00B": "A#00B_2"},
		"ExtGState":  {"GS0": "GS0_2"},
		"ColorSpace": {"DeviceRGB": "DeviceRGB_2", "CS0": "CS0_2"},
	}
	outputName := func(category, name pdf.Name) pdf.Name {
		if out, ok := rename[category][name]; ok {
			return out
		}
		return name
	}
	cases := []struct {
		in, want string
	}{
		{"/Helv 12 Tf 0 g", "/Helv_2 12 Tf 0 g"},             // basic substitution
		{"/Helvetica 12 Tf", "/Helvetica 12 Tf"},             // prefix must not match
		{"/Cour 12 Tf", "/Cour 12 Tf"},                       // unrelated font untouched
		{"/He#6cv 12 Tf 0 g", "/Helv_2 12 Tf 0 g"},           // escaped name matches
		{"0 0 1 rg /Helv 10 Tf", "0 0 1 rg /Helv_2 10 Tf"},   // name after color ops
		{"/A#00B 12 Tf", "/A#2300B_2 12 Tf"},                 // #00 is not an escape
		{"0 0 1 rg/Helv 12 Tf", "0 0 1 rg /Helv_2 12 Tf"},    // delimiter ends a token
		{"/Helv 12 Tf\x000 g", "/Helv_2 12 Tf 0 g"},          // NUL is white space
		{"%/Helv\n/Helv 12 Tf", "/Helv_2 12 Tf"},             // comments are dropped
		{"/Helv cs /Helv 12 Tf", "/Helv cs /Helv_2 12 Tf"},   // categories are separate
		{"/GS0 gs /Helv 12 Tf", "/GS0_2 gs /Helv_2 12 Tf"},   // other categories
		{"/GS0 Tf", "/GS0 Tf"},                               // GS0 is no font
		{"Tf /Helv 12 Tf", "Tf /Helv_2 12 Tf"},               // no operands
		{"1 0 0 scn /Helv 12 Tf", "1 0 0 scn /Helv_2 12 Tf"}, // no pattern name
		{"/Tag BDC /Helv 12 Tf", "/Tag BDC /Helv_2 12 Tf"},   // too few operands
		{"/DeviceRGB cs /CS0 CS", "/DeviceRGB cs /CS0_2 CS"}, // families are no resources
	}
	for _, tc := range cases {
		da, _, err := rewriteDA(pdf.String(tc.in), outputName, 0)
		if err != nil {
			t.Errorf("rewriteDA(%q): %v", tc.in, err)
			continue
		}
		if got := string(da); got != tc.want {
			t.Errorf("rewriteDA(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestRewriteDANoRename(t *testing.T) {
	da := pdf.String("/Helv  12 Tf %comment\n0 g")
	same := func(_, name pdf.Name) pdf.Name { return name }
	if got, changed, err := rewriteDA(da, same, 0); err != nil || changed || string(got) != string(da) {
		t.Errorf("rewriteDA with no renames = %q, %v, %v, want unchanged", got, changed, err)
	}
}

// formOptions configures the input file written by makeNestedFormPDF.
type formOptions struct {
	da pdf.Object // the document-wide /DA, or nil
	q  pdf.Object // the document-wide /Q, or nil

	// baseFont is the font behind /Helv, Helvetica if empty
	baseFont pdf.Name

	// linkAnnot adds /DR /Properties /MC0, a dictionary which refers to
	// the FreeText annotation
	linkAnnot bool

	// fixedContents gives the FreeText annotation the same contents
	// whatever the base font, so that it differs between inputs only in
	// what its /DA names
	fixedContents bool

	// noFont leaves /Font out of /DR, so that the /DA strings name a
	// missing font
	noFont bool

	// secondFont adds /Helv_2, a Times-Roman font, to /DR /Font
	secondFont bool

	// sigFlags is the /SigFlags entry, or nil
	sigFlags pdf.Object
}

// makeNestedFormPDF writes a one-page PDF whose form has default resources
// /Font /Helv and /ExtGState /GS0, and a root field "name" with one kid, a
// text field merged with its widget, whose own /DA names both resources.  The
// page also has a FreeText annotation, whose /DA names /Helv and whose
// contents is the base font name, unless opt.fixedContents is set.
func makeNestedFormPDF(t *testing.T, path string, opt formOptions) {
	t.Helper()
	baseFont := opt.baseFont
	if baseFont == "" {
		baseFont = "Helvetica"
	}
	contents := pdf.String(baseFont)
	if opt.fixedContents {
		contents = pdf.String("text")
	}
	w, err := pdf.Create(path, pdf.V1_7, nil)
	if err != nil {
		t.Fatal(err)
	}

	pagesRef := w.Alloc()
	pageRef := w.Alloc()
	fontRef := w.Alloc()
	gsRef := w.Alloc()
	rootRef := w.Alloc()
	kidRef := w.Alloc()
	freeTextRef := w.Alloc()
	acroRef := w.Alloc()

	objects := map[pdf.Reference]pdf.Native{
		pagesRef: pdf.Dict{
			"Type":  pdf.Name("Pages"),
			"Kids":  pdf.Array{pageRef},
			"Count": pdf.Integer(1),
		},
		pageRef: pdf.Dict{
			"Type":     pdf.Name("Page"),
			"Parent":   pagesRef,
			"MediaBox": pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(200), pdf.Integer(200)},
			"Annots":   pdf.Array{kidRef, freeTextRef},
		},
		fontRef: pdf.Dict{
			"Type":     pdf.Name("Font"),
			"Subtype":  pdf.Name("Type1"),
			"BaseFont": baseFont,
		},
		gsRef: pdf.Dict{
			"Type": pdf.Name("ExtGState"),
			"CA":   pdf.Number(0.5),
		},
		rootRef: pdf.Dict{
			"T":    pdf.String("name"),
			"Kids": pdf.Array{kidRef},
		},
		kidRef: pdf.Dict{
			"FT":      pdf.Name("Tx"),
			"T":       pdf.String("kid"),
			"Parent":  rootRef,
			"Type":    pdf.Name("Annot"),
			"Subtype": pdf.Name("Widget"),
			"Rect":    pdf.Array{pdf.Integer(10), pdf.Integer(10), pdf.Integer(100), pdf.Integer(30)},
			"P":       pageRef,
			"DA":      pdf.String("/Helv 10 Tf /GS0 gs 0 g"),
		},
		freeTextRef: pdf.Dict{
			"Type":     pdf.Name("Annot"),
			"Subtype":  pdf.Name("FreeText"),
			"Rect":     pdf.Array{pdf.Integer(10), pdf.Integer(50), pdf.Integer(100), pdf.Integer(80)},
			"Contents": contents,
			"DA":       pdf.String("/Helv 12 Tf 0 0 1 rg"),
		},
	}
	acro := pdf.Dict{
		"Fields": pdf.Array{rootRef},
		"DR": pdf.Dict{
			"Font":      pdf.Dict{"Helv": fontRef},
			"ExtGState": pdf.Dict{"GS0": gsRef},
		},
	}
	if opt.secondFont {
		fonts := acro["DR"].(pdf.Dict)["Font"].(pdf.Dict)
		fonts["Helv_2"] = pdf.Dict{
			"Type":     pdf.Name("Font"),
			"Subtype":  pdf.Name("Type1"),
			"BaseFont": pdf.Name("Times-Roman"),
		}
	}
	if opt.noFont {
		delete(acro["DR"].(pdf.Dict), "Font")
	}
	if opt.sigFlags != nil {
		acro["SigFlags"] = opt.sigFlags
	}
	if opt.linkAnnot {
		acro["DR"].(pdf.Dict)["Properties"] = pdf.Dict{
			"MC0": pdf.Dict{"Annot": freeTextRef},
		}
	}
	if opt.da != nil {
		acro["DA"] = opt.da
	}
	if opt.q != nil {
		acro["Q"] = opt.q
	}
	objects[acroRef] = acro
	for ref, obj := range objects {
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
	}

	meta := w.GetMeta()
	meta.Catalog.Pages = pagesRef
	meta.Catalog.AcroForm = acroRef
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// concatForms concatenates input files made by makeNestedFormPDF with the
// given options and opens the result.
func concatForms(t *testing.T, opts ...formOptions) *pdf.Reader {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "out.pdf")
	c, err := NewConcat(out, pdf.V1_7)
	if err != nil {
		t.Fatal(err)
	}
	for i, opt := range opts {
		in := filepath.Join(dir, string(rune('a'+i))+".pdf")
		makeNestedFormPDF(t, in, opt)
		if err := c.Append(in); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := pdf.Open(out, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}

// TestConcatMergesDR checks how the default resources of the inputs are
// merged, and that a resource renamed in the process is renamed in every
// /DA copied from that input: on a nested field merged with its widget and
// on a FreeText annotation.  The first input keeps its names.
func TestConcatMergesDR(t *testing.T) {
	const (
		widgetDA   = "/Helv 10 Tf /GS0 gs 0 g"
		freeTextDA = "/Helv 12 Tf 0 0 1 rg"
		widgetDA2  = "/Helv_2 10 Tf /GS0 gs 0 g"
		freeText2  = "/Helv_2 12 Tf 0 0 1 rg"
	)
	cases := []struct {
		name string
		opts []formOptions
		da   [][2]string             // the widget's and the FreeText /DA, by page
		dr   map[pdf.Name][]pdf.Name // the output's /DR names, by category
	}{
		{
			name: "equal resources shared",
			opts: []formOptions{{}, {}},
			da:   [][2]string{{widgetDA, freeTextDA}, {widgetDA, freeTextDA}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":      {"Helv"},
				"ExtGState": {"GS0"},
			},
		},
		{
			name: "different resource renamed",
			opts: []formOptions{{}, {baseFont: "Courier"}},
			da:   [][2]string{{widgetDA, freeTextDA}, {widgetDA2, freeText2}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":      {"Helv", "Helv_2"},
				"ExtGState": {"GS0"},
			},
		},
		{
			name: "shared under a new name",
			opts: []formOptions{{}, {baseFont: "Courier"}, {baseFont: "Courier"}},
			da: [][2]string{
				{widgetDA, freeTextDA},
				{widgetDA2, freeText2},
				{widgetDA2, freeText2},
			},
			dr: map[pdf.Name][]pdf.Name{
				"Font":      {"Helv", "Helv_2"},
				"ExtGState": {"GS0"},
			},
		},
		{
			// the second FreeText annotation is copied as part of /DR
			name: "DA reached from DR",
			opts: []formOptions{{linkAnnot: true}, {baseFont: "Courier", linkAnnot: true}},
			da:   [][2]string{{widgetDA, freeTextDA}, {widgetDA2, freeText2}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":       {"Helv", "Helv_2"},
				"ExtGState":  {"GS0"},
				"Properties": {"MC0", "MC0_2"},
			},
		},
		{
			// the two /MC0 entries are the same objects, but their /DA
			// names different fonts
			name: "DA reached from DR names renamed font",
			opts: []formOptions{
				{linkAnnot: true, fixedContents: true},
				{baseFont: "Courier", linkAnnot: true, fixedContents: true},
			},
			da: [][2]string{{widgetDA, freeTextDA}, {widgetDA2, freeText2}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":       {"Helv", "Helv_2"},
				"ExtGState":  {"GS0"},
				"Properties": {"MC0", "MC0_2"},
			},
		},
		{
			// the fresh name for /Helv avoids the input's own /Helv_2
			name: "fresh name avoids own name",
			opts: []formOptions{{}, {baseFont: "Courier", secondFont: true}},
			da: [][2]string{
				{widgetDA, freeTextDA},
				{"/Helv_3 10 Tf /GS0 gs 0 g", "/Helv_3 12 Tf 0 0 1 rg"},
			},
			dr: map[pdf.Name][]pdf.Name{
				"Font":      {"Helv", "Helv_2", "Helv_3"},
				"ExtGState": {"GS0"},
			},
		},
		{
			name: "font missing from later input",
			opts: []formOptions{{}, {noFont: true}},
			da:   [][2]string{{widgetDA, freeTextDA}, {widgetDA2, freeText2}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":      {"Helv"},
				"ExtGState": {"GS0"},
			},
		},
		{
			name: "font missing from earlier input",
			opts: []formOptions{{noFont: true}, {}},
			da:   [][2]string{{widgetDA, freeTextDA}, {widgetDA2, freeText2}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":      {"Helv_2"},
				"ExtGState": {"GS0"},
			},
		},
		{
			name: "font missing from both inputs",
			opts: []formOptions{{noFont: true}, {noFont: true}},
			da:   [][2]string{{widgetDA, freeTextDA}, {widgetDA, freeTextDA}},
			dr: map[pdf.Name][]pdf.Name{
				"ExtGState": {"GS0"},
			},
		},
		{
			name: "DA reached from DR shared",
			opts: []formOptions{
				{linkAnnot: true, fixedContents: true},
				{linkAnnot: true, fixedContents: true},
			},
			da: [][2]string{{widgetDA, freeTextDA}, {widgetDA, freeTextDA}},
			dr: map[pdf.Name][]pdf.Name{
				"Font":       {"Helv"},
				"ExtGState":  {"GS0"},
				"Properties": {"MC0"},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := concatForms(t, c.opts...)
			cur := pdf.NewCursor(r)

			i := 0
			for _, page := range pagetree.NewIterator(r).All() {
				annots, err := cur.Array(page["Annots"])
				if err != nil || len(annots) != 2 {
					t.Fatalf("page %d: annotations %v, %v", i+1, annots, err)
				}
				for j, annot := range annots {
					dict, err := cur.Dict(annot)
					if err != nil {
						t.Fatal(err)
					}
					da, err := cur.String(dict["DA"])
					if err != nil {
						t.Fatal(err)
					}
					if string(da) != c.da[i][j] {
						t.Errorf("page %d, annotation %d: DA = %q, want %q", i+1, j, da, c.da[i][j])
					}
				}
				i++
			}
			if i != len(c.opts) {
				t.Fatalf("got %d pages, want %d", i, len(c.opts))
			}

			acro, err := cur.Dict(r.GetMeta().Catalog.AcroForm)
			if err != nil {
				t.Fatal(err)
			}
			dr, err := cur.Dict(acro["DR"])
			if err != nil {
				t.Fatal(err)
			}
			if len(dr) != len(c.dr) {
				t.Errorf("DR categories = %v, want %v", dr, c.dr)
			}
			for cat, names := range c.dr {
				res, err := cur.Dict(dr[cat])
				if err != nil {
					t.Fatal(err)
				}
				if len(res) != len(names) {
					t.Errorf("DR %s = %v, want %v", cat, res, names)
				}
				for _, name := range names {
					if res[name] == nil {
						t.Errorf("DR %s lacks %s", cat, name)
					}
				}
			}
		})
	}
}

// TestConcatFormDefaults checks how the document-wide /DA and /Q of the
// inputs are merged: kept in the interactive form dictionary where all
// inputs agree, and otherwise set on each input's root fields.
func TestConcatFormDefaults(t *testing.T) {
	type want struct {
		formDA, formQ pdf.Object   // in the output's interactive form
		rootDA, rootQ []pdf.Object // on the root field of each input
	}
	cases := []struct {
		name string
		opts []formOptions
		want want
	}{
		{
			name: "agree",
			opts: []formOptions{
				{da: pdf.String("/ZaDb 0 Tf 0 g"), q: pdf.Integer(1)},
				{da: pdf.String("/ZaDb 0 Tf 0 g"), q: pdf.Integer(1)},
			},
			want: want{
				formDA: pdf.String("/ZaDb 0 Tf 0 g"),
				formQ:  pdf.Integer(1),
				rootDA: []pdf.Object{nil, nil},
				rootQ:  []pdf.Object{nil, nil},
			},
		},
		{
			name: "renamed font",
			opts: []formOptions{
				{da: pdf.String("/Helv 0 Tf 0 g")},
				{da: pdf.String("/Helv 0 Tf 0 g"), baseFont: "Courier"},
			},
			want: want{
				rootDA: []pdf.Object{pdf.String("/Helv 0 Tf 0 g"), pdf.String("/Helv_2 0 Tf 0 g")},
				rootQ:  []pdf.Object{nil, nil},
			},
		},
		{
			name: "one missing",
			opts: []formOptions{
				{q: pdf.Integer(2)},
				{da: pdf.String("/ZaDb 0 Tf 0 g")},
			},
			want: want{
				rootDA: []pdf.Object{nil, pdf.String("/ZaDb 0 Tf 0 g")},
				rootQ:  []pdf.Object{pdf.Integer(2), nil},
			},
		},
		{
			name: "invalid Q dropped",
			opts: []formOptions{
				{q: pdf.Integer(7)},
				{},
			},
			want: want{
				rootDA: []pdf.Object{nil, nil},
				rootQ:  []pdf.Object{nil, nil},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := concatForms(t, c.opts...)
			cur := pdf.NewCursor(r)

			acro, err := cur.Dict(r.GetMeta().Catalog.AcroForm)
			if err != nil {
				t.Fatal(err)
			}
			if !objectsEqual(t, r, acro["DA"], c.want.formDA) {
				t.Errorf("form DA = %v, want %v", acro["DA"], c.want.formDA)
			}
			if !objectsEqual(t, r, acro["Q"], c.want.formQ) {
				t.Errorf("form Q = %v, want %v", acro["Q"], c.want.formQ)
			}

			fields, err := cur.Array(acro["Fields"])
			if err != nil || len(fields) != len(c.opts) {
				t.Fatalf("fields = %v, %v", fields, err)
			}
			for i, ref := range fields {
				root, err := cur.Dict(ref)
				if err != nil {
					t.Fatal(err)
				}
				if !objectsEqual(t, r, root["DA"], c.want.rootDA[i]) {
					t.Errorf("root field %d: DA = %v, want %v", i, root["DA"], c.want.rootDA[i])
				}
				if !objectsEqual(t, r, root["Q"], c.want.rootQ[i]) {
					t.Errorf("root field %d: Q = %v, want %v", i, root["Q"], c.want.rootQ[i])
				}
			}
		})
	}
}

// TestConcatSigFlags checks that the /SigFlags of the inputs are combined,
// keeping only the defined bits, and that a negative value is dropped.
func TestConcatSigFlags(t *testing.T) {
	cases := []struct {
		name string
		in   []pdf.Object
		want pdf.Object
	}{
		{"combined", []pdf.Object{pdf.Integer(1), pdf.Integer(2)}, pdf.Integer(3)},
		{"one input", []pdf.Object{nil, pdf.Integer(1)}, pdf.Integer(1)},
		{"reserved bits dropped", []pdf.Object{pdf.Integer(6), nil}, pdf.Integer(2)},
		{"negative dropped", []pdf.Object{pdf.Integer(-1), nil}, nil},
		{"none", []pdf.Object{nil, nil}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := make([]formOptions, len(c.in))
			for i, sf := range c.in {
				opts[i].sigFlags = sf
			}
			r := concatForms(t, opts...)
			acro, err := pdf.NewCursor(r).Dict(r.GetMeta().Catalog.AcroForm)
			if err != nil {
				t.Fatal(err)
			}
			if !objectsEqual(t, r, acro["SigFlags"], c.want) {
				t.Errorf("SigFlags = %v, want %v", acro["SigFlags"], c.want)
			}
		})
	}
}

// objectsEqual reports whether got, resolved in r, equals want.
func objectsEqual(t *testing.T, r pdf.Getter, got, want pdf.Object) bool {
	t.Helper()
	resolved, err := pdf.Resolve(r, got)
	if err != nil {
		t.Fatal(err)
	}
	if resolved == nil || want == nil {
		return resolved == nil && want == nil
	}
	return pdf.Equal(resolved, want.AsPDF(0))
}

// TestDARenamerPatch checks that daRenamer rewrites /DA entries in direct
// dictionaries nested in arrays, resolves a /DA given as a reference, and
// leaves the source objects alone.
func TestDARenamerPatch(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	daRef := w.Alloc()
	if err := w.Put(daRef, pdf.String("/Helv 9 Tf")); err != nil {
		t.Fatal(err)
	}
	g := newDARenamer(w, &Concat{w: w})
	g.dr = map[pdf.Name]pdf.Dict{"Font": {"Helv": pdf.Reference(1)}}
	g.rename = resRename{"Font": {"Helv": "Helv_2"}}

	direct := pdf.Dict{"Subtype": pdf.Name("FreeText"), "DA": pdf.String("/Helv 12 Tf")}
	indirect := pdf.Dict{"Subtype": pdf.Name("FreeText"), "DA": daRef}
	page := pdf.Dict{"Annots": pdf.Array{direct, indirect}}

	patched, changed, err := g.patchDict(page)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("page not changed")
	}
	annots := patched["Annots"].(pdf.Array)
	for i, want := range []string{"/Helv_2 12 Tf", "/Helv_2 9 Tf"} {
		da := annots[i].(pdf.Dict)["DA"]
		if s, ok := da.(pdf.String); !ok || string(s) != want {
			t.Errorf("annotation %d: DA = %v, want %q", i, da, want)
		}
	}

	if string(direct["DA"].(pdf.String)) != "/Helv 12 Tf" || indirect["DA"] != daRef {
		t.Error("source objects were modified")
	}

	// nothing to rename: the object is returned as it is
	other := pdf.Dict{"Annots": pdf.Array{pdf.Dict{"DA": pdf.String("/Cour 9 Tf")}}}
	if _, changed, err := g.patchDict(other); err != nil || changed {
		t.Errorf("unrelated DA: changed = %v, err = %v", changed, err)
	}
}

// TestDARenamerPatchOrder checks that the names reserved while patching a
// dictionary with several /DA strings do not depend on map iteration order.
func TestDARenamerPatchOrder(t *testing.T) {
	field := pdf.Dict{
		"DA":   pdf.String("/F 9 Tf"),
		"Kids": pdf.Array{pdf.Dict{"DA": pdf.String("/F_2 9 Tf")}},
	}
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	for range 20 {
		c := &Concat{w: w}
		c.category("Font").taken["F"] = true // a resource of an earlier input
		g := newDARenamer(nil, c)

		patched, _, err := g.patchDict(field)
		if err != nil {
			t.Fatal(err)
		}
		da := patched["DA"].(pdf.String)
		kidDA := patched["Kids"].(pdf.Array)[0].(pdf.Dict)["DA"].(pdf.String)
		if string(da) != "/F_2 9 Tf" || string(kidDA) != "/F_2_2 9 Tf" {
			t.Fatalf("DA = %q, kid DA = %q, want \"/F_2 9 Tf\", \"/F_2_2 9 Tf\"", da, kidDA)
		}
	}
}

// TestDARenamerOutputName checks that names missing from an input's /DR are
// reserved in the merged /DR, and that inputs lacking the same name share
// its reservation.
func TestDARenamerOutputName(t *testing.T) {
	c := &Concat{}
	c.category("Font").taken["Helv"] = true // a resource of an earlier input

	g1 := newDARenamer(nil, c)
	g1.dr = map[pdf.Name]pdf.Dict{"Font": {"Own": pdf.Reference(1)}}
	c.category("Font").taken["Own"] = true // as mergeDR records it
	g2 := newDARenamer(nil, c)

	cases := []struct {
		g          *daRenamer
		name, want pdf.Name
	}{
		{g1, "Own", "Own"},     // a resource of the input
		{g1, "Helv", "Helv_2"}, // missing, name taken by a resource
		{g1, "Helv", "Helv_2"}, // the same again
		{g1, "ZaDb", "ZaDb"},   // missing, name free
		{g2, "ZaDb", "ZaDb"},   // missing from a second input
		{g2, "Helv", "Helv_2"}, // missing from a second input
		{g2, "Own", "Own_2"},   // name reserved by the first input
	}
	for _, tc := range cases {
		if got := tc.g.outputName("Font", tc.name); got != tc.want {
			t.Errorf("outputName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
	if res := c.dr["Font"].res; len(res) != 0 {
		t.Errorf("reserved names have resources: %v", res)
	}
}
