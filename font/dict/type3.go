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

package dict

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

// Type3 represents a Type 3 font dictionary.
type Type3 struct {
	// Ref is the reference to the font dictionary in the PDF file.
	Ref pdf.Reference

	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the font
	// from within content streams.
	Name pdf.Name

	// Descriptor (optional) is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding maps character codes to glyph names.
	Encoding encoding.Type1

	// Width contains the glyph widths for all character codes
	// (PDF glyph space units).
	Width [256]float64

	// Text gives the text content for each character code.
	Text [256]string

	// CharProcs maps the name of each glyph to the content stream which paints
	// the glyph for that character.
	CharProcs map[pdf.Name]pdf.Reference

	// FontBBox (optional) is the font bounding box in glyph space units.
	FontBBox *pdf.Rectangle

	// The FontMatrix maps glyph space to text space.
	FontMatrix matrix.Matrix

	// Resources (optional) holds named resources directly used by contents
	// streams referenced by CharProcs, when the content stream does not itself
	// have a resource dictionary.
	//
	// TODO(voss): Should this be a pdf.Object instead, so that an
	// indirect reference can be used on writing?
	Resources *pdf.Resources
}

// ExtractType3 reads a Type 3 font dictionary from a PDF file.
func ExtractType3(r pdf.Getter, obj pdf.Object) (*Type3, error) {
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
	if subtype != "" && subtype != "Type3" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type3, got %q", subtype),
		}
	}

	d := &Type3{}
	d.Ref, _ = obj.(pdf.Reference)

	d.Name, _ = pdf.GetName(r, fontDict["Name"])

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
		for c := range d.Width {
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

	fontBBox, _ := pdf.GetRectangle(r, fontDict["FontBBox"])
	if fontBBox != nil && !fontBBox.IsZero() {
		d.FontBBox = fontBBox
	}

	d.FontMatrix, _ = pdf.GetMatrix(r, fontDict["FontMatrix"])
	if d.FontMatrix == matrix.Zero { // fallback in case of invalid matrix
		d.FontMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	}

	d.Resources, err = pdf.GetResources(r, fontDict["Resources"])
	if pdf.IsReadError(err) {
		return nil, err
	}

	return d, nil
}

// WriteToPDF adds the Type 3 font dictionary to the PDF file.
func (d *Type3) WriteToPDF(rm *pdf.ResourceManager) error {
	// Check that all data are valid and consistent.
	if d.Ref == 0 {
		return errors.New("missing font dictionary reference")
	}
	if d.CharProcs == nil {
		return errors.New("no glyphs found")
	}
	if d.FontMatrix.IsZero() {
		return errors.New("invalid FontMatrix")
	}

	w := rm.Out

	fontBBox := d.FontBBox
	if fontBBox == nil {
		// In the file, the field is required but [0 0 0 0] is always valid.
		fontBBox = &pdf.Rectangle{}
	}
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type3"),
		"FontBBox": fontBBox,
		"FontMatrix": pdf.Array{
			pdf.Number(d.FontMatrix[0]),
			pdf.Number(d.FontMatrix[1]),
			pdf.Number(d.FontMatrix[2]),
			pdf.Number(d.FontMatrix[3]),
			pdf.Number(d.FontMatrix[4]),
			pdf.Number(d.FontMatrix[5]),
		},
	}
	if d.Name != "" {
		fontDict["Name"] = d.Name
	}

	compressedObjects := []pdf.Object{fontDict}
	compressedRefs := []pdf.Reference{d.Ref}

	charProcsDict := make(pdf.Dict, len(d.CharProcs))
	for name, ref := range d.CharProcs {
		charProcsDict[name] = ref
	}
	if len(charProcsDict) > 5 {
		charProcsRef := w.Alloc()
		fontDict["CharProcs"] = charProcsRef
		compressedObjects = append(compressedObjects, charProcsDict)
		compressedRefs = append(compressedRefs, charProcsRef)
	} else {
		fontDict["CharProcs"] = charProcsDict
	}

	encodingObj, err := d.Encoding.AsPDFType3(w.GetOptions())
	if err != nil {
		return fmt.Errorf("/Encoding: %w", err)
	}
	encodingRef := w.Alloc()
	fontDict["Encoding"] = encodingRef
	compressedObjects = append(compressedObjects, encodingObj)
	compressedRefs = append(compressedRefs, encodingRef)

	// TODO(voss): Introduce a helper function for constructing the widths
	// array.
	var defaultWidth float64
	if d.Descriptor != nil {
		defaultWidth = d.Descriptor.MissingWidth
	}
	firstChar, lastChar := 0, 255
	for lastChar > 0 && (d.Encoding(byte(lastChar)) == "" || d.Width[lastChar] == defaultWidth) {
		lastChar--
	}
	for firstChar < lastChar && (d.Encoding(byte(firstChar)) == "" || d.Width[firstChar] == defaultWidth) {
		firstChar++
	}
	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		widths[i] = pdf.Number(d.Width[firstChar+i])
	}
	fontDict["FirstChar"] = pdf.Integer(firstChar)
	fontDict["LastChar"] = pdf.Integer(lastChar)
	if len(widths) > 10 {
		widthRef := w.Alloc()
		fontDict["Widths"] = widthRef
		compressedObjects = append(compressedObjects, widths)
		compressedRefs = append(compressedRefs, widthRef)
	} else {
		fontDict["Widths"] = widths
	}

	if d.Descriptor != nil {
		fdRef := w.Alloc()
		fdDict := d.Descriptor.AsDict()
		fontDict["FontDescriptor"] = fdRef
		compressedObjects = append(compressedObjects, fdDict)
		compressedRefs = append(compressedRefs, fdRef)
	}

	toUnicodeData := make(map[byte]string)
	for code := range 256 {
		glyphName := d.Encoding(byte(code))
		switch glyphName {
		case "":
			// unused character code, nothing to do

		default:
			rr := names.ToUnicode(glyphName, false)
			if text := d.Text[code]; text != string(rr) {
				toUnicodeData[byte(code)] = text
			}
		}
	}
	if len(toUnicodeData) > 0 {
		tuInfo := cmap.MakeSimpleToUnicode(toUnicodeData)
		ref, _, err := pdf.ResourceManagerEmbed(rm, tuInfo)
		if err != nil {
			return fmt.Errorf("ToUnicode cmap: %w", err)
		}
		fontDict["ToUnicode"] = ref
	}

	if d.Resources != nil {
		resRef := w.Alloc()
		resDict := pdf.AsDict(d.Resources)
		fontDict["Resources"] = resRef
		compressedObjects = append(compressedObjects, resDict)
		compressedRefs = append(compressedRefs, resRef)
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 3 font dicts")
	}

	return nil
}

func (d *Type3) MakeFont() (font.FromFile, error) {
	return d, nil
}

func (d *Type3) GetDict() font.Dict {
	return d
}

func (d *Type3) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (d *Type3) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			code.CID = cid.CID(c) + 1 // leave CID 0 for notdef
			code.Width = d.Width[c]
			code.Text = d.Text[c]
			code.UseWordSpacing = (c == 0x20)
			if !yield(&code) {
				return
			}
		}
	}
}

func init() {
	font.RegisterReader("Type3", func(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
		return ExtractType3(r, obj)
	})
}
