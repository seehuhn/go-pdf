// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"bytes"
	"maps"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
)

// inRegion selects the image cells which [region] covers, for an image
// painted with [placeAt100].
func inRegion(x, y int) bool {
	return x >= 2 && x <= 4 && y >= 5 && y <= 7
}

// regionDict is the redaction annotation for [region].
func regionDict() pdf.Dict {
	return pdf.Dict{"Rect": rect(region[0], region[1], region[2], region[3])}
}

// writeRaw applies the redactions of r and returns the output, both as a
// reader and as the bytes of the file.
func writeRaw(t testing.TB, r *pdf.Reader) (*pdf.Reader, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if err := Write(&buf, r, nil); err != nil {
		t.Fatal(err)
	}
	out := openPDF(t, buf.Bytes())
	return out, buf.Bytes()
}

// plainForm writes a form XObject with uncompressed content.
func plainForm(b *nestBuilder, dict pdf.Dict, data string) pdf.Reference {
	b.t.Helper()
	d := pdf.Dict{
		"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
		"BBox": rect(0, 0, 50, 50),
	}
	maps.Copy(d, dict)
	ref := b.w.Alloc()
	stm, err := b.w.OpenStream(ref, d)
	if err != nil {
		b.t.Fatal(err)
	}
	if _, err := stm.Write([]byte(data)); err != nil {
		b.t.Fatal(err)
	}
	if err := stm.Close(); err != nil {
		b.t.Fatal(err)
	}
	return ref
}

// deepChain writes a chain of forms, each of which paints the next, and
// the last of which paints inner.  On a page with regions, inner and the
// forms above it are nested too deeply to be rewritten.  The result is the
// first form.
func deepChain(b *nestBuilder, inner pdf.Reference) pdf.Reference {
	next := inner
	for range 200 {
		next = b.form(pdf.Dict{
			"BBox":      rect(0, 0, 50, 50),
			"Resources": pdf.Dict{"XObject": pdf.Dict{"N": next}},
		}, "/N Do")
	}
	return next
}

func TestTaintSharedResources(t *testing.T) {
	// two pages share their XObject resources; the first paints Im1 under a
	// region, the second paints only Im2
	other := secretRGB(imgW, imgH, nil)
	in := nestDoc(t, []pdf.Dict{regionDict()}, func(b *nestBuilder) []nestPage {
		im1, err := b.rm.Embed(rgbDict(imgW, imgH, secretRGB(imgW, imgH, inRegion)))
		if err != nil {
			t.Fatal(err)
		}
		im2, err := b.rm.Embed(rgbDict(imgW, imgH, other))
		if err != nil {
			t.Fatal(err)
		}
		xo := b.put(pdf.Dict{"Im1": im1, "Im2": im2})
		res := pdf.Dict{"XObject": xo}
		return []nestPage{
			{content: placeAt100, res: res},
			{content: "q 100 0 0 100 100 100 cm /Im2 Do Q", res: res},
		}
	})
	out := writeAndRead(t, in, nil, "")
	checkNoSecret(t, out)

	page1 := pageAt(t, out, 1)
	if _, got := samplesOf(t, out, xobject(t, out, page1, "Im2")); !bytes.Equal(got, other) {
		t.Error("second page lost its image")
	}
}

func TestTaintFormUsesPageResources(t *testing.T) {
	// the form Fm1 has the resources of the page, and paints the image which
	// the page paints under a region
	data := secretRGB(imgW, imgH, inRegion)
	build := func(paintForm bool) *pdf.Reader {
		return nestDoc(t, []pdf.Dict{regionDict()}, func(b *nestBuilder) []nestPage {
			im, err := b.rm.Embed(rgbDict(imgW, imgH, data))
			if err != nil {
				t.Fatal(err)
			}
			resRef, fm := b.w.Alloc(), b.w.Alloc()
			if err := b.w.Put(resRef, pdf.Dict{"XObject": pdf.Dict{"Im1": im, "Fm1": fm}}); err != nil {
				t.Fatal(err)
			}
			stm, err := b.w.OpenStream(fm, pdf.Dict{
				"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
				"BBox": rect(0, 0, 100, 100), "Resources": resRef,
			}, pdf.FilterCompress{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stm.Write([]byte("q 100 0 0 100 0 0 cm /Im1 Do Q")); err != nil {
				t.Fatal(err)
			}
			if err := stm.Close(); err != nil {
				t.Fatal(err)
			}
			pages := []nestPage{{content: placeAt100, resObj: resRef}}
			if paintForm {
				pages = append(pages, nestPage{content: "q 1 0 0 1 100 100 cm /Fm1 Do Q", resObj: resRef})
			}
			return pages
		})
	}

	t.Run("unpainted", func(t *testing.T) {
		out := writeAndRead(t, build(false), nil, "")
		checkNoSecret(t, out)
	})

	t.Run("painted elsewhere", func(t *testing.T) {
		// the second page has no regions, and its copy of the form keeps
		// the original image
		out := writeAndRead(t, build(true), nil, "")
		page0, page1 := pageAt(t, out, 0), pageAt(t, out, 1)
		for _, name := range imageNames(t, out, page0) {
			if _, got := samplesOf(t, out, xobject(t, out, page0, name)); bytes.Equal(got, data) {
				t.Error("redacted page refers to the original image")
			}
		}
		fm := resolveDict(t, out, xobject(t, out, page1, "Fm1"))
		im := path(t, out, fm, "Resources", "XObject", "Im1")
		if _, got := samplesOf(t, out, im); !bytes.Equal(got, data) {
			t.Error("form on the second page lost the original image")
		}
	})
}

func TestTaintStructureStm(t *testing.T) {
	// a marked-content reference to a form which is dropped is removed from
	// the structure tree, and the form is not copied
	const marker = "DROPPEDFORM"
	in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 50, 50)}}, func(b *nestBuilder) []nestPage {
		dropped := plainForm(b, pdf.Dict{"StructParents": pdf.Integer(3)},
			"% "+marker+"\n/P <</MCID 0>> BDC 0 0 10 10 re f EMC")
		kept := b.form(pdf.Dict{"BBox": rect(0, 0, 20, 20), "StructParents": pdf.Integer(4)},
			"/P <</MCID 0>> BDC 0 0 10 10 re f EMC")
		rootRef, elem := b.w.Alloc(), b.w.Alloc()
		if err := b.w.Put(elem, pdf.Dict{
			"Type": pdf.Name("StructElem"), "S": pdf.Name("P"), "P": rootRef,
			"K": pdf.Array{
				pdf.Dict{"Type": pdf.Name("MCR"), "Stm": dropped, "MCID": pdf.Integer(0)},
				pdf.Dict{"Type": pdf.Name("MCR"), "Stm": kept, "MCID": pdf.Integer(0)},
			},
		}); err != nil {
			t.Fatal(err)
		}
		if err := b.w.Put(rootRef, pdf.Dict{
			"Type": pdf.Name("StructTreeRoot"), "K": elem,
			"ParentTree": pdf.Dict{"Nums": pdf.Array{
				pdf.Integer(3), pdf.Array{elem},
				pdf.Integer(4), pdf.Array{elem},
			}},
		}); err != nil {
			t.Fatal(err)
		}
		b.w.GetMeta().Catalog.StructTreeRoot = rootRef
		return []nestPage{{
			content: "/C1 Do q 1 0 0 1 200 200 cm /Kp Do Q",
			res:     pdf.Dict{"XObject": pdf.Dict{"C1": deepChain(b, dropped), "Kp": kept}},
		}}
	})
	out, data := writeRaw(t, in)
	if bytes.Contains(data, []byte(marker)) {
		t.Error("output holds the dropped form")
	}

	root := resolveDict(t, out, out.GetMeta().Catalog.StructTreeRoot)
	elem := resolveDict(t, out, root["K"])
	kids, err := pdf.Resolve(out, elem["K"])
	if err != nil {
		t.Fatal(err)
	}
	var stms []pdf.Object
	switch k := kids.(type) {
	case pdf.Array:
		for _, kid := range k {
			stms = append(stms, resolveDict(t, out, kid)["Stm"])
		}
	case pdf.Dict:
		stms = append(stms, k["Stm"])
	}
	if len(stms) != 1 {
		t.Fatalf("element has %d marked-content references, want 1", len(stms))
	}
	if stm, err := pdf.CursorAt(pdf.NewExtractor(out), nil).Stream(stms[0]); err != nil || stm == nil {
		t.Errorf("reference to the kept form is lost (%v)", err)
	}
}

func TestUnrewritableForm(t *testing.T) {
	// a form which cannot be rewritten is dropped on a page with regions,
	// and kept unchanged on a page without regions
	const marker = "UNREWRITABLE"
	cases := []struct{ regionUse, plainUse bool }{
		{true, true}, {true, false}, {false, true},
	}
	for _, c := range cases {
		in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 50, 50)}}, func(b *nestBuilder) []nestPage {
			fm := plainForm(b, nil, "% "+marker+"\n0 0 10 10 re f")
			first := nestPage{content: "0 0 1 1 re f"}
			if c.regionUse {
				first = nestPage{
					content: "/C1 Do",
					res:     pdf.Dict{"XObject": pdf.Dict{"C1": deepChain(b, fm)}},
				}
			}
			pages := []nestPage{first}
			if c.plainUse {
				pages = append(pages, nestPage{
					content: "q 1 0 0 1 200 200 cm /Fm1 Do Q",
					res:     pdf.Dict{"XObject": pdf.Dict{"Fm1": fm}},
				})
			}
			return pages
		})
		out, data := writeRaw(t, in)
		want := 0
		if c.plainUse {
			want = 1
			page1 := pageAt(t, out, 1)
			if got := doNames(t, out, page1); len(got) != 1 || got[0] != "Fm1" {
				t.Errorf("%+v: page without regions paints %v", c, got)
			}
		}
		if n := bytes.Count(data, []byte(marker)); n != want {
			t.Errorf("%+v: output holds %d copies of the form, want %d", c, n, want)
		}
	}
}

func TestOtherXObjectSubtypes(t *testing.T) {
	// an XObject which is neither an image nor a form is dropped on a page
	// with regions, and kept on a page without regions
	const marker = "(SECRETPS) show"
	subtypes := map[string]pdf.Object{
		"PostScript": pdf.Name("PS"),
		"missing":    nil,
		"invalid":    pdf.Integer(7),
		"unknown":    pdf.Name("Foo"),
	}
	for label, subtype := range subtypes {
		for _, onPage1 := range []bool{true, false} {
			in := nestDoc(t, []pdf.Dict{{"Rect": rect(0, 0, 50, 50)}}, func(b *nestBuilder) []nestPage {
				d := pdf.Dict{"Type": pdf.Name("XObject")}
				if subtype != nil {
					d["Subtype"] = subtype
				}
				res := pdf.Dict{"XObject": pdf.Dict{"X1": b.stream(d, marker)}}
				pages := []nestPage{{content: "q 1 0 0 1 200 200 cm /X1 Do Q", res: res}}
				if onPage1 {
					pages = append(pages, pages[0])
				}
				return pages
			})
			out := writeAndRead(t, in, nil, "")
			if got := doNames(t, out, pageAt(t, out, 0)); len(got) != 1 || got[0] == "X1" {
				t.Errorf("%s: page with regions paints %v", label, got)
			}
			want := 0
			if onPage1 {
				want = 1
				page1 := pageAt(t, out, 1)
				if got := doNames(t, out, page1); len(got) != 1 || got[0] != "X1" {
					t.Errorf("%s: page without regions paints %v", label, got)
				}
			}
			if n := streamsContaining(t, out, marker); n != want {
				t.Errorf("%s, onPage1=%t: output holds %d copies of the XObject, want %d",
					label, onPage1, n, want)
			}
		}
	}
}

// Where the resources of the glyph procedure of a Type 3 font come from, see
// [type3ImageFont].
const (
	t3FontRes       = iota // the font has resources
	t3PageRes              // the glyph uses the resources of the content
	t3DirectRes            // as t3PageRes, for a direct font dictionary
	t3DirectFontRes        // as t3FontRes, for a direct font dictionary
	t3GlyphRes             // the glyph procedure has resources (PDF 2.0)
)

// type3ImageFont returns a Type 3 font whose glyph "a" paints the image im
// over the glyph box.  The mode gives where the resources come from.
func type3ImageFont(b *nestBuilder, im pdf.Reference, mode int) pdf.Object {
	xo := pdf.Dict{"XObject": pdf.Dict{"Im1": im}}
	procDict := pdf.Dict{}
	if mode == t3GlyphRes {
		procDict["Resources"] = xo
	}
	proc := b.stream(procDict, "1000 0 d0 q 1000 0 0 1000 0 0 cm /Im1 Do Q")
	d := pdf.Dict{
		"Type": pdf.Name("Font"), "Subtype": pdf.Name("Type3"),
		"FontBBox":   rect(0, 0, 1000, 1000),
		"FontMatrix": pdf.Array{pdf.Number(0.001), pdf.Integer(0), pdf.Integer(0), pdf.Number(0.001), pdf.Integer(0), pdf.Integer(0)},
		"CharProcs":  pdf.Dict{"a": proc},
		"Encoding": pdf.Dict{
			"Type":        pdf.Name("Encoding"),
			"Differences": pdf.Array{pdf.Integer(97), pdf.Name("a")},
		},
		"FirstChar": pdf.Integer(97), "LastChar": pdf.Integer(97),
		"Widths": pdf.Array{pdf.Integer(1000)},
	}
	switch mode {
	case t3FontRes:
		d["Resources"] = xo
	case t3DirectFontRes:
		d["Resources"] = xo
		return d
	case t3DirectRes:
		return d
	}
	return b.put(d)
}

func TestTaintType3Glyph(t *testing.T) {
	// The first page paints Im1 under a region.  The glyph "a" of the Type 3
	// font T3 paints Im1 as well; it is shown outside the region on the
	// first page, and on the second page.
	data := secretRGB(imgW, imgH, inRegion)
	build := func(mode int, show bool) *pdf.Reader {
		return nestDoc(t, []pdf.Dict{regionDict()}, func(b *nestBuilder) []nestPage {
			im, err := b.rm.Embed(rgbDict(imgW, imgH, data))
			if err != nil {
				t.Fatal(err)
			}
			res := pdf.Dict{
				"XObject": pdf.Dict{"Im1": im},
				"Font":    pdf.Dict{"T3": type3ImageFont(b, im.(pdf.Reference), mode)},
			}
			text := "BT /T3 20 Tf ET"
			if show {
				text = "BT /T3 20 Tf 10 10 Td (a) Tj ET"
			}
			return []nestPage{
				{content: placeAt100 + text, res: res},
				{content: text, res: res},
			}
		})
	}

	// glyphImage returns the image which the glyph paints on a page
	glyphImage := func(out *pdf.Reader, page pdf.Dict, mode int) pdf.Object {
		res := resolveDict(t, out, page["Resources"])
		font := resolveDict(t, out, resolveDict(t, out, res["Font"])["T3"])
		switch mode {
		case t3FontRes, t3DirectFontRes:
			return path(t, out, font, "Resources", "XObject", "Im1")
		case t3GlyphRes:
			proc := path(t, out, font, "CharProcs", "a")
			stm, err := pdf.CursorAt(pdf.NewExtractor(out), nil).Stream(proc)
			if err != nil {
				t.Fatal(err)
			}
			return path(t, out, stm.Dict, "Resources", "XObject", "Im1")
		}
		return xobject(t, out, page, "Im1")
	}

	modes := map[string]int{
		"font": t3FontRes, "page": t3PageRes, "direct": t3DirectRes,
		"direct with resources": t3DirectFontRes, "glyph": t3GlyphRes,
	}
	for label, mode := range modes {
		t.Run(label, func(t *testing.T) {
			out := writeAndRead(t, build(mode, true), nil, "")
			for i := range 2 {
				page := pageAt(t, out, i)
				if _, got := samplesOf(t, out, glyphImage(out, page, mode)); !bytes.Equal(got, data) {
					t.Errorf("page %d: the glyph lost its image", i)
				}
			}
			page0 := pageAt(t, out, 0)
			for _, name := range doNames(t, out, page0) {
				if name == "RdMk0" {
					continue
				}
				if _, got := samplesOf(t, out, xobject(t, out, page0, name)); bytes.Equal(got, data) {
					t.Error("the page paints the original image under the region")
				}
			}
			checkStable(t, out)
		})
		t.Run(label+", not shown", func(t *testing.T) {
			out := writeAndRead(t, build(mode, false), nil, "")
			checkNoSecret(t, out)
		})
	}
}

func TestTaintType3StateFont(t *testing.T) {
	// The first page paints Im1 under a region.  The extended graphics
	// state GS1 sets the Type 3 font T3, whose glyph "a" paints Im1 from the
	// resources of the font.  On the first page, a second region covers two
	// glyphs of text at a font size so small that the gap for them needs a
	// font size of its own.  The second page has no regions.
	data := secretRGB(imgW, imgH, inRegion)
	regs := []pdf.Dict{regionDict(), {"Rect": rect(54, 299, 61, 301)}}
	build := func(mode int, show bool) *pdf.Reader {
		return nestDoc(t, regs, func(b *nestBuilder) []nestPage {
			im, err := b.rm.Embed(rgbDict(imgW, imgH, data))
			if err != nil {
				t.Fatal(err)
			}
			gs := pdf.Dict{"Font": pdf.Array{type3ImageFont(b, im.(pdf.Reference), mode), pdf.Number(0.0001)}}
			res := pdf.Dict{
				"XObject":   pdf.Dict{"Im1": im},
				"ExtGState": pdf.Dict{"GS1": b.put(gs)},
			}
			text := "q /GS1 gs Q"
			if show {
				text = "q /GS1 gs BT 50 300 Td 5 Tc (aaaa) Tj ET Q"
			}
			return []nestPage{
				{content: placeAt100 + text, res: res},
				{content: text, res: res},
			}
		})
	}

	// glyphImages returns the images which the glyphs of the Type 3 fonts
	// of a page paint
	glyphImages := func(t *testing.T, out *pdf.Reader, page pdf.Dict) []pdf.Object {
		res := resolveDict(t, out, page["Resources"])
		gs := resolveDict(t, out, path(t, out, res, "ExtGState", "GS1"))
		a, err := pdf.CursorAt(pdf.NewExtractor(out), nil).Array(gs["Font"])
		if err != nil || len(a) == 0 {
			t.Fatalf("the extended graphics state lost its font (%v)", err)
		}
		fonts := []pdf.Object{a[0]}
		if res["Font"] != nil {
			// fonts which the rewritten content selects with Tf
			for _, f := range resolveDict(t, out, res["Font"]) {
				fonts = append(fonts, f)
			}
		}
		var images []pdf.Object
		for _, f := range fonts {
			images = append(images, path(t, out, resolveDict(t, out, f), "Resources", "XObject", "Im1"))
		}
		return images
	}

	modes := map[string]int{"indirect": t3FontRes, "direct": t3DirectFontRes}
	for label, mode := range modes {
		t.Run(label, func(t *testing.T) {
			out := writeAndRead(t, build(mode, true), nil, "")
			for i := range 2 {
				page := pageAt(t, out, i)
				if !showsText(t, out, page) {
					// the text was removed, since the font cannot be read
					continue
				}
				for _, im := range glyphImages(t, out, page) {
					if _, got := samplesOf(t, out, im); !bytes.Equal(got, data) {
						t.Errorf("page %d: a glyph lost its image", i)
					}
				}
			}
			checkStable(t, out)
		})
		t.Run(label+", not shown", func(t *testing.T) {
			out := writeAndRead(t, build(mode, false), nil, "")
			checkNoSecret(t, out)
		})
	}
}

// showsText reports whether the content of a page has a text-showing
// operator.
func showsText(t testing.TB, r *pdf.Reader, page pdf.Dict) bool {
	t.Helper()
	for _, op := range pageOps(t, r, page) {
		switch op.name {
		case content.OpTextShow, content.OpTextShowArray,
			content.OpTextShowMoveNextLine, content.OpTextShowMoveNextLineSetSpacing:
			return true
		}
	}
	return false
}

func TestTaintImageMask(t *testing.T) {
	// The soft mask of Im1 is painted under a region as Im2, on the first
	// page.  Im1 is painted outside the region on the first page, and on
	// the second page.
	const w, h = imgW, imgH
	maskData := make([]byte, w*h)
	for i := range maskData {
		maskData[i] = byte(7 * i)
	}
	in := nestDoc(t, []pdf.Dict{regionDict()}, func(b *nestBuilder) []nestPage {
		sm := b.stream(pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Image"),
			"Width": pdf.Integer(w), "Height": pdf.Integer(h),
			"ColorSpace": pdf.Name("DeviceGray"), "BitsPerComponent": pdf.Integer(8),
		}, string(maskData))
		im := b.stream(pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Image"),
			"Width": pdf.Integer(w), "Height": pdf.Integer(h),
			"ColorSpace": pdf.Name("DeviceRGB"), "BitsPerComponent": pdf.Integer(8),
			"SMask": sm,
		}, string(secretRGB(w, h, nil)))
		res := pdf.Dict{"XObject": pdf.Dict{"Im1": im, "Im2": sm}}
		const kept = "q 100 0 0 100 0 0 cm /Im1 Do Q\n"
		return []nestPage{
			{content: "q 100 0 0 100 100 100 cm /Im2 Do Q\n" + kept, res: res},
			{content: kept, res: res},
		}
	})
	out := writeAndRead(t, in, nil, "")
	for i := range 2 {
		page := pageAt(t, out, i)
		im := resolveDict(t, out, xobject(t, out, page, "Im1"))
		if _, got := samplesOf(t, out, im["SMask"]); !bytes.Equal(got, maskData) {
			t.Errorf("page %d: the image lost its soft mask", i)
		}
	}
	checkStable(t, out)
}

func TestTaintSoftMaskAlias(t *testing.T) {
	// The extended graphics state GS1, shared by both pages, has a soft-mask
	// group whose text lies under a region on the first page.  The second
	// page has no regions.
	build := func(use bool) *pdf.Reader {
		return nestDoc(t, []pdf.Dict{{"Rect": rect(60, 108, 100, 130)}}, func(b *nestBuilder) []nestPage {
			g := b.form(pdf.Dict{
				"BBox": rect(0, 0, 250, 50), "Resources": helv(),
				"Group": pdf.Dict{"S": pdf.Name("Transparency"), "CS": pdf.Name("DeviceGray")},
			}, "1 g "+secretText)
			gs := b.put(pdf.Dict{
				"Type": pdf.Name("ExtGState"), "CA": pdf.Number(0.5),
				"SMask": pdf.Dict{"S": pdf.Name("Luminosity"), "G": g},
			})
			res := pdf.Dict{"ExtGState": pdf.Dict{"GS1": gs}}
			const paint = "q 1 0 0 1 20 100 cm /GS1 gs 0 0 200 100 re f Q"
			second := "0 0 10 10 re f"
			if use {
				second = paint
			}
			return []nestPage{
				{content: paint, res: res},
				{content: second, res: res},
			}
		})
	}

	out := writeAndRead(t, build(true), nil, "")
	page := pageAt(t, out, 1)
	gs := resolveDict(t, out, path(t, out, resolveDict(t, out, page["Resources"]), "ExtGState", "GS1"))
	if gs["CA"] != pdf.Real(0.5) && gs["CA"] != pdf.Number(0.5) {
		t.Errorf("the extended graphics state lost /CA: %v", gs["CA"])
	}
	sm := resolveDict(t, out, gs["SMask"])
	if sm["S"] != pdf.Name("Luminosity") {
		t.Errorf("the soft mask lost /S: %v", sm["S"])
	}
	cur := pdf.CursorAt(pdf.NewExtractor(out), nil)
	stm, err := cur.Stream(sm["G"])
	if err != nil || stm == nil {
		t.Fatalf("the soft-mask group is lost (%v)", err)
	}
	if n := streamsContaining(t, out, "Hello Secret World"); n != 1 {
		t.Errorf("%d streams hold the soft-mask group, want 1", n)
	}
	checkStable(t, out)

	out = writeAndRead(t, build(false), nil, "")
	if n := streamsContaining(t, out, "Secret"); n != 0 {
		t.Errorf("%d streams hold the soft-mask text, which nothing paints", n)
	}
}
