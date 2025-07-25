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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
)

var (
	_ font.Dict = (*Type3)(nil)
)

// Type3 holds the information from a Type 3 font dictionary.
type Type3 struct {
	// Name is deprecated and should be left empty.
	// Only used in PDF 1.0 where it was the name used to reference the font
	// from within content streams.
	Name pdf.Name

	// Descriptor (optional) is the font descriptor.
	Descriptor *font.Descriptor

	// Encoding maps character codes to glyph names.
	Encoding encoding.Simple

	// Width contains the glyph widths for all character codes
	// (in PDF glyph space units).
	Width [256]float64

	// ToUnicode (optional) specifies how character codes are mapped to Unicode
	// strings.  This overrides the mapping implied by the glyph names.
	ToUnicode *cmap.ToUnicodeFile

	// CharProcs maps the name of each glyph to the content stream which paints
	// the glyph for that character.
	CharProcs map[pdf.Name]pdf.Reference

	// FontBBox (optional) is the font bounding box in glyph space units.
	FontBBox *pdf.Rectangle

	// The FontMatrix maps glyph space to text space.
	FontMatrix matrix.Matrix

	// Resources (optional) holds named resources directly used by content
	// streams referenced by CharProcs, when the content stream does not itself
	// have a resource dictionary.
	//
	// TODO(voss): Should this be a pdf.Object instead, so that an
	// indirect reference can be used on writing?
	Resources *pdf.Resources
}

// ReadType3 reads a Type 3 font dictionary from a PDF file.
func ReadType3(r pdf.Getter, obj pdf.Object) (*Type3, error) {
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
		return nil, pdf.Errorf("expected font subtype Type3, got %q", subtype)
	}

	d := &Type3{}

	d.Name, _ = pdf.GetName(r, fontDict["Name"])

	fdDict, err := pdf.GetDictTyped(r, fontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	fd, _ := font.ExtractDescriptor(r, fdDict)
	d.Descriptor = fd

	enc, err := encoding.ExtractType3(r, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}
	d.Encoding = enc

	var defaultWidth float64
	if fd != nil {
		defaultWidth = fd.MissingWidth
	}
	getSimpleWidths(d.Width[:], r, fontDict, defaultWidth)

	d.ToUnicode, _ = cmap.ExtractToUnicode(r, fontDict["ToUnicode"])

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

	d.Resources, err = pdf.ExtractResources(r, fontDict["Resources"])
	if pdf.IsReadError(err) {
		return nil, err
	}

	d.repair(r)

	return d, nil
}

// repair fixes invalid data in the font dictionary.
// After repair() has been called, validate() will return nil.
func (d *Type3) repair(r pdf.Getter) {
	if v := pdf.GetVersion(r); v == pdf.V1_0 {
		if d.Name == "" {
			d.Name = "Font"
		}
	}

	if d.FontMatrix.IsZero() {
		d.FontMatrix = matrix.Matrix{0.001, 0, 0, 0.001, 0, 0}
	}
}

// validate performs some basic checks on the font dictionary.
// This is guaranteed to pass after repair has been run.
func (d *Type3) validate(w *pdf.Writer) error {
	if v := pdf.GetVersion(w); v == pdf.V1_0 {
		if d.Name == "" {
			return errors.New("missing font name")
		}
	}

	if d.FontMatrix.IsZero() {
		return errors.New("invalid FontMatrix")
	}

	return nil
}

// WriteToPDF adds the font dictionary to a PDF file using the given reference.
// This implements the [font.Dict] interface.
func (d *Type3) WriteToPDF(rm *pdf.ResourceManager, ref pdf.Reference) error {
	w := rm.Out

	err := d.validate(w)
	if err != nil {
		return err
	}

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
	compressedRefs := []pdf.Reference{ref}

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

	var defaultWidth float64
	if d.Descriptor != nil {
		defaultWidth = d.Descriptor.MissingWidth
	}
	oo, rr := setSimpleWidths(w, fontDict, d.Width[:], d.Encoding, defaultWidth)
	compressedObjects = append(compressedObjects, oo...)
	compressedRefs = append(compressedRefs, rr...)

	if d.Descriptor != nil {
		fdRef := w.Alloc()
		fdDict := d.Descriptor.AsDict()
		fontDict["FontDescriptor"] = fdRef
		compressedObjects = append(compressedObjects, fdDict)
		compressedRefs = append(compressedRefs, fdRef)
	}

	if d.ToUnicode != nil {
		ref, _, err := pdf.ResourceManagerEmbed(rm, d.ToUnicode)
		if err != nil {
			return err
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
		return fmt.Errorf("Type 3 font dict: %w", err)
	}

	return nil
}

func (d *Type3) Codec() *charcode.Codec {
	codec, _ := charcode.NewCodec(charcode.Simple)
	return codec
}

func (d *Type3) Characters() iter.Seq2[charcode.Code, font.Code] {
	return func(yield func(charcode.Code, font.Code) bool) {
		textMap := simpleTextMap("", d.Encoding, d.ToUnicode)
		for c := range 256 {
			code := byte(c)
			var info font.Code
			if d.Encoding(code) != "" {
				info = font.Code{
					CID:            cid.CID(code) + 1,
					Width:          d.Width[code],
					Text:           textMap[code],
					UseWordSpacing: code == 0x20,
				}
			} else {
				continue
			}
			if !yield(charcode.Code(code), info) {
				return
			}
		}
	}
}

// FontInfo returns information about the embedded font program.
// The returned value is of type [*FontInfoType3].
func (d *Type3) FontInfo() any {
	return &FontInfoType3{
		CharProcs:  d.CharProcs,
		FontMatrix: d.FontMatrix,
		Resources:  d.Resources,
	}
}

// MakeFont returns a new font object that can be used to typeset text.
// The font is immutable, i.e. no new glyphs can be added and no new codes
// can be defined via the returned font object.
func (d *Type3) MakeFont() font.FromFile {
	textMap := simpleTextMap("", d.Encoding, d.ToUnicode)
	return &t3Font{
		Dict: d,
		Text: textMap,
	}
}

var (
	_ font.FromFile = &t3Font{}
)

type t3Font struct {
	Dict *Type3
	Text map[byte]string
}

func (f *t3Font) GetDict() font.Dict {
	return f.Dict
}

func (*t3Font) WritingMode() font.WritingMode {
	return font.Horizontal
}

func (f *t3Font) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var res font.Code
		for _, code := range s {
			if f.Dict.Encoding(code) == "" {
				res.CID = 0
			} else {
				res.CID = cid.CID(code) + 1
			}
			res.Width = f.Dict.Width[code]
			res.UseWordSpacing = (code == 0x20)
			res.Text = f.Text[code]
			if !yield(&res) {
				return
			}
		}
	}
}

func init() {
	registerReader("Type3", func(r pdf.Getter, obj pdf.Object) (font.Dict, error) {
		return ReadType3(r, obj)
	})
}
