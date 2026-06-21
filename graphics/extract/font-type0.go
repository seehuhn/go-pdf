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

package extract

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/postscript/cid"
)

// extractFontCIDType0 reads a Type 0 CIDFont dictionary from the PDF file.
func extractFontCIDType0(c pdf.Cursor, obj pdf.Object) (*dict.CIDFontType0, error) {
	fontDict, err := c.DictTyped(obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := c.Name(fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type0" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type0, got %q", subtype),
		}
	}

	a, err := c.Array(fontDict["DescendantFonts"])
	if err != nil {
		return nil, err
	} else if len(a) != 1 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid DescendantFonts array"),
		}
	}
	cidFontDict, err := c.DictTyped(a[0], "Font")
	if err != nil {
		return nil, err
	} else if cidFontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing CIDFont dictionary"),
		}
	}
	subtype, err = c.Name(cidFontDict["Subtype"])
	if err != nil {
		return nil, err
	} else if subtype != "CIDFontType0" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected CIDFontType0, got %q", subtype),
		}
	}

	d := &dict.CIDFontType0{}

	// fields in the font dictionary

	d.CMap, err = cmap.Extract(c, fontDict["Encoding"], false)
	if err != nil {
		return nil, err
	}

	d.ToUnicode, _ = pdf.Decode(c, fontDict["ToUnicode"], cmap.ExtractToUnicode)

	// fields in the CIDFont dictionary

	baseFont, err := c.Name(cidFontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		d.PostScriptName = m[2]
		d.SubsetTag = m[1]
	} else {
		d.PostScriptName = string(baseFont)
	}

	fdDict, err := c.DictTyped(cidFontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.Descriptor, err = pdf.DecodeOptional(c, fdDict, font.ExtractDescriptor)
	if err != nil {
		return nil, err
	}

	d.ROS, _ = pdf.Decode(c, cidFontDict["CIDSystemInfo"], font.ExtractCIDSystemInfo)

	d.Width, err = decodeCompositeWidths(c, cidFontDict["W"])
	if err != nil {
		return nil, err
	}
	if obj, ok := cidFontDict["DW"]; ok {
		dw, err := c.Number(obj)
		if pdf.IsReadError(err) {
			return nil, err
		}
		d.DefaultWidth = dw
	} else {
		d.DefaultWidth = dict.DefaultWidthDefault
	}

	dw2, err := decodeVDefault(c, cidFontDict["DW2"])
	if err != nil {
		return nil, err
	}
	d.DefaultVMetrics = dw2
	w2, err := decodeVMetrics(c, cidFontDict["W2"])
	if err != nil {
		return nil, err
	}
	d.VMetrics = w2

	d.FontFile, err = glyphdata.ExtractFontFile(c, fdDict, "Type0")
	if err != nil {
		return nil, err
	}

	repairCIDType0(d)

	return d, nil
}

// repairCIDType0 fixes invalid data in a CIDFontType0 dictionary after extraction.
func repairCIDType0(d *dict.CIDFontType0) {
	if d.Descriptor == nil {
		d.Descriptor = &font.Descriptor{}
	}

	m := subset.TagRegexp.FindStringSubmatch(d.Descriptor.FontName)
	if m != nil {
		if d.SubsetTag == "" {
			d.SubsetTag = m[1]
		}
		if d.PostScriptName == "" {
			d.PostScriptName = m[2]
		}
	} else if d.PostScriptName == "" {
		d.PostScriptName = d.Descriptor.FontName
	}
	if d.PostScriptName == "" {
		d.PostScriptName = "Font"
	}
	if !subset.IsValidTag(d.SubsetTag) {
		d.SubsetTag = ""
	}
	if d.FontFile == nil {
		d.SubsetTag = ""
	}
	d.Descriptor.FontName = subset.Join(d.SubsetTag, d.PostScriptName)

	d.Descriptor.MissingWidth = 0

	if d.ROS == nil {
		d.ROS = &cid.SystemInfo{}
	}
	if d.CMap == nil {
		d.CMap, _ = cmap.Predefined("Identity-H")
	}

	if d.CMap != nil && d.CMap.ROS != nil {
		if d.CMap.Name != "Identity-H" && d.CMap.Name != "Identity-V" ||
			!d.CMap.IsPredefined() {
			if d.ROS.Registry != d.CMap.ROS.Registry ||
				d.ROS.Ordering != d.CMap.ROS.Ordering {
				d.CMap = d.CMap.Clone()
				d.CMap.ROS = d.ROS
			}
		}
	}
}
