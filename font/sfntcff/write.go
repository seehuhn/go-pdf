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

package sfntcff

import (
	"bytes"
	"io"
	"math"
	"time"

	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/maxp"
	"seehuhn.de/go/pdf/font/sfnt/name"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/post"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/locale"
)

func (info *Info) Write(w io.Writer) (int64, error) {
	tables := make(map[string][]byte)

	hheaData, hmtxData := info.makeHmtx()
	tables["hhea"] = hheaData
	tables["hmtx"] = hmtxData

	var ss cmap.Table
	if info.CMap != nil {
		uniEncoding := uint16(3)
		winEncoding := uint16(1)
		if _, high := info.CMap.CodeRange(); high > 0xFFFF {
			// TODO(voss): should this check the cmap table format instead?
			uniEncoding = 4
			winEncoding = 10
		}
		cmapSubtable := info.CMap.Encode(0)
		ss = cmap.Table{
			{PlatformID: 0, EncodingID: uniEncoding}: cmapSubtable,
			{PlatformID: 3, EncodingID: winEncoding}: cmapSubtable,
		}
		tables["cmap"] = ss.Encode()
	}

	tables["OS/2"] = info.makeOS2()
	tables["name"] = info.makeName(ss)
	tables["post"] = info.makePost()

	var locaFormat int16
	var scalerType uint32
	var maxpTtf *maxp.TTFInfo
	switch outlines := info.Outlines.(type) {
	case *cff.Outlines:
		cffData, err := info.makeCFF(outlines)
		if err != nil {
			return 0, err
		}
		tables["CFF "] = cffData
		scalerType = table.ScalerTypeCFF
	case *GlyfOutlines:
		enc := outlines.Glyphs.Encode()
		tables["glyf"] = enc.GlyfData
		tables["loca"] = enc.LocaData
		locaFormat = enc.LocaFormat
		for name, data := range outlines.Tables {
			tables[name] = data
		}
		scalerType = table.ScalerTypeTrueType
		maxpTtf = outlines.Maxp
	default:
		panic("unexpected font type")
	}

	maxpInfo := &maxp.Info{
		NumGlyphs: info.NumGlyphs(),
		TTF:       maxpTtf,
	}
	tables["maxp"] = maxpInfo.Encode()

	tables["head"] = info.makeHead(locaFormat)

	return sfnt.WriteTables(w, scalerType, tables)
}

// EmbedSimple writes the binary form of the font for embedding in a PDF
// file as a simple font.
func (info *Info) EmbedSimple(w io.Writer) (int64, error) {
	tables := make(map[string][]byte)
	ss := cmap.Table{
		{PlatformID: 1, EncodingID: 0}: info.CMap.Encode(0),
	}
	tables["cmap"] = ss.Encode()

	tables["hhea"], tables["hmtx"] = info.makeHmtx()

	outlines := info.Outlines.(*GlyfOutlines)
	enc := outlines.Glyphs.Encode()
	tables["glyf"] = enc.GlyfData
	tables["loca"] = enc.LocaData
	for name, data := range outlines.Tables {
		tables[name] = data
	}

	maxpInfo := &maxp.Info{
		NumGlyphs: info.NumGlyphs(),
		TTF:       outlines.Maxp,
	}
	tables["maxp"] = maxpInfo.Encode()

	tables["head"] = info.makeHead(enc.LocaFormat)

	return sfnt.WriteTables(w, table.ScalerTypeTrueType, tables)
}

// IsFixedPitch returns true if all glyphs in the font have the same width.
func (info *Info) IsFixedPitch() bool {
	ww := info.Widths()
	if len(ww) == 0 {
		return false
	}

	var width uint16
	for _, w := range ww {
		if w == 0 {
			continue
		}
		if width == 0 {
			width = w
		} else if width != w {
			return false
		}
	}

	return true
}

func (info *Info) makeHead(locaFormat int16) []byte {
	var bbox funit.Rect
	switch outlines := info.Outlines.(type) {
	case *cff.Outlines:
		for _, g := range outlines.Glyphs {
			bbox.Extend(g.Extent())
		}
	case *GlyfOutlines:
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
		Widths:       info.fWidths(),
		GlyphExtents: info.fExtents(),
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

	os2Info := &os2.Info{
		WeightClass: info.Weight,
		WidthClass:  info.Width,

		IsBold:    info.IsBold,
		IsItalic:  info.ItalicAngle != 0,
		IsRegular: info.IsRegular,
		IsOblique: info.IsOblique,

		Ascent:    info.Ascent,
		Descent:   info.Descent,
		LineGap:   info.LineGap,
		CapHeight: info.CapHeight,
		XHeight:   info.XHeight,

		AvgGlyphWidth: int16(avgGlyphWidth),

		FamilyClass: familyClass,

		PermUse: info.PermUse,
	}
	return os2Info.Encode(info.CMap)
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

	nameInfo := &name.Info{
		Tables: map[name.Loc]*name.Table{},
	}
	fullName := info.FullName()
	for _, country := range []locale.Country{locale.CountryUSA, locale.CountryUndefined} {
		nameInfo.Tables[name.Loc{Language: locale.LangEnglish, Country: country}] = &name.Table{
			Copyright:      info.Copyright,
			Trademark:      info.Trademark,
			Family:         info.FamilyName,
			Subfamily:      info.Subfamily(),
			Identifier:     fullName + "; " + info.Version.String() + "; " + dayString,
			FullName:       fullName,
			Version:        "Version " + info.Version.String(),
			PostScriptName: string(info.PostscriptName()),
		}
	}

	return nameInfo.Encode(ss)
}

func (info *Info) makePost() []byte {
	postInfo := &post.Info{
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       info.IsFixedPitch(),
	}
	if outlines, ok := info.Outlines.(*GlyfOutlines); ok {
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
