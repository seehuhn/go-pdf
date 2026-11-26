// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

import (
	"slices"
	"strings"
	"testing"

	"seehuhn.de/go/icc"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/measure"
)

// countObjects returns the number of indirect objects of r for which match
// returns true.
func countObjects(t testing.TB, r *pdf.Reader, match func(obj pdf.Native) bool) int {
	t.Helper()
	n := 0
	for i := uint32(1); i < 2000; i++ {
		obj, err := r.Get(pdf.NewReference(i, 0), true)
		if err != nil || obj == nil {
			continue
		}
		if match(obj) {
			n++
		}
	}
	return n
}

// dictOf returns the dictionary of obj, which is a dictionary or a stream.
func dictOf(obj pdf.Native) pdf.Dict {
	switch o := obj.(type) {
	case pdf.Dict:
		return o
	case *pdf.Stream:
		return o.Dict
	}
	return nil
}

// TestSharedExtraFont checks that a font which zero-size text needs through
// an extended graphics state is written once, together with the kept text and
// the graphics state which use it.
func TestSharedExtraFont(t *testing.T) {
	const content = "BT 50 100 Td /G1 gs 3 Tc (ab) Tj (Secret) Tj (cd) Tj /F1 12 Tf (Kept) Tj ET\n" +
		"BT 50 200 Td /F4 12 Tf (Other) Tj ET"
	for _, hide := range []bool{false, true} {
		name := "in resources"
		if hide {
			name = "only in gs"
		}
		t.Run(name, func(t *testing.T) {
			bold := pdf.Dict{
				"Type":     pdf.Name("Font"),
				"Subtype":  pdf.Name("Type1"),
				"BaseFont": pdf.Name("Helvetica-Bold"),
			}
			o := &srcOpts{
				rawFonts: map[pdf.Name]pdf.Dict{"F4": bold},
				gsFont:   "F4", gsHide: hide,
			}
			data := content
			if hide {
				data = strings.Replace(data, "/F4 12 Tf", "/G1 gs 12 Tf", 1)
			}
			before := pageGlyphs(t, textSourceOpt(t, data, o))
			pos := strings.Index(glyphString(before), "Secret")
			if pos < 0 {
				t.Fatalf("test page does not show the secret (got %q)", glyphString(before))
			}
			in := textSourceOpt(t, data, o, regionOver(before[pos:pos+6]))
			out := writeAndRead(t, in, nil, "")

			_, dicts := pageDicts(t, out)
			fonts := resolveDict(t, out, resolveDict(t, out, dicts[0]["Resources"])["Font"])
			extra := false
			for name := range fonts {
				extra = extra || strings.HasPrefix(string(name), "RdF")
			}
			if !extra {
				t.Fatal("the rewritten page adds no font")
			}

			n := countObjects(t, out, func(obj pdf.Native) bool {
				d, ok := obj.(pdf.Dict)
				return ok && d["BaseFont"] == pdf.Name("Helvetica-Bold")
			})
			if n != 1 {
				t.Errorf("output has %d copies of the font, want 1", n)
			}
		})
	}
}

// TestSharedImageEntries checks that the colour space and the measure of a
// redacted image are written once, together with a kept image which uses
// them.
func TestSharedImageEntries(t *testing.T) {
	cs, err := color.ICCBased(icc.SRGBv2Profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	mm := []*measure.NumberFormat{{
		Unit:             pdf.Text("mm"),
		ConversionFactor: 1,
		FractionFormat:   measure.FractionRound,
		SingleUse:        true,
	}}
	m := &measure.RectilinearMeasure{
		ScaleRatio: pdf.Text("1:1000"),
		XAxis:      mm,
		Distance:   mm,
		Area:       mm,
	}
	src := imageDocVersion(t, pdf.V2_0, func(w *pdf.Writer, rm *pdf.ResourceManager) []imgPage {
		var refs [2]pdf.Native
		for i := range refs {
			d := rgbDict(imgW, imgH, secretRGB(imgW, imgH, nil))
			d.ColorSpace = cs
			d.Measure = m
			ref, err := rm.Embed(d)
			if err != nil {
				t.Fatal(err)
			}
			refs[i] = ref
		}
		return []imgPage{{
			content:  placeAt100 + "q 50 0 0 50 220 220 cm /Im2 Do Q\n",
			xobjects: pdf.Dict{"Im1": refs[0], "Im2": refs[1]},
			redacts:  [][4]int{region},
		}}
	})
	out := writeAndRead(t, src, nil, "")

	names := imageNames(t, out, pageAt(t, out, 0))
	if len(names) != 2 || !slices.Contains(names, "Im2") || slices.Contains(names, "Im1") {
		t.Fatalf("got images %v, want Im2 and a redacted copy of Im1", names)
	}

	iccStreams := countObjects(t, out, func(obj pdf.Native) bool {
		stm, ok := obj.(*pdf.Stream)
		// /N without /Type: object streams have /N as well
		return ok && stm.Dict["N"] != nil && stm.Dict["Type"] == nil
	})
	if iccStreams != 1 {
		t.Errorf("output has %d ICC profile streams, want 1", iccStreams)
	}
	measures := countObjects(t, out, func(obj pdf.Native) bool {
		return dictOf(obj)["Subtype"] == pdf.Name("RL")
	})
	if measures != 1 {
		t.Errorf("output has %d measure dictionaries, want 1", measures)
	}
}

// TestRegeneratedAppearanceFresh checks that a regenerated widget keeps none
// of its old appearances, since its rollover appearance may show the removed
// value.
func TestRegeneratedAppearanceFresh(t *testing.T) {
	in := formPageDoc(t, func(b *formPage) pdf.Array {
		f, w1, w2 := b.w.Alloc(), b.w.Alloc(), b.w.Alloc()
		rollover := b.w.Alloc()
		stm, err := b.w.OpenStream(rollover, pdf.Dict{
			"Type": pdf.Name("XObject"), "Subtype": pdf.Name("Form"),
			"BBox":      rect(0, 0, 100, 20),
			"Resources": pdf.Dict{"Font": pdf.Dict{"Helv": b.font}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stm.Write([]byte("BT /Helv 12 Tf 2 2 Td (HOVERSECRET) Tj ET")); err != nil {
			t.Fatal(err)
		}
		if err := stm.Close(); err != nil {
			t.Fatal(err)
		}
		ap := b.ap("SECRETVAL")
		ap["R"] = rollover
		ap["D"] = rollover
		b.set(f, pdf.Dict{
			"FT": pdf.Name("Tx"), "T": pdf.String("F"), "DA": formDA,
			"V": pdf.String("SECRETVAL"), "Kids": pdf.Array{w1, w2},
		})
		b.set(w1, b.widget(under, "w1", pdf.Dict{"Parent": f, "AP": b.ap("SECRETVAL")}))
		b.set(w2, b.widget(outside, "w2", pdf.Dict{"Parent": f, "AP": ap}))
		b.form(f)
		return pdf.Array{w1, w2}
	})
	o := redactForm(t, in)
	o.regenerated("w2", "SECRETVAL")

	apDict := o.dict(o.annot("w2")["AP"])
	if apDict["R"] != nil || apDict["D"] != nil {
		t.Errorf("regenerated widget keeps old appearances: %v", apDict)
	}
	if n := streamsContaining(t, o.r, "HOVERSECRET"); n != 0 {
		t.Errorf("%d streams still show the rollover text", n)
	}
}
