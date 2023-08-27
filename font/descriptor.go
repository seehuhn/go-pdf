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

package font

import (
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/os2"
)

// Descriptor represents a PDF font descriptor.
//
// See section 9.8.1 of PDF 32000-1:2008.
type Descriptor struct {
	FontName    string     // required, except (usually) for Type 3 fonts
	FontFamily  string     // optional
	FontStretch os2.Width  // optional
	FontWeight  os2.Weight // optional

	IsFixedPitch bool // flag
	IsSerif      bool // flag
	IsSymbolic   bool // flag
	IsScript     bool // flag
	IsItalic     bool // flag
	IsAllCap     bool // flag
	IsSmallCap   bool // flag
	ForceBold    bool // flag

	FontBBox     *pdf.Rectangle // required, except for Type 3 fonts
	ItalicAngle  float64        // required
	Ascent       float64        // required, except for Type 3 fonts
	Descent      float64        // required, except for Type 3 fonts
	Leading      float64        // optional (default: 0)
	CapHeight    float64        // required, except if no latin chars and for Type 3 fonts
	XHeight      float64        // optional (default: 0)
	StemV        float64        // required, except for Type 3 fonts (0 = unknown, set to -1 for Type 3 fonts)
	StemH        float64        // optional (default: 0)
	MaxWidth     float64        // optional (default: 0)
	AvgWidth     float64        // optional (default: 0)
	MissingWidth pdf.Number     // optional (default: 0)
}

func DecodeDescriptor(r pdf.Getter, obj pdf.Object) (*Descriptor, error) {
	fontDescriptor, err := pdf.GetDictTyped(r, obj, "FontDescriptor")
	if err != nil {
		return nil, err
	}

	res := &Descriptor{}

	fontName, err := pdf.GetName(r, fontDescriptor["FontName"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontName")
	}
	res.FontName = string(fontName)

	fontFamily, err := pdf.GetString(r, fontDescriptor["FontFamily"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontFamily")
	}
	res.FontFamily = string(fontFamily)

	fontStretch, err := pdf.GetName(r, fontDescriptor["FontStretch"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontStretch")
	}
	switch fontStretch {
	case "UltraCondensed":
		res.FontStretch = os2.WidthUltraCondensed
	case "ExtraCondensed":
		res.FontStretch = os2.WidthExtraCondensed
	case "Condensed":
		res.FontStretch = os2.WidthCondensed
	case "SemiCondensed":
		res.FontStretch = os2.WidthSemiCondensed
	case "Normal":
		res.FontStretch = os2.WidthNormal
	case "SemiExpanded":
		res.FontStretch = os2.WidthSemiExpanded
	case "Expanded":
		res.FontStretch = os2.WidthExpanded
	case "ExtraExpanded":
		res.FontStretch = os2.WidthExtraExpanded
	case "UltraExpanded":
		res.FontStretch = os2.WidthUltraExpanded
	}

	fontWeight, err := pdf.GetNumber(r, fontDescriptor["FontWeight"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontWeight")
	}
	if fontWeight > 0 && fontWeight < 1000 {
		res.FontWeight = os2.Weight(math.Round(float64(fontWeight))).Rounded()
	}

	flags, err := pdf.GetInteger(r, fontDescriptor["Flags"])
	if err != nil {
		return nil, pdf.Wrap(err, "Flags")
	}
	res.IsFixedPitch = flags&flagFixedPitch != 0
	res.IsSerif = flags&flagSerif != 0
	res.IsSymbolic = flags&flagSymbolic != 0
	res.IsScript = flags&flagScript != 0
	res.IsItalic = flags&flagItalic != 0
	res.IsAllCap = flags&flagAllCap != 0
	res.IsSmallCap = flags&flagSmallCap != 0
	res.ForceBold = flags&flagForceBold != 0

	fontBBox, err := pdf.GetRectangle(r, fontDescriptor["FontBBox"])
	if err != nil {
		return nil, pdf.Wrap(err, "FontBBox")
	}
	res.FontBBox = fontBBox

	italicAngle, err := pdf.GetNumber(r, fontDescriptor["ItalicAngle"])
	if err != nil {
		return nil, pdf.Wrap(err, "ItalicAngle")
	}
	res.ItalicAngle = float64(italicAngle)

	ascent, err := pdf.GetNumber(r, fontDescriptor["Ascent"])
	if err != nil {
		return nil, pdf.Wrap(err, "Ascent")
	}
	res.Ascent = float64(ascent)

	descent, err := pdf.GetNumber(r, fontDescriptor["Descent"])
	if err != nil {
		return nil, pdf.Wrap(err, "Descent")
	}
	res.Descent = float64(descent)

	leading, err := pdf.GetNumber(r, fontDescriptor["Leading"])
	if err != nil {
		return nil, pdf.Wrap(err, "Leading")
	}
	res.Leading = float64(leading)

	capHeight, err := pdf.GetNumber(r, fontDescriptor["CapHeight"])
	if err != nil {
		return nil, pdf.Wrap(err, "CapHeight")
	}
	res.CapHeight = float64(capHeight)

	xHeight, err := pdf.GetNumber(r, fontDescriptor["XHeight"])
	if err != nil {
		return nil, pdf.Wrap(err, "XHeight")
	}
	res.XHeight = float64(xHeight)

	if stemVObj, ok := fontDescriptor["StemV"]; ok {
		stemV, err := pdf.GetNumber(r, stemVObj)
		if err != nil {
			return nil, pdf.Wrap(err, "StemV")
		}
		res.StemV = float64(stemV)
	} else {
		res.StemV = -1
	}

	stemH, err := pdf.GetNumber(r, fontDescriptor["StemH"])
	if err != nil {
		return nil, pdf.Wrap(err, "StemH")
	}
	res.StemH = float64(stemH)

	maxWidth, err := pdf.GetNumber(r, fontDescriptor["MaxWidth"])
	if err != nil {
		return nil, pdf.Wrap(err, "MaxWidth")
	}
	res.MaxWidth = float64(maxWidth)

	avgWidth, err := pdf.GetNumber(r, fontDescriptor["AvgWidth"])
	if err != nil {
		return nil, pdf.Wrap(err, "AvgWidth")
	}
	res.AvgWidth = float64(avgWidth)

	missingWidth, err := pdf.GetNumber(r, fontDescriptor["MissingWidth"])
	if err != nil {
		return nil, pdf.Wrap(err, "MissingWidth")
	}
	res.MissingWidth = missingWidth

	return res, nil
}

func (d *Descriptor) AsDict() pdf.Dict {
	var flags pdf.Integer
	if d.IsFixedPitch {
		flags |= flagFixedPitch
	}
	if d.IsSerif {
		flags |= flagSerif
	}
	if d.IsSymbolic {
		flags |= flagSymbolic
	} else {
		flags |= flagNonsymbolic
	}
	if d.IsScript {
		flags |= flagScript
	}
	if d.IsItalic {
		flags |= flagItalic
	}
	if d.IsAllCap {
		flags |= flagAllCap
	}
	if d.IsSmallCap {
		flags |= flagSmallCap
	}
	if d.ForceBold {
		flags |= flagForceBold
	}

	dict := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"Flags":       flags,
		"ItalicAngle": pdf.Number(d.ItalicAngle),
	}
	if d.FontName != "" {
		// optional for Type 3 fonts
		dict["FontName"] = pdf.Name(d.FontName)
	}
	if d.FontFamily != "" {
		dict["FontFamily"] = pdf.String(d.FontFamily)
	}
	switch d.FontStretch {
	case os2.WidthUltraCondensed:
		dict["FontStretch"] = pdf.Name("UltraCondensed")
	case os2.WidthExtraCondensed:
		dict["FontStretch"] = pdf.Name("ExtraCondensed")
	case os2.WidthCondensed:
		dict["FontStretch"] = pdf.Name("Condensed")
	case os2.WidthSemiCondensed:
		dict["FontStretch"] = pdf.Name("SemiCondensed")
	case os2.WidthNormal:
		dict["FontStretch"] = pdf.Name("Normal")
	case os2.WidthSemiExpanded:
		dict["FontStretch"] = pdf.Name("SemiExpanded")
	case os2.WidthExpanded:
		dict["FontStretch"] = pdf.Name("Expanded")
	case os2.WidthExtraExpanded:
		dict["FontStretch"] = pdf.Name("ExtraExpanded")
	case os2.WidthUltraExpanded:
		dict["FontStretch"] = pdf.Name("UltraExpanded")
	}
	if d.FontWeight != 0 {
		dict["FontWeight"] = pdf.Integer(d.FontWeight.Rounded())
	}
	if d.FontBBox != nil {
		dict["FontBBox"] = d.FontBBox
	}
	if d.Ascent != 0 {
		dict["Ascent"] = pdf.Number(d.Ascent)
	}
	if d.Descent != 0 {
		dict["Descent"] = pdf.Number(d.Descent)
	}
	if d.Leading != 0 {
		dict["Leading"] = pdf.Number(d.Leading)
	}
	if d.CapHeight != 0 {
		dict["CapHeight"] = pdf.Number(d.CapHeight)
	}
	if d.XHeight != 0 {
		dict["XHeight"] = pdf.Number(d.XHeight)
	}
	if d.StemV >= 0 {
		dict["StemV"] = pdf.Number(d.StemV)
	}
	if d.StemH != 0 {
		dict["StemH"] = pdf.Number(d.StemH)
	}
	if d.MaxWidth != 0 {
		dict["MaxWidth"] = pdf.Number(d.MaxWidth)
	}
	if d.AvgWidth != 0 {
		dict["AvgWidth"] = pdf.Number(d.AvgWidth)
	}
	if d.MissingWidth != 0 {
		dict["MissingWidth"] = pdf.Number(d.MissingWidth)
	}

	return dict
}

// Possible values for PDF Font Descriptor Flags.
const (
	flagFixedPitch  pdf.Integer = 1 << 0  // All glyphs have the same width (as opposed to proportional or variable-pitch fonts, which have different widths).
	flagSerif       pdf.Integer = 1 << 1  // Glyphs have serifs, which are short strokes drawn at an angle on the top and bottom of glyph stems. (Sans serif fonts do not have serifs.)
	flagSymbolic    pdf.Integer = 1 << 2  // Font contains glyphs outside the Adobe standard Latin character set. This flag and the Nonsymbolic flag shall not both be set or both be clear.
	flagScript      pdf.Integer = 1 << 3  // Glyphs resemble cursive handwriting.
	flagNonsymbolic pdf.Integer = 1 << 5  // Font uses the Adobe standard Latin character set or a subset of it.
	flagItalic      pdf.Integer = 1 << 6  // Glyphs have dominant vertical strokes that are slanted.
	flagAllCap      pdf.Integer = 1 << 16 // Font contains no lowercase letters; typically used for display purposes, such as for titles or headlines.
	flagSmallCap    pdf.Integer = 1 << 17 // Font contains both uppercase and lowercase letters.  The uppercase letters are similar to those in the regular version of the same typeface family. The glyphs for the lowercase letters have the same shapes as the corresponding uppercase letters, but they are sized and their proportions adjusted so that they have the same size and stroke weight as lowercase glyphs in the same typeface family.
	flagForceBold   pdf.Integer = 1 << 18 // ...
)
