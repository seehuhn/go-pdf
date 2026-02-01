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

	"seehuhn.de/go/geom/matrix"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/graphics/content"
)

// extractFontType3 reads a Type 3 font dictionary from a PDF file.
func extractFontType3(x *pdf.Extractor, obj pdf.Object) (*dict.Type3, error) {
	fontDict, err := x.GetDictTyped(obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := x.GetName(fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type3" {
		return nil, pdf.Errorf("expected font subtype Type3, got %q", subtype)
	}

	d := &dict.Type3{}

	d.Name, _ = x.GetName(fontDict["Name"])

	fdDict, err := x.GetDictTyped(fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(x.R, fdDict)
	d.Descriptor = fd

	enc, err := encoding.ExtractType3(x, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	getSimpleWidths(d.Width[:], x.R, fontDict, defaultWidth)

	d.ToUnicode, _ = cmap.ExtractToUnicode(x, fontDict["ToUnicode"])

	// Extract CharProcs - parse each content stream
	charProcsDict, err := x.GetDict(fontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}

	// Extract font-level resources (PDF spec 7.8: search order is glyph stream,
	// then font dict, then page dict). We can access glyph and font resources here.
	var fontRes *content.Resources
	if fontDict["Resources"] != nil {
		fontRes, _ = Resources(x, fontDict["Resources"])
	}

	v := pdf.GetVersion(x.R)
	charProcs := make(map[pdf.Name]*dict.CharProc, len(charProcsDict))
	for name, obj := range charProcsDict {
		stm, err := x.GetStream(obj)
		if err != nil {
			continue // permissive: skip malformed CharProcs
		}
		if stm == nil {
			continue
		}

		// Extract glyph resources per PDF spec 7.8 search order:
		// 1. glyph stream dict, 2. font dict, 3. page dict (not available here)
		// We track foundRes separately to store nil in CharProc when no resources were found.
		var res *content.Resources
		var foundRes *content.Resources
		if stm.Dict["Resources"] != nil {
			foundRes, _ = Resources(x, stm.Dict["Resources"])
			res = foundRes
		} else if fontRes != nil {
			foundRes = fontRes
			res = fontRes
		} else {
			// TODO(voss): ideally we should use page resources here per PDF spec 7.8,
			// but we don't have access to the page context during font extraction.
			res = &content.Resources{}
			// foundRes stays nil - we didn't find actual resources
		}

		// Parse the content stream
		body, err := pdf.DecodeStream(x.R, stm, 0)
		if err != nil {
			continue // permissive
		}
		stream, err := content.ReadStream(body, v, content.Glyph, res)
		body.Close()
		if err != nil {
			continue // permissive
		}

		// Validate the stream is embeddable
		w := content.NewWriter(v, content.Glyph, res)
		if err := w.Validate(stream); err != nil {
			continue // invalid stream, skip this CharProc
		}
		if err := w.Close(); err != nil {
			continue // invalid end state
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

	// Extract font-level resources (only if present)
	if fontDict["Resources"] != nil {
		d.Resources, _ = Resources(x, fontDict["Resources"])
	}

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

	if d.FontMatrix.IsZero() {
		d.FontMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	}
}
