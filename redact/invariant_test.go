// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package redact

// Checks shared by many tests: finding glyphs and image pixels which
// remain under a redaction region, and extracting the page contents of a
// document.

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/geom/vec"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/decode"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/graphics/image"
	"seehuhn.de/go/pdf/internal/redactmark"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/reader"
)

// placedImage is an image as painted on a page.
type placedImage struct {
	// img is the image, or nil if it cannot be read
	img graphics.Image

	// ctm maps image space to page space
	ctm matrix.Matrix

	inline bool
}

// pageContent is what the content stream of a page, and the forms it
// paints, show.
type pageContent struct {
	glyphs []glyph
	images []placedImage

	// glyphMark and imageMark tell which glyphs and images belong to the
	// marks which Write draws, the last top-level marked-content sequence
	// tagged /Artifact with the subtype /Redaction
	glyphMark, imageMark []bool
}

// errWalkBudget stops the walk of a page which paints too much.
var errWalkBudget = errors.New("walk budget exceeded")

// walkBudget bounds the number of glyphs, images and forms which
// [pageContents] visits on one page, since nested forms can multiply the
// work.
const walkBudget = 1 << 20

// pageContents returns the glyphs and images on every page of a document.
// It is false if some page paints more than [walkBudget] glyphs, images and
// forms.
func pageContents(t testing.TB, r *pdf.Reader) ([]pageContent, bool) {
	t.Helper()
	x := pdf.NewExtractor(r)
	refs, _ := pageDicts(t, r)
	res := make([]pageContent, len(refs))
	for i, ref := range refs {
		pg, err := pdf.CursorAt(x, nil).Decode(ref, page.Decode)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		pc := &res[i]

		// span counts the marked-content sequences of the marks, and
		// glyphSpan and imageSpan record the sequence each glyph and image
		// lies in, or 0
		span, inSpan := 0, 0
		var markMC *graphics.MarkedContent
		var glyphSpan, imageSpan []int

		steps := 0
		step := func() error {
			steps++
			if steps > walkBudget {
				return errWalkBudget
			}
			return nil
		}

		active := map[*form.Form]bool{}
		var install func(rd *reader.Reader, top bool)
		install = func(rd *reader.Reader, top bool) {
			rd.Character = func(c font.Code) error {
				gs := rd.State.GState
				m := gs.GlyphMatrix(&c).Mul(gs.CTM)
				ctr := m.Apply(vec2(c.Width/2, 0.3))
				pc.glyphs = append(pc.glyphs, glyph{
					text: c.Text, code: c,
					x: m[4], y: m[5], cx: ctr.X, cy: ctr.Y,
				})
				glyphSpan = append(glyphSpan, inSpan)
				return step()
			}
			rd.InlineImage = func(op content.Operator, ctm matrix.Matrix) error {
				img, _ := image.ExtractInline(op, rd.State.Resources)
				pc.images = append(pc.images, placedImage{img: img, ctm: ctm, inline: true})
				imageSpan = append(imageSpan, inSpan)
				return step()
			}
			rd.XObject = func(obj graphics.XObject, ctm matrix.Matrix) error {
				if err := step(); err != nil {
					return err
				}
				switch obj := obj.(type) {
				case graphics.Image:
					pc.images = append(pc.images, placedImage{img: obj, ctm: ctm})
					imageSpan = append(imageSpan, inSpan)
					return nil
				case *form.Form:
					if active[obj] {
						return nil
					}
					active[obj] = true
					defer delete(active, obj)
					fres := obj.Res
					if fres == nil {
						fres = rd.State.Resources
					}
					sub := reader.New(x)
					sub.State = content.NewState(content.Form, fres)
					m := obj.Matrix
					if m.IsZero() {
						m = matrix.Identity
					}
					sub.State.GState.CTM = m.Mul(ctm)
					install(sub, false)
					return sub.ProcessIter(obj.Content.NewIter())
				}
				return nil
			}
			if top {
				rd.MarkedContent = func(event reader.MarkedContentEvent, mc *graphics.MarkedContent) error {
					switch {
					case event == reader.MarkedContentBegin && markMC == nil && isMarkSpan(mc):
						span++
						inSpan, markMC = span, mc
					case event == reader.MarkedContentEnd && mc == markMC:
						inSpan, markMC = 0, nil
					}
					return nil
				}
			}
		}
		rd := reader.New(x)
		install(rd, true)
		err = rd.ProcessPage(pg)
		if errors.Is(err, errWalkBudget) {
			return nil, false
		} else if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}

		pc.glyphMark = make([]bool, len(glyphSpan))
		for k, s := range glyphSpan {
			pc.glyphMark[k] = s > 0 && s == span
		}
		pc.imageMark = make([]bool, len(imageSpan))
		for k, s := range imageSpan {
			pc.imageMark[k] = s > 0 && s == span
		}
	}
	return res, true
}

// isMarkSpan reports whether a marked-content sequence is tagged as the
// sequence in which Write draws the marks.
func isMarkSpan(mc *graphics.MarkedContent) bool {
	if mc.Tag != "Artifact" || mc.Properties == nil {
		return false
	}
	d := mc.Properties.AsDirectDict()
	return d != nil && d["Subtype"] == pdf.Name("Redaction")
}

// inputRegions returns the regions of the redaction annotations of each page,
// in page space.  The document must be one which [Write] accepts.
func inputRegions(t testing.TB, r *pdf.Reader) [][]poly {
	t.Helper()
	cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
	_, dicts := pageDicts(t, r)
	res := make([][]poly, len(dicts))
	for i, dict := range dicts {
		annots, err := pdf.Optional(cur.Array(dict["Annots"]))
		if err != nil {
			t.Fatal(err)
		}
		var quads []redactmark.Quad
		for _, obj := range annots {
			d, err := pdf.Optional(cur.Dict(obj))
			if err != nil {
				t.Fatal(err)
			}
			if st, _ := pdf.Optional(cur.Name(d["Subtype"])); st != "Redact" {
				continue
			}
			a, err := cur.Decode(obj, decode.Annotation)
			if err != nil {
				t.Fatal(err)
			}
			quads = append(quads, redactmark.FromAnnotation(a.(*annotation.Redact), func() font.Layouter { return nil }).Quads...)
		}
		for _, h := range regionHulls(quads) {
			if len(h) >= 3 {
				res[i] = append(res[i], h)
			}
		}
	}
	return res
}

// leakDepth is the depth inside a region beyond which no glyph origin and no
// image cell may lie.  Content which only touches a region, up to
// [touchTol], may remain; the margin absorbs rounding.
const leakDepth = 2 * touchTol

// deepInside reports whether every point of pts lies inside the convex
// polygon h, which has counterclockwise vertices, at a distance of more than
// d from the boundary.
func deepInside(pts []vec.Vec2, h poly, d float64) bool {
	for i := range h {
		a, b := h[i], h[(i+1)%len(h)]
		e := b.Sub(a)
		l := e.Length()
		for _, p := range pts {
			if !(e.Cross(p.Sub(a)) > d*l) {
				return false
			}
		}
	}
	return true
}

// inAnyRegion reports whether all of pts lie deep inside one of the regions.
func inAnyRegion(pts []vec.Vec2, regions []poly) bool {
	for _, h := range regions {
		if deepInside(pts, h, leakDepth) {
			return true
		}
	}
	return false
}

// maxCheckedCells bounds the size of the images whose samples
// [checkNoLeak] inspects.
const maxCheckedCells = 1 << 20

// leakKind says what a [leak] found.
type leakKind int

const (
	leakGlyph  leakKind = iota // a glyph origin lies in a region
	leakImage                  // the footprint of an image lies in a region
	leakPixels                 // an image has unredacted pixels in a region
)

// leak is a finding of [leaks].
type leak struct {
	kind leakKind
	msg  string
}

// leaks lists the glyphs and images of a document which lie under the
// regions of the corresponding pages, as given by [inputRegions].  The marks
// drawn by Write are exempt.
//
// The box which Write tests for a glyph contains the glyph origin, so a glyph
// whose origin lies deep inside a region must be removed.  An image lies
// under a region if its whole footprint does.  For an image XObject or
// inline image whose samples can be read, other than a JPEG 2000 image, every
// pixel which lies deep inside a region must hold the fill value.  Soft
// masks, explicit masks, tiling patterns and annotation appearances are not
// checked.
func leaks(regions [][]poly, doc []pageContent) []leak {
	var res []leak
	for i, pc := range doc {
		if i >= len(regions) || len(regions[i]) == 0 {
			continue
		}
		hulls := regions[i]
		for k, g := range pc.glyphs {
			if pc.glyphMark[k] {
				continue
			}
			if inAnyRegion([]vec.Vec2{{X: g.x, Y: g.y}}, hulls) {
				res = append(res, leak{leakGlyph, fmt.Sprintf("page %d: glyph %q at (%g, %g) lies in a region", i, g.text, g.x, g.y)})
			}
		}
		for k, im := range pc.images {
			if pc.imageMark[k] {
				continue
			}
			if inAnyRegion(transformPoints(unitSquare, im.ctm), hulls) {
				res = append(res, leak{leakImage, fmt.Sprintf("page %d: image %d with matrix %v lies in a region", i, k, im.ctm)})
				continue
			}
			if n := leakedPixels(im, hulls); n > 0 {
				res = append(res, leak{leakPixels, fmt.Sprintf("page %d: image %d with matrix %v has %d pixels in a region", i, k, im.ctm, n)})
			}
		}
	}
	return res
}

// checkNoLeak reports an error if the output of Write has a different number
// of pages from the input, or if some glyph or image of the output lies
// under the regions of the input (see [leaks]).
func checkNoLeak(t testing.TB, regions [][]poly, out []pageContent) {
	t.Helper()
	if len(regions) != len(out) {
		t.Fatalf("output has %d pages, input has %d", len(out), len(regions))
	}
	for _, l := range leaks(regions, out) {
		t.Error(l.msg)
	}
}

// leakedPixels returns the number of pixels of an image which lie deep
// inside a region and do not hold the fill value.  Images which
// [checkNoLeak] does not inspect give 0.
func leakedPixels(im placedImage, hulls []poly) int {
	var sr *image.SampleReader
	var err error
	var fill []uint16
	var bpc int
	switch img := im.img.(type) {
	case *image.Dict:
		if img.Data == nil || img.Data.IsJPX() || img.ColorSpace == nil ||
			img.Width*img.Height > maxCheckedCells {
			return 0
		}
		bpc = img.BitsPerComponent
		fill = fillValues(img.ColorSpace, bpc)
		sr, err = img.SampleRows()
	case *image.Mask:
		if img.Source == nil || img.Source.IsJPX() || img.Width*img.Height > maxCheckedCells {
			return 0
		}
		bpc = 1
		fill = []uint16{1}
		if img.Inverted {
			fill = []uint16{0}
		}
		sr, err = img.SampleRows()
	default:
		return 0
	}
	if err != nil {
		return 0
	}

	w, h := float64(sr.Width), float64(sr.Height)
	leaked := 0
	for y, row := range sr.All() {
		y0, y1 := 1-float64(y+1)/h, 1-float64(y)/h
		for c := range sr.Width {
			x0, x1 := float64(c)/w, float64(c+1)/w
			corners := transformPoints([]vec.Vec2{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}, im.ctm)
			if !inAnyRegion(corners, hulls) {
				continue
			}
			for k, v := range fill {
				if sampleAt(row, c*len(fill)+k, bpc) != v {
					leaked++
					break
				}
			}
		}
	}
	return leaked
}

// sampleAt returns sample i of a packed row of samples of bpc bits.
func sampleAt(row []byte, i, bpc int) uint16 {
	switch bpc {
	case 8:
		return uint16(row[i])
	case 16:
		return uint16(row[2*i])<<8 | uint16(row[2*i+1])
	}
	bit := i * bpc
	shift := 8 - bpc - bit%8
	return uint16(row[bit/8]>>shift) & (1<<bpc - 1)
}

// checkSameContent reports an error if two documents differ in the text or
// position of their glyphs or in the placement of their images.
func checkSameContent(t testing.TB, want, got []pageContent) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("got %d pages, want %d", len(got), len(want))
	}
	for i := range want {
		if !sameGlyphs(want[i].glyphs, got[i].glyphs) {
			t.Errorf("page %d: got text %q, want %q", i,
				glyphString(got[i].glyphs), glyphString(want[i].glyphs))
		}
		a, b := want[i].images, got[i].images
		if len(a) != len(b) {
			t.Errorf("page %d: got %d images, want %d", i, len(b), len(a))
			continue
		}
		for k := range a {
			if a[k].inline != b[k].inline || !closeMatrix(a[k].ctm, b[k].ctm) {
				t.Errorf("page %d: image %d moved from %v to %v", i, k, a[k].ctm, b[k].ctm)
			}
		}
	}
}

// closeMatrix reports whether two matrices agree up to rounding.
func closeMatrix(a, b matrix.Matrix) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-3 {
			return false
		}
	}
	return true
}

// TestLeaks checks that [leaks] finds the glyphs and the image pixels under
// the regions of the first fuzz seeds, and none after redaction.
func TestLeaks(t *testing.T) {
	var glyphs, pixels int
	for _, data := range fuzzSeeds(t)[:2] {
		in := openSource(t, data)
		regions := inputRegions(t, in)
		before, ok := pageContents(t, in)
		if !ok {
			t.Fatal("walk budget exceeded")
		}
		for _, l := range leaks(regions, before) {
			switch l.kind {
			case leakGlyph:
				glyphs++
			case leakPixels:
				pixels++
			}
		}

		after, ok := pageContents(t, writeAndRead(t, in, nil, ""))
		if !ok {
			t.Fatal("walk budget exceeded")
		}
		checkNoLeak(t, regions, after)
	}
	if glyphs == 0 || pixels == 0 {
		t.Errorf("found %d glyphs and %d images under the regions, want some of each", glyphs, pixels)
	}
}

// checkNoRedactAnnots reports an error if a page of the document has a
// redaction annotation.
func checkNoRedactAnnots(t *testing.T, r *pdf.Reader) {
	t.Helper()
	it := pagetree.NewIterator(r)
	for _, dict := range it.All() {
		cur := pdf.CursorAt(pdf.NewExtractor(r), nil)
		annots, err := pdf.Optional(cur.Array(dict["Annots"]))
		if err != nil {
			t.Fatal(err)
		}
		for _, obj := range annots {
			d, err := pdf.Optional(cur.Dict(obj))
			if err != nil {
				t.Fatal(err)
			}
			if st, _ := pdf.Optional(cur.Name(d["Subtype"])); st == "Redact" {
				t.Error("output has a redaction annotation")
			}
		}
	}
	if it.Err != nil {
		t.Fatal(it.Err)
	}
}
