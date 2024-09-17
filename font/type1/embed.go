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

package type1

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/widths"
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/type1"
)

// FontDict is the information needed to embed a Type 1 font.
type FontDict struct {
	// Font (optional) is the (subsetted as needed) font to embed.
	// This is non-nil, if and only if the font program is embedded.
	// At least one of `Font` and `Metrics` must be non-nil.
	Font *type1.Font

	// Metrics (optional) are the font metrics for the font.
	// At least one of `Font` and `Metrics` must be non-nil.
	//
	// TODO(voss): remove this field, and add widths etc as separate fields
	Metrics *afm.Metrics

	// SubsetTag should be a unique tag for the font subset,
	// or the empty string if this is the full font.
	SubsetTag string

	// Encoding (a slice of length 256) is the encoding vector used by the client.
	// When embedding a font, this is used to determine the `Encoding` entry in
	// the PDF font dictionary.
	Encoding []string

	// ResName is the resource name for the font (only used for PDF-1.0).
	ResName pdf.Name

	IsSerif    bool
	IsScript   bool
	IsAllCap   bool
	IsSmallCap bool

	// ToUnicode (optional) is a map from character codes to unicode strings.
	ToUnicode *cmap.ToUnicode
}

// Extract extracts information about a Type 1 font from a PDF file.
//
// The `Font` field in the result is only filled if the font program
// is included in the file.  `Metrics` is always present, and contains
// all information available in the PDF file.
func Extract(r pdf.Getter, dicts *font.Dicts) (*FontDict, error) {
	if err := dicts.Type.MustBe(font.Type1); err != nil {
		return nil, err
	}

	// We ignore errors as much as possible, to allow for reading of malformed
	// PDF files.

	res := &FontDict{}

	var psFont *type1.Font
	if dicts.FontProgram != nil {
		stm, err := pdf.DecodeStream(r, dicts.FontProgram, 0)
		if err != nil {
			return nil, err
		}
		psFont, err = type1.Read(stm)
		if err != nil {
			return nil, err
		}
	}
	res.Font = psFont

	res.SubsetTag = dicts.SubsetTag

	var builtinEncoding []string
	if psFont != nil {
		builtinEncoding = psFont.Encoding
	} else {
		switch dicts.PostScriptName {
		case "Symbol":
			builtinEncoding = pdfenc.SymbolEncoding[:]
		case "ZapfDingbats":
			builtinEncoding = pdfenc.ZapfDingbatsEncoding[:]
		default:
			builtinEncoding = pdfenc.StandardEncoding[:]
		}
	}
	encoding, err := encoding.UndescribeEncodingType1(
		r, dicts.FontDict["Encoding"], builtinEncoding)
	if err != nil {
		encoding = builtinEncoding
	}
	res.Encoding = encoding

	res.SubsetTag = dicts.SubsetTag

	res.ResName, _ = pdf.GetName(r, dicts.FontDict["Name"])

	if dicts.FontDescriptor != nil {
		res.IsSerif = dicts.FontDescriptor.IsSerif
		res.IsScript = dicts.FontDescriptor.IsScript
		res.IsAllCap = dicts.FontDescriptor.IsAllCap
		res.IsSmallCap = dicts.FontDescriptor.IsSmallCap
	}

	if info, _ := cmap.ExtractToUnicode(r, dicts.FontDict["ToUnicode"], charcode.Simple); info != nil {
		res.ToUnicode = info
	}

	useMetrics := psFont == nil || dicts.FontDescriptor != nil
	if useMetrics {
		ww := make([]float64, 256) // text space units times 1000
		widthsDict, _ := pdf.GetArray(r, dicts.FontDict["Widths"])
		if widthsDict != nil {
			var defaultWidth float64
			if dicts.FontDescriptor != nil {
				defaultWidth = dicts.FontDescriptor.MissingWidth
			}

			firstChar, _ := pdf.GetInteger(r, dicts.FontDict["FirstChar"])
			if firstChar < 0 {
				firstChar = 0
			} else if firstChar > 256 {
				firstChar = 256
			}
			for i := range ww {
				ww[i] = defaultWidth
				if i >= int(firstChar) && i < int(firstChar)+len(widthsDict) {
					x, _ := pdf.GetNumber(r, widthsDict[i-int(firstChar)])
					ww[i] = float64(x)
				}
			}
		} else if m, ok := builtinMetrics[string(dicts.PostScriptName)]; ok {
			for i, name := range encoding {
				ww[i] = m.Widths[name]
			}
		}

		metrics := &afm.Metrics{
			Glyphs:   map[string]*afm.GlyphInfo{},
			FontName: string(dicts.PostScriptName),
			Encoding: builtinEncoding,
		}
		if psFont != nil {
			q := psFont.FontInfo.FontMatrix[0] * 1000
			for name, g := range psFont.Glyphs {
				metrics.Glyphs[name] = &afm.GlyphInfo{WidthX: g.WidthX * q}
			}
		} else {
			metrics.Glyphs[".notdef"] = &afm.GlyphInfo{}
			for i, name := range encoding {
				metrics.Glyphs[name] = &afm.GlyphInfo{WidthX: ww[i]}
			}
		}
		if dicts.FontDescriptor != nil {
			metrics.Ascent = dicts.FontDescriptor.Ascent
			metrics.Descent = dicts.FontDescriptor.Descent
			metrics.CapHeight = dicts.FontDescriptor.CapHeight
			metrics.XHeight = dicts.FontDescriptor.XHeight
		}
		res.Metrics = metrics
	}

	return res, nil
}

// Embed implements the [font.Dict] interface.
func (info *FontDict) Embed(w *pdf.Writer, fontDictRef pdf.Reference) error {
	postScriptName := info.PostScriptName()
	fontName := postScriptName
	if info.SubsetTag != "" {
		fontName = info.SubsetTag + "+" + fontName
	}

	var fontFileRef pdf.Reference

	// See section 9.6.2.1 of PDF 32000-1:2008.
	fontDict := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(fontName),
	}
	if w.GetMeta().Version == pdf.V1_0 && info.ResName != "" {
		fontDict["Name"] = info.ResName
	}
	if enc := encoding.DescribeEncodingType1(info.Encoding, info.BuiltinEncoding()); enc != nil {
		fontDict["Encoding"] = enc
	}
	var toUnicodeRef pdf.Reference
	if info.ToUnicode != nil {
		toUnicodeRef = w.Alloc()
		fontDict["ToUnicode"] = toUnicodeRef
	}

	compressedRefs := []pdf.Reference{fontDictRef}
	compressedObjects := []pdf.Object{fontDict}

	ww := info.GetWidths()
	for i := range ww {
		ww[i] *= 1000
	}

	omitMetrics := pdf.GetVersion(w) < pdf.V2_0 && isStandard(postScriptName, info.Encoding, ww)

	if !omitMetrics {
		widthsRef := w.Alloc()
		widthsInfo := widths.EncodeSimple(ww)
		fontDict["FirstChar"] = widthsInfo.FirstChar
		fontDict["LastChar"] = widthsInfo.LastChar
		fontDict["Widths"] = widthsRef
		compressedRefs = append(compressedRefs, widthsRef)
		compressedObjects = append(compressedObjects, widthsInfo.Widths)

		fdRef := w.Alloc()
		fontDict["FontDescriptor"] = fdRef

		fd := &font.Descriptor{
			FontName:     fontName,
			IsSerif:      info.IsSerif,
			IsScript:     info.IsScript,
			IsAllCap:     info.IsAllCap,
			IsSmallCap:   info.IsSmallCap,
			MissingWidth: widthsInfo.MissingWidth,
		}

		if metrics := info.Metrics; metrics != nil {
			fd.IsFixedPitch = metrics.IsFixedPitch
			fd.CapHeight = float64(metrics.CapHeight)
			fd.XHeight = float64(metrics.XHeight)
			fd.Ascent = float64(metrics.Ascent)
			fd.Descent = float64(metrics.Descent)
		}
		if psFont := info.Font; psFont != nil {
			fd.IsFixedPitch = psFont.FontInfo.IsFixedPitch
			fd.ForceBold = psFont.Private.ForceBold
			q := 1000 * psFont.FontInfo.FontMatrix[0]
			fd.StemV = psFont.Private.StdVW * q
			fontFileRef = w.Alloc()
		}

		isSymbolic := false
		var italicAngle float64
		var fontBBox *pdf.Rectangle
		if psFont := info.Font; psFont != nil {
			for name := range psFont.Glyphs {
				if name != ".notdef" && !pdfenc.IsStandardLatin[name] {
					isSymbolic = true
					break
				}
			}
			italicAngle = psFont.FontInfo.ItalicAngle
			bbox := psFont.BBox()
			q := 1000 * psFont.FontInfo.FontMatrix[0]
			fontBBox = &pdf.Rectangle{
				LLx: bbox.LLx.AsFloat(q),
				LLy: bbox.LLy.AsFloat(q),
				URx: bbox.URx.AsFloat(q),
				URy: bbox.URy.AsFloat(q),
			}
		} else {
			metrics := info.Metrics
			for name := range metrics.Glyphs {
				if name != ".notdef" && !pdfenc.IsStandardLatin[name] {
					isSymbolic = true
					break
				}
			}
			italicAngle = metrics.ItalicAngle
			// TODO(voss): fontBBox
		}
		fd.IsSymbolic = isSymbolic
		fd.IsItalic = italicAngle != 0
		fd.ItalicAngle = italicAngle
		fd.FontBBox = fontBBox

		fontDescriptor := fd.AsDict()
		if fontFileRef != 0 {
			fontDescriptor["FontFile"] = fontFileRef
		}
		compressedObjects = append(compressedObjects, fontDescriptor)
		compressedRefs = append(compressedRefs, fdRef)
	}

	err := w.WriteCompressed(compressedRefs, compressedObjects...)
	if err != nil {
		return pdf.Wrap(err, "Type 1 font dicts")
	}

	if fontFileRef != 0 {
		// See section 9.9 of PDF 32000-1:2008.
		length1 := pdf.NewPlaceholder(w, 10)
		length2 := pdf.NewPlaceholder(w, 10)
		fontFileDict := pdf.Dict{
			"Length1": length1,
			"Length2": length2,
			"Length3": pdf.Integer(0),
		}
		fontFileStream, err := w.OpenStream(fontFileRef, fontFileDict, pdf.FilterCompress{})
		if err != nil {
			return err
		}
		l1, l2, err := info.Font.WritePDF(fontFileStream)
		if err != nil {
			return err
		}
		err = length1.Set(pdf.Integer(l1))
		if err != nil {
			return err
		}
		err = length2.Set(pdf.Integer(l2))
		if err != nil {
			return err
		}
		err = fontFileStream.Close()
		if err != nil {
			return err
		}
	}

	if toUnicodeRef != 0 {
		err = info.ToUnicode.Embed(w, toUnicodeRef)
		if err != nil {
			return err
		}
	}

	return nil
}

// PostScriptName returns the PostScript name of the font.
func (info *FontDict) PostScriptName() string {
	if info.Font != nil {
		return info.Font.FontInfo.FontName
	}
	return info.Metrics.FontName
}

// BuiltinEncoding returns the builtin encoding vector for this font.
func (info *FontDict) BuiltinEncoding() []string {
	if info.Font != nil {
		return info.Font.Encoding
	}
	return info.Metrics.Encoding
}

// GetWidths returns the widths of the 256 encoded characters.
// The returned widths are given in PDF text space units.
func (info *FontDict) GetWidths() []float64 {
	ww := make([]float64, 256)
	if psFont := info.Font; psFont != nil {
		q := psFont.FontInfo.FontMatrix[0]
		notdefWidth := float64(psFont.Glyphs[".notdef"].WidthX) * q
		for i, name := range info.Encoding {
			if g, ok := psFont.Glyphs[name]; ok {
				ww[i] = float64(g.WidthX) * q
			} else {
				ww[i] = notdefWidth
			}
		}
	} else {
		notdefWidth := float64(info.Metrics.Glyphs[".notdef"].WidthX) / 1000
		for i, name := range info.Encoding {
			if g, ok := info.Metrics.Glyphs[name]; ok {
				ww[i] = float64(g.WidthX) / 1000
			} else {
				ww[i] = notdefWidth
			}
		}
	}
	return ww
}

// GlyphList returns the list of glyph names, in a standardised order.
// Glyph IDs, where used, are indices into this list.
func (info *FontDict) GlyphList() []string {
	if info.Font != nil {
		return info.Font.GlyphList()
	}
	return info.Metrics.GlyphList()
}
