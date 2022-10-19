// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package sfnt

import (
	"bytes"
	"io"
	"math"
	"time"

	"seehuhn.de/go/pdf/sfnt/cff"
	"seehuhn.de/go/pdf/sfnt/cmap"
	"seehuhn.de/go/pdf/sfnt/funit"
	"seehuhn.de/go/pdf/sfnt/glyf"
	"seehuhn.de/go/pdf/sfnt/head"
	"seehuhn.de/go/pdf/sfnt/header"
	"seehuhn.de/go/pdf/sfnt/hmtx"
	"seehuhn.de/go/pdf/sfnt/maxp"
	"seehuhn.de/go/pdf/sfnt/name"
	"seehuhn.de/go/pdf/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/sfnt/os2"
	"seehuhn.de/go/pdf/sfnt/post"
)

func (info *Info) Write(w io.Writer) (int64, error) {
	tableData := make(map[string][]byte)

	hheaData, hmtxData := info.makeHmtx()
	tableData["hhea"] = hheaData
	tableData["hmtx"] = hmtxData

	var ss cmap.Table
	if info.CMap != nil {
		uniEncoding := uint16(3)
		winEncoding := uint16(1)
		if _, high := info.CMap.CodeRange(); high > 0xFFFF {
			uniEncoding = 4
			winEncoding = 10
		}
		cmapSubtable := info.CMap.Encode(0)
		ss = cmap.Table{
			{PlatformID: 0, EncodingID: uniEncoding}: cmapSubtable,
			{PlatformID: 3, EncodingID: winEncoding}: cmapSubtable,
		}
		tableData["cmap"] = ss.Encode()
	}

	tableData["OS/2"] = info.makeOS2()
	tableData["name"] = info.makeName(ss)
	tableData["post"] = info.makePost()

	var locaFormat int16
	var scalerType uint32
	var maxpTtf *maxp.TTFInfo
	switch outlines := info.Outlines.(type) {
	case *cff.Outlines:
		cffData, err := info.makeCFF(outlines)
		if err != nil {
			return 0, err
		}
		tableData["CFF "] = cffData
		scalerType = header.ScalerTypeCFF
	case *glyf.Outlines:
		enc := outlines.Glyphs.Encode()
		tableData["glyf"] = enc.GlyfData
		tableData["loca"] = enc.LocaData
		locaFormat = enc.LocaFormat
		for name, data := range outlines.Tables {
			tableData[name] = data
		}
		scalerType = header.ScalerTypeTrueType
		maxpTtf = outlines.Maxp
	default:
		panic("unexpected font type")
	}

	maxpInfo := &maxp.Info{
		NumGlyphs: info.NumGlyphs(),
		TTF:       maxpTtf,
	}
	tableData["maxp"] = maxpInfo.Encode()

	tableData["head"] = info.makeHead(locaFormat)

	if info.Gdef != nil {
		tableData["GDEF"] = info.Gdef.Encode()
	}
	if info.Gsub != nil {
		tableData["GSUB"] = info.Gsub.Encode(gtab.GsubExtensionLookupType)
	}
	if info.Gpos != nil {
		tableData["GPOS"] = info.Gpos.Encode(gtab.GposExtensionLookupType)
	}

	return header.Write(w, scalerType, tableData)
}

// Embed writes the binary form of the font for embedding in a PDF file.
func (info *Info) Embed(w io.Writer) (int64, error) {
	tableData := make(map[string][]byte)

	if info.CMap != nil {
		ss := cmap.Table{
			{PlatformID: 1, EncodingID: 0}: info.CMap.Encode(0),
		}
		tableData["cmap"] = ss.Encode()
	}

	tableData["hhea"], tableData["hmtx"] = info.makeHmtx()

	outlines := info.Outlines.(*glyf.Outlines)
	enc := outlines.Glyphs.Encode()
	tableData["glyf"] = enc.GlyfData
	tableData["loca"] = enc.LocaData
	for name, data := range outlines.Tables {
		tableData[name] = data
	}

	maxpInfo := &maxp.Info{
		NumGlyphs: info.NumGlyphs(),
		TTF:       outlines.Maxp,
	}
	tableData["maxp"] = maxpInfo.Encode()

	tableData["head"] = info.makeHead(enc.LocaFormat)

	return header.Write(w, header.ScalerTypeTrueType, tableData)
}

func (info *Info) makeHead(locaFormat int16) []byte {
	var bbox funit.Rect
	switch outlines := info.Outlines.(type) {
	case *cff.Outlines:
		for _, g := range outlines.Glyphs {
			bbox.Extend(g.Extent())
		}
	case *glyf.Outlines:
		for _, g := range outlines.Glyphs {
			if g == nil {
				continue
			}
			bbox.Extend(g.Rect)
		}
	}

	headInfo := head.Info{
		FontRevision:  info.Version,
		HasYBaseAt0:   true,
		HasXBaseAt0:   true,
		UnitsPerEm:    info.UnitsPerEm,
		Created:       info.CreationTime,
		Modified:      info.ModificationTime,
		FontBBox:      bbox,
		IsBold:        info.IsBold,
		IsItalic:      info.ItalicAngle != 0,
		LowestRecPPEM: 7,
		LocaFormat:    locaFormat,
	}
	return headInfo.Encode()
}

func (info *Info) makeHmtx() ([]byte, []byte) {
	hmtxInfo := &hmtx.Info{
		Widths:       info.Widths(),
		GlyphExtents: info.Extents(),
		Ascent:       info.Ascent,
		Descent:      info.Descent,
		LineGap:      info.LineGap,
		CaretAngle:   info.ItalicAngle / 180 * math.Pi,
	}

	return hmtxInfo.Encode()
}

func (info *Info) makeOS2() []byte {
	avgGlyphWidth := 0
	count := 0
	ww := info.Widths()
	for _, w := range ww {
		if w > 0 {
			avgGlyphWidth += int(w)
			count++
		}
	}
	if count > 0 {
		avgGlyphWidth = (avgGlyphWidth + count/2) / count
	}

	var familyClass int16
	if info.IsSerif {
		familyClass = 3 << 8
	} else if info.IsScript {
		familyClass = 10 << 8
	}

	var firstCharIndex, lastCharIndex uint16
	if info.CMap != nil {
		low, high := info.CMap.CodeRange()
		firstCharIndex = uint16(low)
		if low > 0xFFFF {
			firstCharIndex = 0xFFFF
		}
		lastCharIndex = uint16(high)
		if high > 0xFFFF {
			lastCharIndex = 0xFFFF
		}
	}

	os2Info := &os2.Info{
		WeightClass: info.Weight,
		WidthClass:  info.Width,

		IsBold:    info.IsBold,
		IsItalic:  info.ItalicAngle != 0,
		IsRegular: info.IsRegular,
		IsOblique: info.IsOblique,

		FirstCharIndex: firstCharIndex,
		LastCharIndex:  lastCharIndex,

		Ascent:    info.Ascent,
		Descent:   info.Descent,
		LineGap:   info.LineGap,
		CapHeight: info.CapHeight,
		XHeight:   info.XHeight,

		AvgGlyphWidth: funit.Int16(avgGlyphWidth),

		FamilyClass: familyClass,

		PermUse: info.PermUse,
	}
	return os2Info.Encode()
}

func (info *Info) makeName(ss cmap.Table) []byte {
	day := info.ModificationTime
	if day.IsZero() {
		day = info.CreationTime
	}
	if day.IsZero() {
		day = time.Now()
	}
	dayString := day.Format("2006-01-02")

	fullName := info.FullName()
	nameTable := &name.Table{
		Family:         info.FamilyName,
		Subfamily:      info.Subfamily(),
		Description:    info.Description,
		Copyright:      info.Copyright,
		Trademark:      info.Trademark,
		Identifier:     fullName + "; " + info.Version.String() + "; " + dayString,
		FullName:       fullName,
		Version:        "Version " + info.Version.String(),
		PostScriptName: info.PostscriptName(),
		SampleText:     info.SampleText,
	}
	nameInfo := &name.Info{
		Mac: name.Tables{
			"en": nameTable,
		},
		Windows: name.Tables{
			"en-US": nameTable,
		},
	}

	return nameInfo.Encode(1)
}

func (info *Info) makePost() []byte {
	postInfo := &post.Info{
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       info.IsFixedPitch(),
	}
	if outlines, ok := info.Outlines.(*glyf.Outlines); ok {
		postInfo.Names = outlines.Names
	}
	return postInfo.Encode()
}

func (info *Info) makeCFF(outlines *cff.Outlines) ([]byte, error) {
	fontInfo := info.GetFontInfo()
	myCff := &cff.Font{
		FontInfo: fontInfo,
		Outlines: outlines,
	}

	buf := &bytes.Buffer{}
	err := myCff.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
