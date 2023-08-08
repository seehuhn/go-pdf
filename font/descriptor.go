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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/os2"
)

// Descriptor represents a PDF font descriptor.
//
// See section 9.8.1 of PDF 32000-1:2008.
type Descriptor struct {
	FontName     pdf.Name
	FontFamily   pdf.Name
	FontStretch  os2.Width
	FontWeight   os2.Weight
	IsFixedPitch bool
	IsSerif      bool
	IsScript     bool
	IsItalic     bool
	IsAllCap     bool
	IsSmallCap   bool
	ForceBold    bool
	FontBBox     *pdf.Rectangle
	ItalicAngle  float64
	Ascent       float64
	Descent      float64
	Leading      float64
	CapHeight    float64
	XHeight      float64
	StemV        float64 // set to -1 for Type 3 fonts
	StemH        float64
}

func (d *Descriptor) AsDict(isSymbolic bool) pdf.Dict {
	var flags pdf.Integer
	if d.IsFixedPitch {
		flags |= 1 << (1 - 1)
	}
	if d.IsSerif {
		flags |= 1 << (2 - 1)
	}
	if isSymbolic {
		flags |= 1 << (3 - 1)
	} else {
		flags |= 1 << (6 - 1)
	}
	if d.IsScript {
		flags |= 1 << (4 - 1)
	}
	if d.IsItalic {
		flags |= 1 << (7 - 1)
	}
	if d.IsAllCap {
		flags |= 1 << (17 - 1)
	}
	if d.IsSmallCap {
		flags |= 1 << (18 - 1)
	}
	if d.ForceBold {
		flags |= 1 << (19 - 1)
	}

	dict := pdf.Dict{
		"Type":        pdf.Name("FontDescriptor"),
		"Flags":       flags,
		"ItalicAngle": pdf.Number(d.ItalicAngle),
	}
	if d.FontName != "" {
		// optional for Type 3 fonts
		dict["FontName"] = d.FontName
	}
	if d.FontFamily != "" {
		dict["FontFamily"] = d.FontFamily
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

	return dict
}
