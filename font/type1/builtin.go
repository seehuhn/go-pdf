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

package type1

import (
	"seehuhn.de/go/postscript/afm"
	"seehuhn.de/go/postscript/funit"
	pstype1 "seehuhn.de/go/postscript/type1"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/loader"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// TODO(voss): Find a way to optionally embed the PostScript font program for
// the 14 built-in PDF fonts for PDF versions before PDF-2.0.

// TODO(voss): if font.Options lists the relevant features, enable ligatures.

// Builtin is one of the 14 built-in PDF fonts.
type Builtin string

// Embed implements the [font.Font] interface.
func (f Builtin) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	afm, err := f.AFM()
	if err != nil {
		return nil, err
	}

	var glyphs *pstype1.Font
	if pdf.GetVersion(w) >= pdf.V2_0 {
		glyphs, err = f.psFont()
		if err != nil {
			return nil, err
		}
	} else {
		afm.FontName = string(f)
	}

	F, err := NewFont(glyphs, afm)
	if err != nil {
		return nil, err
	}

	return F.Embed(w, opt)
}

// psFont returns the PostScript font program for this builtin font.
func (f Builtin) psFont() (*pstype1.Font, error) {
	data, err := builtin.Open(string(f), loader.FontTypeType1)
	if err != nil {
		return nil, err
	}

	psFont, err := pstype1.Read(data)
	if err != nil {
		return nil, err
	}

	// TODO(voss): introduce a way to access the full URW fonts.
	psFont.Encoding = restrictGlyphList(f, psFont.Glyphs)

	return psFont, nil
}

// AFM returns the font metrics for this builtin font.
func (f Builtin) AFM() (*afm.Metrics, error) {
	data, err := builtin.Open(string(f), loader.FontTypeAFM)
	if err != nil {
		return nil, err
	}

	metrics, err := afm.Read(data)
	if err != nil {
		return nil, err
	}

	// TODO(voss): introduce a way to access the full URW fonts.
	metrics.Encoding = restrictGlyphList(f, metrics.Glyphs)

	// Some of the fonts wrongly have a non-zero bounding box for some of the
	// whitespace glyphs.  We fix this here.
	//
	// Revisit this, once
	// https://github.com/ArtifexSoftware/urw-base35-fonts/issues/48
	// is resolved.
	for _, name := range []string{"space", "uni00A0", "uni2002"} {
		if g, ok := metrics.Glyphs[name]; ok {
			g.BBox = funit.Rect16{}
		}
	}

	// Some metrics missing from our .afm files.  We infer values for
	// these from other metrics.
	for _, name := range []string{"d", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := float64(glyph.BBox.URy)
			if y > metrics.Ascent {
				metrics.Ascent = y
			}
		}
	}
	for _, name := range []string{"p", "bracketleft", "bar"} {
		if glyph, ok := metrics.Glyphs[name]; ok {
			y := float64(glyph.BBox.LLy)
			if y < metrics.Descent {
				metrics.Descent = y
			}
		}
	}

	// We add the standard ligatures here, just in case.
	if !metrics.IsFixedPitch {
		type lig struct {
			left, right, result string
		}
		var all = []lig{
			{"f", "f", "ff"},
			{"f", "i", "fi"},
			{"f", "l", "fl"},
			{"ff", "i", "ffi"},
			{"ff", "l", "ffl"},
		}
		for _, l := range all {
			_, leftOk := metrics.Glyphs[l.left]
			_, rightOk := metrics.Glyphs[l.right]
			_, resOk := metrics.Glyphs[l.result]
			if leftOk && rightOk && resOk {
				if len(metrics.Glyphs[l.left].Ligatures) == 0 {
					metrics.Glyphs[l.left].Ligatures = make(map[string]string)
				}
				metrics.Glyphs[l.left].Ligatures[l.right] = l.result
			}
		}
	}

	return metrics, nil
}

// Restrict the font to the character set guaranteed by the spec,
// and return the corresponding encoding.
func restrictGlyphList[T any](f Builtin, glyphs map[string]T) []string {
	var charset map[string]bool
	var encoding []string
	switch f {
	case Symbol:
		charset = pdfenc.IsSymbol
		encoding = pdfenc.SymbolEncoding[:]
	case ZapfDingbats:
		charset = pdfenc.IsZapfDingbats
		encoding = pdfenc.ZapfDingbatsEncoding[:]
	default:
		charset = pdfenc.IsStandardLatin
		encoding = pdfenc.StandardEncoding[:]
	}
	for key := range glyphs {
		if !charset[key] && key != ".notdef" {
			delete(glyphs, key)
		}
	}
	return encoding
}

// standardWidths returns the widths of the encoded glyphs.
func (f Builtin) standardWidths(encoding []string) []float64 {
	ww := make([]float64, len(encoding))
	metrics, err := f.AFM()
	if err != nil {
		panic(err)
	}
	for i, name := range encoding {
		if _, ok := metrics.Glyphs[name]; !ok {
			name = ".notdef"
		}
		ww[i] = float64(metrics.Glyphs[name].WidthX)
	}
	return ww
}

// The 14 standard PDF fonts.
//
// All of these fonts implement the [font.Font] interface.
const (
	Courier              Builtin = "Courier"
	CourierBold          Builtin = "Courier-Bold"
	CourierBoldOblique   Builtin = "Courier-BoldOblique"
	CourierOblique       Builtin = "Courier-Oblique"
	Helvetica            Builtin = "Helvetica"
	HelveticaBold        Builtin = "Helvetica-Bold"
	HelveticaBoldOblique Builtin = "Helvetica-BoldOblique"
	HelveticaOblique     Builtin = "Helvetica-Oblique"
	TimesRoman           Builtin = "Times-Roman"
	TimesBold            Builtin = "Times-Bold"
	TimesBoldItalic      Builtin = "Times-BoldItalic"
	TimesItalic          Builtin = "Times-Italic"
	Symbol               Builtin = "Symbol"
	ZapfDingbats         Builtin = "ZapfDingbats"
)

// Standard contains the standard 14 PDF fonts.
//
// All of these fonts implement the [font.Font] interface.
var Standard = []Builtin{
	Courier,
	CourierBold,
	CourierBoldOblique,
	CourierOblique,
	Helvetica,
	HelveticaBold,
	HelveticaBoldOblique,
	HelveticaOblique,
	TimesRoman,
	TimesBold,
	TimesBoldItalic,
	TimesItalic,
	Symbol,
	ZapfDingbats,
}

var isStandardFont = map[string]bool{
	"Courier":               true,
	"Courier-Bold":          true,
	"Courier-BoldOblique":   true,
	"Courier-Oblique":       true,
	"Helvetica":             true,
	"Helvetica-Bold":        true,
	"Helvetica-BoldOblique": true,
	"Helvetica-Oblique":     true,
	"Times-Roman":           true,
	"Times-Bold":            true,
	"Times-BoldItalic":      true,
	"Times-Italic":          true,
	"Symbol":                true,
	"ZapfDingbats":          true,
}

var builtin = loader.NewFontLoader()
