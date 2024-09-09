// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/graphics/matrix"
)

// EmbedInfo contains the information needed to embed a type 3 font into a PDF document.
type EmbedInfo struct {
	Glyphs map[string]pdf.Reference

	FontMatrix [6]float64

	// Name is the resource name for the font (required for PDF-1.0, optional otherwise).
	Name pdf.Name

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// This is used to construct the `Encoding` entry of the Type 3 font dictionary.
	Encoding []string

	Widths []float64 // Widths of the glyphs in glyph space (!)

	// Resources is the resource dictionary for the font.
	Resources *pdf.Resources

	ItalicAngle float64

	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	ForceBold    bool
	IsAllCap     bool
	IsSmallCap   bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Embed implements the [font.Dict] interface.
func (info *EmbedInfo) Embed(w *pdf.Writer, fontDictRef pdf.Reference) error {
	var compressedRefs []pdf.Reference
	var compressedObjects []pdf.Object

	// reserve space for the font dictionary
	compressedRefs = append(compressedRefs, fontDictRef)
	compressedObjects = append(compressedObjects, nil)

	fontMatrix := make(pdf.Array, len(info.FontMatrix))
	for i, x := range info.FontMatrix {
		fontMatrix[i] = pdf.Number(x)
	}

	charProcs := make(pdf.Dict, len(info.Glyphs))
	for name := range info.Glyphs {
		charProcs[pdf.Name(name)] = info.Glyphs[name]
	}
	var charProcsRef pdf.Object
	if len(charProcs) > 5 { // TODO(voss): tune the threshold
		ref := w.Alloc()
		compressedRefs = append(compressedRefs, ref)
		compressedObjects = append(compressedObjects, charProcs)
		charProcsRef = ref
	} else {
		charProcsRef = charProcs
	}

	differences := pdf.Array{}
	prev := 256
	used := make([]bool, 256)
	for code, name := range info.Encoding {
		name := pdf.Name(name)
		if _, exists := charProcs[name]; !exists {
			continue
		}
		if code != prev+1 {
			differences = append(differences, pdf.Integer(code))
		}
		differences = append(differences, name)
		used[code] = true
		prev = code
	}
	encoding := pdf.Dict{
		"Differences": differences,
	}
	var encodingRef pdf.Object
	if len(differences) > 5 { // TODO(voss): tune the threshold
		ref := w.Alloc()
		compressedRefs = append(compressedRefs, ref)
		compressedObjects = append(compressedObjects, encoding)
		encodingRef = ref
	} else {
		encodingRef = encoding
	}

	// TODO(voss): consider using MissingWidth
	var firstChar pdf.Integer
	lastChar := pdf.Integer(255)
	for lastChar > 0 && (info.Widths[lastChar] == 0 || !used[lastChar]) {
		lastChar--
	}
	for firstChar < lastChar && (info.Widths[firstChar] == 0 || !used[firstChar]) {
		firstChar++
	}
	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		widths[i] = pdf.Integer(info.Widths[int(firstChar)+i])
	}
	var widthsRef pdf.Object
	if len(widths) > 10 { // TODO(voss): tune the threshold
		ref := w.Alloc()
		compressedRefs = append(compressedRefs, ref)
		compressedObjects = append(compressedObjects, widths)
		widthsRef = ref
	} else {
		widthsRef = widths
	}

	// See section 9.6.4 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":       pdf.Name("Font"),
		"Subtype":    pdf.Name("Type3"),
		"FontBBox":   &pdf.Rectangle{}, // required, but [0 0 0 0] is always valid
		"FontMatrix": fontMatrix,
		"CharProcs":  charProcsRef,
		"Encoding":   encodingRef,
		"FirstChar":  firstChar,
		"LastChar":   lastChar,
		"Widths":     widthsRef,
	}
	if info.Name != "" {
		fontDict["Name"] = info.Name
	}
	if !info.Resources.IsEmpty() {
		resources := pdf.AsDict(info.Resources)

		var resourcesRef pdf.Object
		if len(pdf.AsString(resources)) > 60 { // TODO(voss): tune the threshold
			ref := w.Alloc()
			compressedRefs = append(compressedRefs, ref)
			compressedObjects = append(compressedObjects, resources)
			resourcesRef = ref
		} else {
			resourcesRef = resources
		}

		fontDict["Resources"] = resourcesRef
	}

	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	isSymbolic := false
	for name := range info.Glyphs {
		if !pdfenc.IsStandardLatin[string(name)] {
			isSymbolic = true
			break
		}
	}

	needFontDescriptor := (pdf.IsTagged(w) ||
		info.IsFixedPitch || info.IsSerif || info.IsScript || info.IsAllCap || info.IsSmallCap || info.ForceBold ||
		isSymbolic ||
		info.ItalicAngle != 0)
	if needFontDescriptor {
		fd := &font.Descriptor{
			IsFixedPitch: info.IsFixedPitch,
			IsSerif:      info.IsSerif,
			IsSymbolic:   isSymbolic,
			IsScript:     info.IsScript,
			IsItalic:     info.ItalicAngle != 0,
			IsAllCap:     info.IsAllCap,
			IsSmallCap:   info.IsSmallCap,
			ForceBold:    info.ForceBold,
			ItalicAngle:  info.ItalicAngle,
			StemV:        -1,
		}
		if info.Name != "" {
			// See https://pdf-issues.pdfa.org/32000-2-2020/clause09.html#Table120 .
			fd.FontName = string(info.Name)
		}

		fontDescriptorRef := w.Alloc()
		compressedRefs = append(compressedRefs, fontDescriptorRef)
		compressedObjects = append(compressedObjects, fd.AsDict())
		fontDict["FontDescriptor"] = fontDescriptorRef
	}

	compressedObjects[0] = fontDict

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 3 font dicts")
	}

	if toUnicodeRef != 0 {
		err = info.ToUnicode.Embed(w, toUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// Extract extracts information about a type 3 font from a PDF file.
func Extract(r pdf.Getter, dicts *font.Dicts) (*EmbedInfo, error) {
	if err := dicts.Type.MustBe(font.Type3); err != nil {
		return nil, err
	}

	// We ignore errors as much as possible, to allow for reading of malformed
	// PDF files.

	res := &EmbedInfo{}

	charProcs, err := pdf.GetDict(r, dicts.FontDict["CharProcs"])
	if err != nil {
		return nil, pdf.Wrap(err, "CharProcs")
	}
	glyphs := make(map[string]pdf.Reference, len(charProcs))
	for name, ref := range charProcs {
		if ref, ok := ref.(pdf.Reference); ok {
			glyphs[string(name)] = ref
		}
	}
	res.Glyphs = glyphs

	fontMatrix, err := pdf.GetArray(r, dicts.FontDict["FontMatrix"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontMatrix")
	}
	if len(fontMatrix) == 6 {
		for i, x := range fontMatrix {
			xi, _ := pdf.GetNumber(r, x)
			res.FontMatrix[i] = float64(xi)
		}
	}
	if res.FontMatrix == matrix.Zero {
		res.FontMatrix = [6]float64{0.001, 0, 0, 0.001, 0, 0}
	}

	res.Name, _ = pdf.GetName(r, dicts.FontDict["Name"])

	res.Encoding = make([]string, 256)
	encodingIsGood := false
	encoding, err := pdf.GetDict(r, dicts.FontDict["Encoding"])
	if err == nil {
		differences, err := pdf.GetArray(r, encoding["Differences"])
		if err == nil {
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
					name := string(obj)
					if _, exists := glyphs[name]; exists && code >= 0 && code < 256 {
						res.Encoding[code] = name
						encodingIsGood = true
					}
					code++
				}
			}
		}
	}
	if !encodingIsGood { // Use the standard encoding as a fallback.
		for i, name := range pdfenc.StandardEncoding {
			if _, exists := glyphs[name]; exists {
				res.Encoding[i] = name
			}
		}
	}

	firstChar, _ := pdf.GetInteger(r, dicts.FontDict["FirstChar"])
	if firstChar < 0 || firstChar > 255 {
		firstChar = 0
	}
	widths, err := pdf.GetArray(r, dicts.FontDict["Widths"])
	if err != nil {
		return nil, pdf.Wrap(err, "Widths")
	}
	res.Widths = make([]float64, 256)
	for i := range res.Widths {
		idx := i - int(firstChar)
		if idx >= 0 && idx < len(widths) {
			x, _ := pdf.GetNumber(r, widths[idx])
			res.Widths[i] = float64(x)
		}
	}

	resources, _ := pdf.GetDict(r, dicts.FontDict["Resources"])
	if resources != nil {
		res.Resources = &pdf.Resources{}
		err := pdf.DecodeDict(r, res.Resources, resources)
		if err != nil {
			return nil, pdf.Wrap(err, "Resources")
		}
	}

	if dicts.FontDescriptor != nil {
		res.ItalicAngle = dicts.FontDescriptor.ItalicAngle

		res.IsFixedPitch = dicts.FontDescriptor.IsFixedPitch
		res.IsSerif = dicts.FontDescriptor.IsSerif
		res.IsScript = dicts.FontDescriptor.IsScript
		res.ForceBold = dicts.FontDescriptor.ForceBold
		res.IsAllCap = dicts.FontDescriptor.IsAllCap
		res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	}

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	return res, nil
}
