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

// Command watermark adds a watermark annotation to every page of a PDF file.
//
// Usage:
//
//	watermark [-r] input.pdf TEXT
//
// The text is drawn in grey, half transparent, diagonally across each page.
// With -r, watermark annotations already present in the file are removed
// first.
//
// The result is written to test.pdf as an incremental update: the original
// file is copied unchanged and the new objects are appended, together with a
// cross-reference section listing only the objects which changed.  Each
// page's annotation list is rewritten at its original object number when it
// is an indirect object; otherwise the page dictionary is.  Nothing else in
// the file changes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/extgstate"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/page"
	"seehuhn.de/go/pdf/pagetree"
)

const (
	outputName = "test.pdf"

	// the text spans this fraction of the page diagonal
	diagonalFraction = 0.8

	// approximate cap height of Helvetica-Bold, as a fraction of the font size
	capHeight = 0.72

	opacity = 0.5
)

var textGray = color.DeviceGray(0.5)

func main() {
	replace := flag.Bool("r", false, "remove existing watermark annotations")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: watermark [-r] input.pdf TEXT")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}

	err := run(flag.Arg(0), flag.Arg(1), *replace)
	if err != nil {
		log.Fatal(err)
	}
}

func run(inputName, text string, replace bool) error {
	src, err := os.Open(inputName)
	if err != nil {
		return err
	}
	defer src.Close()
	fi, err := src.Stat()
	if err != nil {
		return err
	}

	dst, err := os.Create(outputName)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Watermark annotations need PDF 1.6; older documents are raised to
	// that version through the catalog's /Version entry.
	opt := &pdf.UpdateOptions{Version: pdf.V1_6}
	w, err := pdf.NewUpdaterTo(src, fi.Size(), dst, opt)
	if err != nil {
		return err
	}
	rm := pdf.NewResourceManager(w)

	stamp, err := newStamp(text, w.GetMeta().Version)
	if err != nil {
		return err
	}

	c := pdf.NewCursor(w)
	freed := make(map[pdf.Reference]bool)
	for pageRef, view := range pagetree.NewIterator(w).All() {
		// The iterator yields a view of the page with inherited attributes
		// filled in, so MediaBox and Rotate can be read from it directly.
		mediaBox, err := pdf.Optional(c.Rectangle(view["MediaBox"]))
		if err != nil {
			return err
		}
		if mediaBox == nil {
			mediaBox = &pdf.Rectangle{URx: 612, URy: 792} // US Letter
		}
		rotate, err := pdf.Optional(c.Integer(view["Rotate"]))
		if err != nil {
			return err
		}

		old, err := pdf.Decode(c, pageRef, page.Decode)
		if err != nil {
			return err
		}

		// Values decoded through the writer stand for the objects they were
		// read from and must not be modified; a new list is built instead.
		annots := &page.Annots{}
		if old.Annots != nil {
			annots.SingleUse = old.Annots.SingleUse
			for _, a := range old.Annots.List {
				if _, isWatermark := a.(*annotation.Watermark); isWatermark && replace {
					// an annotation object may be shared between pages
					if ref := w.Origin(a); ref != 0 && !freed[ref] {
						freed[ref] = true
						if err := w.Free(ref); err != nil {
							return err
						}
					}
					continue
				}
				annots.Add(a)
			}
		}
		annots.Add(&annotation.Watermark{
			Common: annotation.Common{
				Rect:       *mediaBox,
				Flags:      annotation.FlagPrint | annotation.FlagReadOnly,
				Appearance: &appearance.Dict{Normal: stamp.form(*mediaBox, int(rotate))},
			},
		})

		if old.Annots != nil && w.Origin(old.Annots) != 0 {
			// an indirect list is replaced on its own; the page stays as it is
			if _, err := rm.Replace(old.Annots, annots); err != nil {
				return err
			}
			continue
		}

		// old.Annots is absent or inline, so the page dictionary itself
		// changes.  The stored dictionary is edited directly rather than
		// going through page.Encode(&p): page.Decode substitutes US
		// Letter for a missing /MediaBox, and a page inheriting a
		// different box from the page tree would otherwise gain a wrong
		// explicit /MediaBox (clipping CropBox against it).  Editing the
		// raw dictionary leaves every entry but /Annots untouched.
		pageDict, err := c.Dict(pageRef)
		if err != nil {
			return err
		}
		annotsObj, err := rm.Embed(annots)
		if err != nil {
			return err
		}
		if annotsObj != nil {
			pageDict["Annots"] = annotsObj
		} else {
			delete(pageDict, "Annots")
		}
		if err := w.Put(pageRef, pageDict); err != nil {
			return err
		}
	}

	if err := rm.Close(); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return dst.Close()
}

// stamp builds the appearance forms for the watermark text.  Pages with the
// same media box and rotation share one form.
type stamp struct {
	text    string
	font    font.Layouter
	version pdf.Version

	// unitWidth is the width of the text at font size 1
	unitWidth float64

	forms map[pageKey]*form.Form
}

type pageKey struct {
	box    pdf.Rectangle
	rotate int
}

func newStamp(text string, v pdf.Version) (*stamp, error) {
	F, err := standard.HelveticaBold.New()
	if err != nil {
		return nil, err
	}
	unitWidth := F.Layout(nil, 1, text).TotalWidth()
	if unitWidth <= 0 {
		return nil, errors.New("watermark text has no visible glyphs")
	}
	return &stamp{
		text:      text,
		font:      F,
		version:   v,
		unitWidth: unitWidth,
		forms:     make(map[pageKey]*form.Form),
	}, nil
}

// form returns the appearance form for a page with the given media box and
// /Rotate value.  The form uses page coordinates, so its bounding box equals
// the annotation rectangle and no further transformation is needed.
func (s *stamp) form(box pdf.Rectangle, rotate int) *form.Form {
	key := pageKey{box, rotate}
	if f, ok := s.forms[key]; ok {
		return f
	}

	width, height := box.Dx(), box.Dy()
	if rotate%180 != 0 {
		width, height = height, width
	}
	// The page is displayed rotated clockwise by rotate degrees.  The text
	// rises from the lower left to the upper right corner of the displayed
	// page, which in unrotated page coordinates means an angle of phi.
	phi := math.Atan2(height, width) + float64(rotate)*math.Pi/180

	diagonal := math.Hypot(width, height)
	fontSize := diagonalFraction * diagonal / s.unitWidth
	textWidth := fontSize * s.unitWidth

	// start of the baseline, so that the text is centred on the page
	cx := (box.LLx + box.URx) / 2
	cy := (box.LLy + box.URy) / 2
	dx, dy := math.Cos(phi), math.Sin(phi)
	x := cx - textWidth/2*dx + capHeight*fontSize/2*dy
	y := cy - textWidth/2*dy - capHeight*fontSize/2*dx

	// text matrix: rotation by phi, then translation to the baseline start
	tm := matrix.Matrix{
		pdf.Round(dx, 4), pdf.Round(dy, 4),
		pdf.Round(-dy, 4), pdf.Round(dx, 4),
		pdf.Round(x, 2), pdf.Round(y, 2),
	}

	b := builder.New(content.Form, nil, s.version)
	b.SetExtGState(&extgstate.ExtGState{
		Set:       graphics.StateFillAlpha,
		FillAlpha: opacity,
		SingleUse: true,
	})
	b.SetFillColor(textGray)
	b.TextBegin()
	b.TextSetFont(s.font, pdf.Round(fontSize, 2))
	b.TextSetMatrix(tm)
	b.TextShow(s.text)
	b.TextEnd()

	f := &form.Form{
		Content: builder.Must(b.Harvest()),
		Res:     b.Resources,
		BBox:    box,
	}
	s.forms[key] = f
	return f
}
