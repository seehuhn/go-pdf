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

package extract

import (
	"errors"
	"io"

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/graphics/content"
)

// extractFontType3 reads a Type 3 font dictionary from a PDF file.
func extractFontType3(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) (*dict.Type3, error) {
	fontDict, err := x.GetDictTyped(path, obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := x.GetName(path, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type3" {
		return nil, pdf.Errorf("expected font subtype Type3, got %q", subtype)
	}

	d := &dict.Type3{}

	d.Name, _ = x.GetName(path, fontDict["Name"])

	fdDict, err := x.GetDictTyped(path, fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(x.R, fdDict)
	d.Descriptor = fd

	enc, err := encoding.ExtractType3(x, path, fontDict["Encoding"], false)
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	getSimpleWidths(d.Width[:], x.R, fontDict, defaultWidth)

	d.ToUnicode, _ = cmap.ExtractToUnicode(x, path, fontDict["ToUnicode"], false)

	// Extract CharProcs - parse each content stream
	charProcsDict, err := x.GetDict(path, fontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}

	// font-level resources (PDF spec 7.8 search order: glyph stream,
	// font dict, page dict — the last is unavailable here).  Stored on
	// d.Resources below, and used as the per-glyph fallback in the loop.
	if fontDict["Resources"] != nil {
		d.Resources, _ = pdf.ExtractorGet(x, path, fontDict["Resources"], Resources)
	}

	charProcs := make(map[pdf.Name]*dict.CharProc, len(charProcsDict))
	for name, obj := range charProcsDict {
		stm, err := x.GetStream(path, obj)
		if err != nil {
			continue // permissive: skip malformed CharProcs
		}
		if stm == nil {
			continue
		}

		// glyph resources (spec 7.8): glyph stream wins, otherwise fall
		// back to the font-level dict; nil when neither is present.
		var foundRes *content.Resources
		if stm.Dict["Resources"] != nil {
			foundRes, _ = pdf.ExtractorGet(x, path, stm.Dict["Resources"], Resources)
		} else if d.Resources != nil {
			foundRes = d.Resources
		}

		// store a reader factory closure so each iteration re-opens the PDF stream
		glyphStm := stm // capture for closure
		stream := content.NewScanner(func() (io.ReadCloser, error) {
			return pdf.DecodeStream(x.R, path, glyphStm, 0)
		})

		// Cheap shape check: a Type 3 glyph procedure must start with
		// either d0 (colored) or d1 (uncolored).  Anything else is
		// rejected here; later operators are not validated.  Break after
		// the first operator closes the underlying reader; subsequent
		// painting reopens via a fresh iterator.
		var firstName content.OpName
		var firstOK bool
		for name := range stream.NewIter().All() {
			firstName = name
			firstOK = true
			break
		}
		if !firstOK ||
			(firstName != content.OpType3ColoredGlyph && firstName != content.OpType3UncoloredGlyph) {
			continue
		}

		charProcs[name] = &dict.CharProc{
			Content:   stream,
			Resources: foundRes, // nil if no resources found in PDF
		}
	}
	d.CharProcs = charProcs

	fontBBox, _ := pdf.GetRectangle(x.R, fontDict["FontBBox"])
	if fontBBox != nil && !fontBBox.IsZero() {
		d.FontBBox = fontBBox
	}

	d.FontMatrix, _ = pdf.GetMatrix(x.R, fontDict["FontMatrix"])

	repairType3(d, x.R)

	return d, nil
}

// repairType3 fixes invalid data in a Type3 font dictionary after extraction.
func repairType3(d *dict.Type3, r pdf.Getter) {
	if v := pdf.GetVersion(r); v == pdf.V1_0 {
		if d.Name == "" {
			d.Name = "Font"
		}
	}
	// Unlike Type 1 / TrueType, Name is preserved in PDF 2.0 for Type 3 fonts
	// because it is the only carrier of the font's human-readable name (Type 3
	// has no BaseFont).  See lrosenthol's clarification at
	// https://github.com/pdf-association/pdf-issues/issues/11#issuecomment-753665847

	if d.FontMatrix.IsZero() {
		d.FontMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	}
}
