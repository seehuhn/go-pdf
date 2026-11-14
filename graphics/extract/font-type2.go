// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package extract

import (
	"errors"
	"fmt"

	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/pdf/internal/limits"
)

// extractFontCIDType2 reads a Type 2 CIDFont dictionary from the PDF file.
func extractFontCIDType2(c pdf.Cursor, obj pdf.Object) (*dict.CIDFontType2, error) {
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
	} else if subtype != "CIDFontType2" {
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("expected CIDFontType2, got %q", subtype),
		}
	}

	d := &dict.CIDFontType2{}

	// fields in the font dictionary

	d.CMap, err = c.Decode(fontDict["Encoding"], cmap.Extract)
	if err != nil {
		return nil, err
	}

	d.ToUnicode, err = c.DecodeOptional(fontDict["ToUnicode"], cmap.ExtractToUnicode)
	if err != nil {
		return nil, err
	}

	// fields in the CIDFont dictionary

	baseFont, err := c.Name(cidFontDict["BaseFont"])
	if err != nil {
		return nil, err
	}
	d.SubsetTag, d.PostScriptName = subset.Split(string(baseFont))

	fdDict, err := c.DictTyped(cidFontDict["FontDescriptor"], "FontDescriptor")
	if pdf.IsReadError(err) {
		return nil, err
	}
	d.Descriptor, err = c.DecodeOptional(fdDict, font.ExtractDescriptor)
	if err != nil {
		return nil, err
	}

	d.ROS, err = c.DecodeOptional(cidFontDict["CIDSystemInfo"], font.ExtractCIDSystemInfo)
	if err != nil {
		return nil, err
	}

	d.Width, err = decodeCompositeWidths(c, cidFontDict["W"])
	if err != nil {
		return nil, err
	}
	if dw, err := number(c, cidFontDict["DW"]); err == nil {
		d.DefaultWidth = dw
	} else if pdf.IsReadError(err) {
		return nil, err
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

	c2g, err := c.Resolve(cidFontDict["CIDToGIDMap"])
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
		cid2gidData, err := c.ReadAll(c2g, limits.MaxCIDToGIDMapBytes)
		if err != nil {
			return nil, err
		}
		if len(cid2gidData)%2 != 0 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("odd length CIDToGIDMap"),
			}
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

	d.FontFile, err = glyphdata.ExtractFontFile(c, fdDict, "TrueType")
	if err != nil {
		return nil, err
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

	d.PostScriptName, d.SubsetTag =
		resolveFontName(d.Descriptor, d.PostScriptName, d.SubsetTag, d.FontFile != nil)

	if d.FontFile == nil {
		d.CIDToGID = nil
	}

	d.Descriptor.MissingWidth = 0

	if d.CMap == nil {
		d.CMap = cmap.IdentityH
	}

	repairROS(&d.ROS, &d.CMap)
}

// repairROS makes the character collection and the CMap of a CIDFont
// consistent.  A missing character collection is taken from the CMap, or set
// to Adobe-Identity-0.  Apart from the predefined Identity-H and Identity-V,
// the CMap is replaced by a copy with the font's character collection if the
// two are missing or incompatible.
func repairROS(ros **cid.SystemInfo, cm **cmap.File) {
	if *ros == nil {
		if (*cm).ROS != nil {
			*ros = (*cm).ROS
		} else {
			*ros = &cid.SystemInfo{
				Registry:   "Adobe",
				Ordering:   "Identity",
				Supplement: 0,
			}
		}
	}

	c := *cm
	if c.Name != "Identity-H" && c.Name != "Identity-V" || !c.IsPredefined() {
		if c.ROS == nil || (*ros).Registry != c.ROS.Registry ||
			(*ros).Ordering != c.ROS.Ordering {
			c = c.Clone()
			c.ROS = *ros
			*cm = c
		}
	}
}
