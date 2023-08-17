// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package type3

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/tounicode"
	"seehuhn.de/go/postscript/funit"
)

type Glyph struct {
	WidthX funit.Int16
	BBox   funit.Rect16
	Data   []byte
}

type EmbedInfo struct {
	Glyphs map[string]*Glyph

	FontMatrix [6]float64

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to determine the `Encoding` entry of the PDF font dictionary.
	Encoding []string

	// ToUnicode (optional) is a map from character codes to unicode strings.
	// Character codes must be in the range 0, ..., 255.
	ToUnicode map[charcode.CharCode][]rune

	// ResName is the resource name for the font.
	// This is only used for PDF version 1.0.
	ResName pdf.Name

	// Resources is the resource dictionary for the font.
	Resources *pdf.Resources

	// Descriptor *font.Descriptor
	Descriptor *font.Descriptor
}

func (info *EmbedInfo) Embed(w pdf.Putter, fontDictRef pdf.Reference) error {
	if len(info.FontMatrix) != 6 {
		return errors.New("invalid font matrix")
	}
	fontMatrix := make(pdf.Array, len(info.FontMatrix))
	for i, x := range info.FontMatrix {
		fontMatrix[i] = pdf.Number(x)
	}

	charProcs := make(pdf.Dict, len(info.Glyphs))
	for name := range info.Glyphs {
		charProcs[pdf.Name(name)] = w.Alloc()
	}

	differences := pdf.Array{}
	prev := 256
	for code, name := range info.Encoding {
		name := pdf.Name(name)
		if _, exists := charProcs[name]; !exists {
			continue
		}
		if code != prev+1 {
			differences = append(differences, pdf.Integer(code))
		}
		differences = append(differences, name)
		prev = code
	}
	encoding := pdf.Dict{
		"Differences": differences,
	}

	ww := make([]funit.Int16, 256)
	for i := range ww {
		name := info.Encoding[i]
		g := info.Glyphs[name]
		if g != nil {
			ww[i] = g.WidthX
		}
	}
	var firstChar pdf.Integer
	lastChar := pdf.Integer(255)
	for lastChar > 0 && ww[lastChar] == 0 {
		lastChar--
	}
	for firstChar < lastChar && ww[firstChar] == 0 {
		firstChar++
	}
	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		// TODO(voss): These widths shall be interpreted in glyph space as
		// specified by FontMatrix (unlike the widths of a Type 1 font, which
		// are in thousandths of a unit of text space).  If FontMatrix
		// specifies a rotation, only the horizontal component of the
		// transformed width shall be used.
		widths[i] = pdf.Integer(ww[int(firstChar)+i])
	}

	// See section 9.6.4 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("Type3"),
		"FontBBox":   &pdf.Rectangle{}, // [0 0 0 0] is always valid for Type 3 fonts
		"FontMatrix": fontMatrix,
		"CharProcs":  charProcs,
		"Encoding":   encoding,
		"FirstChar":  firstChar,
		"LastChar":   lastChar,
		"Widths":     widths,
	}
	if w.GetMeta().Version == pdf.V1_0 {
		fontDict["Name"] = info.ResName
	}
	if !info.Resources.IsEmpty() {
		resources := pdf.AsDict(info.Resources)
		fontDict["Resources"] = resources
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	if info.Descriptor != nil {
		isSymbolic := true
		for name := range charProcs {
			// TODO(voss): should this only check encoded glyphs?
			if !pdfenc.IsStandardLatin[string(name)] {
				isSymbolic = false
				break
			}
		}

		fdRef := w.Alloc()
		fontDict["FontDescriptor"] = fdRef

		d := *info.Descriptor
		d.IsSymbolic = isSymbolic

		fontDescriptor := d.AsDict()
		compressedObjects = append(compressedObjects, fontDescriptor)
		compressedRefs = append(compressedRefs, fdRef)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 3 font dicts")
	}

	for name, g := range info.Glyphs {
		gRef := charProcs[pdf.Name(name)].(pdf.Reference)
		stm, err := w.OpenStream(gRef, nil, pdf.FilterCompress{})
		if err != nil {
			return nil
		}
		_, err = stm.Write(g.Data)
		if err != nil {
			return nil
		}
		err = stm.Close()
		if err != nil {
			return nil
		}
	}

	if toUnicodeRef != 0 {
		err = tounicode.Embed(w, toUnicodeRef, charcode.Simple, info.ToUnicode)
		if err != nil {
			return err
		}
	}

	return nil
}

func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	if err := dicts.Type.MustBe(font.Type3); err != nil {
		return nil, err
	}

	res := &EmbedInfo{}

	charProcs, err := pdf.GetDict(r, dicts.FontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}
	glyphs := make(map[string]*Glyph, len(charProcs))
	for name, ref := range charProcs {
		stm, err := pdf.GetStream(r, ref)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("CharProcs[%s]", name))
		}
		decoded, err := pdf.DecodeStream(r, stm, 0)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("decoding CharProcs[%s]", name))
		}
		data, err := io.ReadAll(decoded)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("reading CharProcs[%s]", name))
		}
		g := &Glyph{Data: data}
		setGlyphGeometry(g, data)
		glyphs[string(name)] = g
	}
	res.Glyphs = glyphs

	fontMatrix, err := pdf.GetArray(r, dicts.FontDict["FontMatrix"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontMatrix")
	}
	if len(fontMatrix) != 6 {
		return nil, errors.New("invalid font matrix")
	}
	for i, x := range fontMatrix {
		xi, err := pdf.GetNumber(r, x)
		if err != nil {
			return nil, pdf.Wrap(err, fmt.Sprintf("FontMatrix[%d]", i))
		}
		res.FontMatrix[i] = float64(xi)
	}

	encoding, err := pdf.GetDict(r, dicts.FontDict["Encoding"])
	if err != nil {
		return nil, pdf.Wrap(err, "Encoding")
	}
	differences, err := pdf.GetArray(r, encoding["Differences"])
	if err != nil {
		return nil, pdf.Wrap(err, "Encoding.Differences")
	}
	res.Encoding = make([]string, 256)
	code := 0
	for _, obj := range differences {
		obj, err = pdf.Resolve(r, obj)
		if err != nil {
			return nil, err
		}
		switch obj := obj.(type) {
		case pdf.Integer:
			code = int(obj)
		case pdf.Name:
			if code < 256 {
				res.Encoding[code] = string(obj)
			}
			code++
		}
	}

	name, err := pdf.GetName(r, dicts.FontDict["Name"])
	if err != nil {
		return nil, pdf.Wrap(err, "Name")
	}
	res.ResName = name

	if info, _ := tounicode.Extract(r, dicts.FontDict["ToUnicode"]); info != nil {
		// TODO(voss): check that the codespace ranges are compatible with the cmap.
		res.ToUnicode = info.GetMapping()
	}

	resources, err := pdf.GetDict(r, dicts.FontDict["Resources"])
	if err != nil {
		return nil, pdf.Wrap(err, "Resources")
	}
	res.Resources = &pdf.Resources{}
	err = pdf.DecodeDict(r, res.Resources, resources)
	if err != nil {
		return nil, pdf.Wrap(err, "decoding Resources")
	}

	fontDescriptor, err := font.DecodeDescriptor(r, dicts.FontDict["FontDescriptor"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontDescriptor")
	}
	fontDescriptor.IsSymbolic = false // TODO(voss)
	res.Descriptor = fontDescriptor

	return res, nil
}

func setGlyphGeometry(g *Glyph, data []byte) {
	m := type3StartRegexp.FindSubmatch(data)
	if len(m) != 9 {
		return
	}
	if m[1] != nil {
		x, _ := strconv.ParseFloat(string(m[1]), 64)
		g.WidthX = funit.Int16(math.Round(x))
	} else if m[3] != nil {
		var xx [6]funit.Int16
		for i := range xx {
			x, _ := strconv.ParseFloat(string(m[3+i]), 64)
			xx[i] = funit.Int16(math.Round(x))
		}
		g.WidthX = xx[0]
		g.BBox = funit.Rect16{
			LLx: xx[2],
			LLy: xx[3],
			URx: xx[4],
			URy: xx[5],
		}
	}
}

var (
	spc = `[\t\n\f\r ]+`
	num = `([+-]?[0-9.]+)` + spc
	d0  = num + num + "d0"
	d1  = num + num + num + num + num + num + "d1"

	type3StartRegexp = regexp.MustCompile(`^[\t\n\f\r ]*(?:` + d0 + "|" + d1 + ")" + spc)
)
