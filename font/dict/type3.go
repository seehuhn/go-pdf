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
	"seehuhn.de/go/pdf/graphics/content"
)

// CharProc represents a Type 3 glyph procedure (content stream).
type CharProc struct {
	// Content is the parsed content stream for the glyph.
	Content content.Stream

	// Resources (optional) holds named resources used by this glyph's content
	// stream. If nil, resources are looked up from the font's Resources field.
	Resources *content.Resources
}

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

	// CharProcs maps glyph names to their content streams.
	CharProcs map[pdf.Name]*CharProc

	// FontBBox (optional) is the font bounding box in glyph space units.
	FontBBox *pdf.Rectangle

	// The FontMatrix maps glyph space to text space.
	FontMatrix matrix.Matrix

	// Resources (optional) holds named resources shared by all glyph content
	// streams that don't have their own resource dictionary.
	Resources *content.Resources
}

var _ Dict = (*Type3)(nil)

// validate performs some basic checks on the font dictionary.
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

// Embed adds the font dictionary to a PDF file.
// This implements the [Dict] interface.
func (d *Type3) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.AllocSelf()
	w := rm.Out()

	err := d.validate(w)
	if err != nil {
		return nil, err
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

	// Write CharProc streams and build the CharProcs dictionary.
	v := pdf.GetVersion(w)
	charProcsDict := make(pdf.Dict, len(d.CharProcs))
	for name, cp := range d.CharProcs {
		if cp == nil {
			continue
		}
		cpRef := w.Alloc()
		charProcsDict[name] = cpRef

		// Build stream dictionary with per-glyph resources if present
		var streamDict pdf.Dict
		if cp.Resources != nil {
			resObj, err := cp.Resources.Embed(rm)
			if err != nil {
				return nil, fmt.Errorf("glyph %q resources: %w", name, err)
			}
			streamDict = pdf.Dict{"Resources": resObj}
		}

		stm, err := w.OpenStream(cpRef, streamDict, pdf.FilterCompress{})
		if err != nil {
			return nil, err
		}

		// Use per-glyph resources if present, else fall back to font-level resources
		res := cp.Resources
		if res == nil {
			res = d.Resources
		}
		err = content.Write(stm, cp.Content, v, content.Glyph, res)
		if err != nil {
			stm.Close()
			return nil, fmt.Errorf("glyph %q: %w", name, err)
		}

		err = stm.Close()
		if err != nil {
			return nil, err
		}
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
		return nil, fmt.Errorf("/Encoding: %w", err)
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
		ref, err := rm.Embed(d.ToUnicode)
		if err != nil {
			return nil, err
		}
		fontDict["ToUnicode"] = ref
	}

	if d.Resources != nil {
		resObj, err := d.Resources.Embed(rm)
		if err != nil {
			return nil, fmt.Errorf("font resources: %w", err)
		}
		fontDict["Resources"] = resObj
	}

	err = w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return nil, fmt.Errorf("Type 3 font dict: %w", err)
	}

	return ref, nil
}

func (d *Type3) Codec() *charcode.Codec {
	codec, _ := charcode.NewCodec(charcode.Simple)
	return codec
}

func (d *Type3) Characters() iter.Seq2[charcode.Code, font.Code] {
	return func(yield func(charcode.Code, font.Code) bool) {
		textMap := SimpleTextMap("", d.Encoding, d.ToUnicode)
		for c := range 256 {
			code := byte(c)
			var info font.Code
			if d.Encoding(code) != "" {
				info = font.Code{
					CID:            cid.CID(code) + 1,
					Width:          d.Width[code] * d.FontMatrix[0],
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

// FontInfo returns information about the embedded font file.
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
func (d *Type3) MakeFont() font.Instance {
	textMap := SimpleTextMap("", d.Encoding, d.ToUnicode)
	return &t3Font{
		Dict: d,
		Text: textMap,
	}
}

var (
	_ font.Instance = &t3Font{}
)

type t3Font struct {
	Dict *Type3
	Text map[byte]string
}

func (f *t3Font) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	ref := rm.Alloc()
	_, err := rm.EmbedAt(ref, f.Dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

func (f *t3Font) PostScriptName() string {
	return ""
}

func (f *t3Font) GetDict() Dict {
	return f.Dict
}

// Codec returns the codec for the encoding used by this font.
func (f *t3Font) Codec() *charcode.Codec {
	return f.Dict.Codec()
}

// FontInfo returns information required to load the font file.
func (f *t3Font) FontInfo() any {
	return f.Dict.FontInfo()
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
			res.Width = f.Dict.Width[code] * f.Dict.FontMatrix[0]
			res.UseWordSpacing = (code == 0x20)
			res.Text = f.Text[code]
			if !yield(&res) {
				return
			}
		}
	}
}
