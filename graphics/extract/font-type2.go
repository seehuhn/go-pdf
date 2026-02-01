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
	"io"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
)

// extractFontCIDType2 reads a Type 2 CIDFont dictionary from the PDF file.
func extractFontCIDType2(x *pdf.Extractor, obj pdf.Object) (*dict.CIDFontType2, error) {
	fontDict, err := x.GetDictTyped(obj, "Font")
	if err != nil {
		return nil, err
	} else if fontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing font dictionary"),
		}
	}
	subtype, err := x.GetName(fontDict["Subtype"])
	if err != nil {
		return nil, err
	}
	if subtype != "" && subtype != "Type0" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected font subtype Type0, got %q", subtype),
		}
	}

	a, err := x.GetArray(fontDict["DescendantFonts"])
	if err != nil {
		return nil, err
	} else if len(a) != 1 {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("invalid DescendantFonts array"),
		}
	}
	cidFontDict, err := x.GetDictTyped(a[0], "Font")
	if err != nil {
		return nil, err
	} else if cidFontDict == nil {
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing CIDFont dictionary"),
		}
	}
	subtype, err = x.GetName(cidFontDict["Subtype"])
	if err != nil {
		return nil, err
	} else if subtype != "CIDFontType2" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected CIDFontType2, got %q", subtype),
		}
	}

	d := &dict.CIDFontType2{}

	// fields in the font dictionary

	d.CMap, err = cmap.Extract(x, fontDict["Encoding"])
	if err != nil {
		return nil, err
	}

	d.ToUnicode, _ = cmap.ExtractToUnicode(x, fontDict["ToUnicode"])

	// fields in the CIDFont dictionary

	baseFont, err := x.GetName(cidFontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	if m := subset.TagRegexp.FindStringSubmatch(string(baseFont)); m != nil {
		d.PostScriptName = m[2]
		d.SubsetTag = m[1]
	} else {
		d.PostScriptName = string(baseFont)
	}

	fdDict, err := x.GetDictTyped(cidFontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.Descriptor, _ = font.ExtractDescriptor(x.R, fdDict)

	d.ROS, _ = font.ExtractCIDSystemInfo(x, cidFontDict["CIDSystemInfo"])

	d.Width, err = decodeCompositeWidths(x.R, cidFontDict["W"])
	if err != nil {
		return nil, err
	}
	if obj, ok := cidFontDict["DW"]; ok {
		dw, err := x.GetNumber(obj)
		if pdf.IsReadError(err) {
			return nil, err
		}
		d.DefaultWidth = dw
	} else {
		d.DefaultWidth = dict.DefaultWidthDefault
	}

	dw2, err := decodeVDefault(x.R, cidFontDict["DW2"])
	if err != nil {
		return nil, err
	}
	d.DefaultVMetrics = dw2
	w2, err := decodeVMetrics(x.R, cidFontDict["W2"])
	if err != nil {
		return nil, err
	}
	d.VMetrics = w2

	c2g, err := x.Resolve(cidFontDict["CIDToGIDMap"])
	if err != nil {
		return nil, err
	}
	switch c2g := c2g.(type) {
	case nil:
		// pass

	case pdf.Name:
		if c2g != "Identity" {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported CIDToGIDMap: %q", c2g),
			}
		}

	case *pdf.Stream:
		in, err := pdf.DecodeStream(x.R, c2g, 0)
		if err != nil {
			return nil, err
		}
		cid2gidData, err := io.ReadAll(in)
		if err == nil && len(cid2gidData)%2 != 0 {
			err = &pdf.MalformedFileError{
				Err: errors.New("odd length CIDToGIDMap"),
			}
		}
		if err != nil {
			return nil, err
		}
		d.CIDToGID = make([]glyph.ID, len(cid2gidData)/2)
		for i := range d.CIDToGID {
			d.CIDToGID[i] = glyph.ID(cid2gidData[2*i])<<8 | glyph.ID(cid2gidData[2*i+1])
		}

	default:
		return nil, &pdf.MalformedFileError{
			Err: errors.New("missing/invalid CIDToGIDMap"),
		}
	}

	for _, key := range []pdf.Name{"FontFile2", "FontFile3"} {
		if fontFile, err := pdf.ExtractorGetOptional(x, fdDict[key],
			func(x *pdf.Extractor, obj pdf.Object) (*glyphdata.Stream, error) {
				return glyphdata.ExtractStream(x, obj, "TrueType", key)
			}); err != nil {
			return nil, err
		} else if fontFile != nil {
			d.FontFile = fontFile
			break
		}
	}

	repairCIDType2(d)

	if d.FontFile == nil && !d.CMap.IsPredefined() {
		return nil, errors.New("custom encoding not allowed for external font")
	}

	return d, nil
}

// repairCIDType2 fixes invalid data in a CIDFontType2 dictionary after extraction.
func repairCIDType2(d *dict.CIDFontType2) {
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

	if d.FontFile == nil {
		d.CIDToGID = nil
	}

	d.Descriptor.MissingWidth = 0

	if d.ROS == nil {
		if d.CMap.ROS != nil {
			d.ROS = d.CMap.ROS
		} else {
			d.ROS = &cid.SystemInfo{
				Registry:   "Adobe",
				Ordering:   "Identity",
				Supplement: 0,
			}
		}
	}

	if d.CMap.Name != "Identity-H" && d.CMap.Name != "Identity-V" ||
		!d.CMap.IsPredefined() {
		if d.CMap.ROS == nil {
			d.CMap = d.CMap.Clone()
			d.CMap.ROS = d.ROS
		} else if d.ROS.Registry != d.CMap.ROS.Registry ||
			d.ROS.Ordering != d.CMap.ROS.Ordering {
			d.CMap = d.CMap.Clone()
			d.CMap.ROS = d.ROS
		}
	}
}
