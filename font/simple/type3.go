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

package simple

import (
	"errors"
	"fmt"
	"iter"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
)

// Type3Dict represents a Type 3 font dictionary.
type Type3Dict struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// Name (optional since PDF 1.1) is deprecated and is normally empty. For
	// PDF 1.0 this was the name the font was referenced by from within content
	// streams.
	Name pdf.Name

	// The FontMatrix maps glyph space to text space.
	FontMatrix matrix.Matrix

	// CharProcs maps the name of each glyph to the content stream which paints
	// the glyph for that character.
	CharProcs map[pdf.Name]pdf.Reference

	// Encoding maps character codes to glyph names.
	Encoding encoding.Type1

	// Descriptor (optional) is the font descriptor.
	Descriptor *font.Descriptor

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// Resources (optional) holds named resources directly used by contents
	// streams referenced by CharProcs, when the content stream does not itself
	// have a resource dictionary.
	Resources *pdf.Resources

	// Text gives the text content for each character code.
	Text [256]string
}

// ExtractType3Dict extracts a Type 3 font dictionary from a PDF file.
func ExtractType3Dict(r pdf.Getter, obj pdf.Object) (*Type3Dict, error) {
	fontDict, err := pdf.GetDictTyped(r, obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := pdf.GetName(r, fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type1" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type1, got %q", subtype),
		}
	}

	d := &Type3Dict{}
	d.Ref, _ = obj.(pdf.Reference)

	d.Name, _ = pdf.GetName(r, fontDict["Name"])

	d.FontMatrix, _ = pdf.GetMatrix(r, fontDict["FontMatrix"])
	if d.FontMatrix == matrix.Zero {
		d.FontMatrix = matrix.Identity
	}

	charProcs, err := pdf.GetDict(r, fontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}
	glyphs := make(map[pdf.Name]pdf.Reference, len(charProcs))
	for name, ref := range charProcs {
		if ref, ok := ref.(pdf.Reference); ok {
			glyphs[name] = ref
		}
	}
	d.CharProcs = glyphs

	enc, err := encoding.ExtractType3(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	fdDict, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(r, fdDict)
	d.Descriptor = fd

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}

	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths != nil && len(widths) <= 256 && firstChar >= 0 && firstChar < 256 {
		for c := range widths {
			d.Width[c] = defaultWidth
		}
		for i, w := range widths {
			w, err := pdf.GetNumber(r, w)
			if err != nil {
				continue
			}
			if code := firstChar + pdf.Integer(i); code < 256 {
				d.Width[byte(code)] = float64(w)
			}
		}
	}

	d.Resources, err = pdf.GetResources(r, fontDict["Resources"])
	if pdf.IsReadError(err) {
		return nil, err
	}

	// First try to derive text content from the glyph names.
	for code := range 256 {
		glyphName := enc(byte(code))
		if glyphName == "" {
			continue
		}

		rr := names.ToUnicode(glyphName, false)
		d.Text[code] = string(rr)
	}
	// the ToUnicode cmap, if present, overrides the derived text content
	toUnicode, err := cmap.ExtractToUnicode(r, fontDict["ToUnicode"])
	if pdf.IsReadError(err) {
		return nil, err
	}
	if toUnicode != nil {
		// TODO(voss): implement an iterator on toUnicode to do this
		// more efficiently?
		for code := range 256 {
			rr, found := toUnicode.Lookup([]byte{byte(code)})
			if found {
				d.Text[code] = rr
			}
		}
	}

	return d, nil
}

// WriteToPDF adds the font dictionary to the PDF file.
func (d *Type3Dict) WriteToPDF(rm *pdf.ResourceManager) error {
	panic("not implemented")
}

func (d *Type3Dict) GetScanner() (font.Scanner, error) {
	return d, nil
}

func (d *Type3Dict) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			code.CID = cid.CID(c) + 1
			code.Width = d.Width[c]
			code.Text = d.Text[c]

			if !yield(&code) {
				return
			}
		}
	}
}

func init() {
	font.RegisterReader("Type3", func(r pdf.Getter, obj pdf.Object) (font.FromFile, error) {
		return ExtractType3Dict(r, obj)
	})
}
