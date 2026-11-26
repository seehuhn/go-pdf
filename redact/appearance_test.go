// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
)

// The appearance streams below have the bounding box (0 0 100 20) and are
// shown at annotation rectangles of width 50 and height 20.  The text
// "APSECRET" lies outside the box, at a height of 60 above the rectangle,
// where apRegion covers it for an annotation at apRect.
const (
	apVisible = "BT /F1 12 Tf 2 2 Td (Visible) Tj ET"
	apSecret  = "BT /F1 12 Tf 2 2 Td (Visible) Tj 0 60 Td (APSECRET) Tj ET"
)

var (
	apRect   = rect(10, 200, 60, 220)
	apRegion = rect(0, 250, 100, 280)
)

// apForm writes an appearance stream with the given content.
func apForm(b *nestBuilder, data string) pdf.Reference {
	return b.form(pdf.Dict{"BBox": rect(0, 0, 100, 20), "Resources": helv()}, data)
}

// stamp returns a stamp annotation with the rectangle r and the appearance
// dictionary ap.
func stamp(r pdf.Array, ap pdf.Dict) pdf.Dict {
	return pdf.Dict{
		"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Stamp"),
		"Rect": r, "AP": ap,
	}
}

// isStamp selects the stamp annotations for [pageAnnot].
func isStamp(d pdf.Dict) bool { return d["Subtype"] == pdf.Name("Stamp") }

// apData returns the decoded data of the appearance stream which the keys
// select in the annotation dictionary annot, or nil if there is no stream.
func apData(t testing.TB, r *pdf.Reader, annot pdf.Dict, keys ...pdf.Name) []byte {
	t.Helper()
	stm, err := pdf.Optional(pdf.CursorAt(pdf.NewExtractor(r), nil).Stream(path(t, r, annot, keys...)))
	if err != nil {
		t.Fatal(err)
	}
	if stm == nil {
		return nil
	}
	data, err := pdf.ReadAll(r, nil, stm, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// checkAP checks that the appearance stream which the keys select shows
// the visible text, and not the secret text.
func checkAP(t testing.TB, r *pdf.Reader, annot pdf.Dict, keys ...pdf.Name) {
	t.Helper()
	data := apData(t, r, annot, keys...)
	if data == nil {
		t.Errorf("appearance %v is missing", keys)
		return
	}
	if !bytes.Contains(data, []byte("Visible")) {
		t.Errorf("appearance %v lost the visible text", keys)
	}
	if bytes.Contains(data, []byte("APSECRET")) {
		t.Errorf("appearance %v shows the text in the region", keys)
	}
}

func TestAppearanceMeetsRegion(t *testing.T) {
	// a kept annotation whose appearance stream paints text in a region
	// shows a redacted copy of the stream
	for _, direct := range []bool{false, true} {
		in := nestDoc(t, []pdf.Dict{{"Rect": apRegion}}, func(b *nestBuilder) []nestPage {
			var annot pdf.Object = stamp(apRect, pdf.Dict{"N": apForm(b, apSecret)})
			if !direct {
				annot = b.put(annot)
			}
			return []nestPage{{content: "0 0 1 1 re f", annots: []pdf.Object{annot}}}
		})
		out := writeAndRead(t, in, nil, "")
		checkAP(t, out, pageAnnot(t, out, isStamp), "AP", "N")
		if n := streamsContaining(t, out, "APSECRET"); n != 0 {
			t.Errorf("direct=%t: %d streams hold the text in the region", direct, n)
		}
	}
}

func TestAppearanceStates(t *testing.T) {
	// all appearance entries and states are analysed, and each is
	// replaced by its own copy
	in := nestDoc(t, []pdf.Dict{{"Rect": apRegion}}, func(b *nestBuilder) []nestPage {
		on, off := apForm(b, apSecret), apForm(b, apVisible)
		ap := pdf.Dict{
			"N": b.put(pdf.Dict{"On": on, "Off": off}),
			"R": apForm(b, apSecret),
			"D": pdf.Dict{"On": on},
		}
		a := stamp(apRect, ap)
		a["AS"] = pdf.Name("On")
		return []nestPage{{content: "0 0 1 1 re f", annots: []pdf.Object{b.put(a)}}}
	})
	out := writeAndRead(t, in, nil, "")
	annot := pageAnnot(t, out, isStamp)
	checkAP(t, out, annot, "AP", "N", "On")
	checkAP(t, out, annot, "AP", "N", "Off")
	checkAP(t, out, annot, "AP", "R")
	checkAP(t, out, annot, "AP", "D", "On")
	if n := streamsContaining(t, out, "APSECRET"); n != 0 {
		t.Errorf("%d streams hold the text in the region", n)
	}
}

func TestAppearanceAliasOnPlainPage(t *testing.T) {
	// the first page paints a form under a region; an annotation on the
	// second page shows the form unchanged in all its appearance entries
	var region pdf.Dict
	build := func(regs []pdf.Dict) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			fm := b.form(pdf.Dict{"BBox": rect(0, 0, 250, 50), "Resources": helv()}, secretText)
			a := stamp(rect(10, 10, 60, 30), pdf.Dict{"N": fm, "D": pdf.Dict{"On": fm}})
			a["AS"] = pdf.Name("On")
			return []nestPage{
				{content: "q 1 0 0 1 20 100 cm /Fm1 Do Q", res: pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}}},
				{content: "0 0 1 1 re f", annots: []pdf.Object{b.put(a)}},
			}
		})
	}
	before := pageGlyphs(t, build(nil))
	pos := bytes.Index([]byte(glyphString(before)), []byte("Secret"))
	region = regionOver(before[pos : pos+6])
	out := writeAndRead(t, build([]pdf.Dict{region}), nil, "")

	if bytes.Contains([]byte(glyphString(pageGlyphs(t, out))), []byte("Secret")) {
		t.Error("page with regions still shows the text")
	}
	annots, err := pdf.Resolve(out, pageAt(t, out, 1)["Annots"])
	if err != nil {
		t.Fatal(err)
	}
	annot := resolveDict(t, out, annots.(pdf.Array)[0])
	for _, keys := range [][]pdf.Name{{"AP", "N"}, {"AP", "D", "On"}} {
		if !bytes.Contains(apData(t, out, annot, keys...), []byte("Secret")) {
			t.Errorf("appearance %v lost the text", keys)
		}
	}
}

func TestAppearanceSharedByPages(t *testing.T) {
	// an annotation listed on two pages with regions avoids the regions of
	// both pages
	in := pagesDoc(t, 2, func(w *pdf.Writer, refs []pdf.Reference, pages []pdf.Dict) {
		b := &nestBuilder{t: t, w: w}
		a := b.put(stamp(apRect, pdf.Dict{"N": apForm(b, apSecret)}))
		for i, pg := range pages {
			pg["Contents"] = b.stream(pdf.Dict{}, "0 0 1 1 re f")
			pg["Annots"] = pdf.Array{a}
			r := rect(250, 250, 260, 260)
			if i == 1 {
				r = apRegion
			}
			addRedaction(t, w, refs[i], pg, r)
		}
	})
	out := writeAndRead(t, in, nil, "")
	checkAP(t, out, pageAnnot(t, out, isStamp), "AP", "N")
	if n := streamsContaining(t, out, "APSECRET"); n != 0 {
		t.Errorf("%d streams hold the text in the region", n)
	}
}

func TestAppearanceWidget(t *testing.T) {
	// the value of a signature field is removed, so that the form code
	// writes the widget; its appearance is rewritten all the same
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		ref := b.w.Alloc()
		stm, err := b.w.OpenStream(ref, pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
			"BBox":      rect(0, 0, 100, 20),
			"Resources": pdf.Dict{"Font": pdf.Dict{"Helv": b.font}},
		})
		if err != nil {
			t.Fatal(err)
		}
		// at the rectangle (10 10 60 30), the second text lies in the region
		stm.Write([]byte("BT /Helv 12 Tf 2 2 Td (Visible) Tj 100 85 Td (APSECRET) Tj ET"))
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		sig := b.put(b.widget(outside, "sig", pdf.Dict{
			"FT": pdf.Name("Sig"), "T": pdf.String("sign"),
			"V":  pdf.Dict{"Type": pdf.Name("Sig"), "Name": pdf.String("SIGVALUE")},
			"AP": pdf.Dict{"N": ref},
		}))
		b.form(sig)
		return pdf.Array{sig}
	})
	o := redactForm(t, in)
	o.lacks("SIGVALUE")
	o.lacks("APSECRET")
	w := o.annot("sig")
	if w == nil {
		t.Fatal("signature widget was removed")
	}
	if !bytes.Contains(o.normal(w), []byte("Visible")) {
		t.Error("appearance of the widget lost the visible text")
	}
}

func TestAppearanceWithoutArea(t *testing.T) {
	// an appearance stream whose bounding box encloses no area cannot be
	// placed, and is removed on a page with regions
	in := nestDoc(t, []pdf.Dict{{"Rect": apRegion}}, func(b *nestBuilder) []nestPage {
		fm := b.form(pdf.Dict{"BBox": rect(0, 0, 0, 20), "Resources": helv()}, apSecret)
		return []nestPage{{
			content: "0 0 1 1 re f",
			annots:  []pdf.Object{b.put(stamp(apRect, pdf.Dict{"N": fm}))},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	annot := pageAnnot(t, out, isStamp)
	if annot["AP"] != nil {
		t.Error("appearance without area was kept")
	}
	if n := streamsContaining(t, out, "APSECRET"); n != 0 {
		t.Errorf("%d streams hold the text", n)
	}
}

func TestAppearanceBrokenStream(t *testing.T) {
	// an appearance stream which cannot be decoded completely is replaced
	// by a copy of the decoded part, on a page with regions
	data := truncatedFlate(apVisible+"\n", "BT /F1 12 Tf 2 10 Td (TopSecretText) Tj ET\n")
	in := nestDoc(t, []pdf.Dict{{"Rect": apRegion}}, func(b *nestBuilder) []nestPage {
		fm := putRawStream(t, b.w, pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
			"BBox": rect(0, 0, 100, 20), "Resources": helv(),
			"Filter": pdf.Name("FlateDecode"),
		}, data)
		return []nestPage{{
			content: "0 0 1 1 re f",
			annots:  []pdf.Object{b.put(stamp(apRect, pdf.Dict{"N": fm}))},
		}}
	})
	out, raw := writeRaw(t, in)
	if bytes.Contains(raw, data) {
		t.Error("output holds the original stream")
	}
	checkAP(t, out, pageAnnot(t, out, isStamp), "AP", "N")
}

// TestAppearanceNoRotate checks that on a rotated page, an annotation which
// stays upright is deleted where it is shown over a region, even though its
// rectangle lies outside the regions.
func TestAppearanceNoRotate(t *testing.T) {
	// the region is left of x = 150; the annotation turned by 270 degrees
	// about (160, 240) covers x from 120 to 160, turned by 90 degrees x from
	// 160 to 200
	cases := []struct {
		name    string
		rotate  int
		subtype pdf.Name
		flags   int
		kept    bool
	}{
		{"NoRotate", 270, "Square", 16, false},
		{"text annotation", 270, "Text", 0, false},
		{"NoRotate turned away", 90, "Square", 16, true},
		{"rotates with page", 270, "Square", 0, true},
		{"page not rotated", 0, "Square", 16, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 150, 300)}}, func(b *nestBuilder) []nestPage {
				fm := b.form(pdf.Dict{"BBox": rect(0, 0, 40, 40), "Resources": helv()}, apVisible)
				a := pdf.Dict{
					"Type": pdf.Name("Annot"), "Subtype": c.subtype,
					"Rect": rect(160, 200, 200, 240), "F": pdf.Integer(c.flags),
					"AP": pdf.Dict{"N": fm},
				}
				return []nestPage{{
					content: "0 0 1 1 re f",
					annots:  []pdf.Object{b.put(a)},
					extra:   pdf.Dict{"Rotate": pdf.Integer(c.rotate)},
				}}
			})
			out := writeAndRead(t, in, nil, "")
			kept := slices.Contains(annotSubtypes(t, out, pageAt(t, out, 0)), c.subtype)
			if kept != c.kept {
				t.Errorf("annotation kept: %t, want %t", kept, c.kept)
			}
		})
	}
}

// TestAppearanceNoRotateContent checks that the appearance of a kept
// annotation which stays upright on a rotated page loses the content which
// the turn moves into a region.
func TestAppearanceNoRotateContent(t *testing.T) {
	// The annotation turns by 90 degrees about (160, 240).  The text APSECRET
	// lies at (170, 270) in the unturned rectangle, outside the region, and
	// at (130, 250) when turned, inside it.
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 150, 300)}}, func(b *nestBuilder) []nestPage {
		fm := b.form(pdf.Dict{"BBox": rect(0, 0, 40, 40), "Resources": helv()},
			"BT /F1 6 Tf 2 2 Td (Visible) Tj 8 68 Td (APSECRET) Tj ET")
		a := pdf.Dict{
			"Type": pdf.Name("Annot"), "Subtype": pdf.Name("Stamp"),
			"Rect": rect(160, 200, 200, 240), "F": pdf.Integer(16),
			"AP": pdf.Dict{"N": fm},
		}
		return []nestPage{{
			content: "0 0 1 1 re f",
			annots:  []pdf.Object{b.put(a)},
			extra:   pdf.Dict{"Rotate": pdf.Integer(90)},
		}}
	})
	out := writeAndRead(t, in, nil, "")
	checkAP(t, out, pageAnnot(t, out, isStamp), "AP", "N")
	if n := streamsContaining(t, out, "APSECRET"); n != 0 {
		t.Errorf("%d streams hold the text in the region", n)
	}
}

// TestAppearanceMalformedMatrix checks that a kept appearance stream with a
// malformed /Matrix, which is analysed with the identity matrix, is shown
// with a matrix which places it in the same way.
func TestAppearanceMalformedMatrix(t *testing.T) {
	for name, m := range badMatrices {
		t.Run(name, func(t *testing.T) {
			in := nestDoc(t, []pdf.Dict{{"Rect": apRegion}}, func(b *nestBuilder) []nestPage {
				fm := b.form(pdf.Dict{"BBox": rect(0, 0, 100, 20), "Resources": helv(), "Matrix": m}, apSecret)
				return []nestPage{{
					content: "0 0 1 1 re f",
					annots:  []pdf.Object{b.put(stamp(apRect, pdf.Dict{"N": fm}))},
				}}
			})
			out := writeAndRead(t, in, nil, "")
			checkMatrices(t, out)
			checkAP(t, out, pageAnnot(t, out, isStamp), "AP", "N")
		})
	}
}
