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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/name"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/post"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/locale"
)

func (info *Info) Write(w io.Writer) (int64, error) {
	blobs := make(map[string][]byte)

	headData, err := makeHead(info)
	if err != nil {
		return 0, err
	}
	blobs["head"] = headData

	hheaData, hmtxData := makeHmtx(info)
	blobs["hhea"] = hheaData
	blobs["hmtx"] = hmtxData

	if info.Glyphs != nil {
		maxpInfo := table.MaxpInfo{
			NumGlyphs: len(info.Glyphs),
		}
		maxpData, err := maxpInfo.Encode()
		if err != nil {
			return 0, err
		}
		blobs["maxp"] = maxpData
	}

	cmapSubtable := info.CMap.Encode(0)
	ss := cmap.Table{
		{PlatformID: 1, EncodingID: 0, Language: 0}: cmapSubtable,
		{PlatformID: 3, EncodingID: 1, Language: 0}: cmapSubtable,
	}
	buf := &bytes.Buffer{}
	ss.Write(buf)
	blobs["cmap"] = buf.Bytes()

	os2Data := makeOS2(info)
	blobs["OS/2"] = os2Data

	nameData := makeName(info, ss)
	blobs["name"] = nameData

	postData := makePost(info)
	blobs["post"] = postData

	cffData, err := makeCFF(info)
	if err != nil {
		return 0, err
	}
	blobs["CFF "] = cffData

	// ----------------------------------------------------------------------

	tables := []string{"head", "hhea", "hmtx", "maxp", "OS/2", "name", "cmap", "post", "CFF "}

	return sfnt.WriteTables(w, table.ScalerTypeCFF, tables, blobs)
}

func isFixedPitch(glyphs []*cff.Glyph) bool {
	var width int16

	for i := range glyphs {
		w := glyphs[i].Width
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

func makeHead(info *Info) ([]byte, error) {
	var bbox font.Rect
	first := true
	for _, g := range info.Glyphs {
		ext := g.Extent()
		if ext.IsZero() {
			continue
		}
		if first || ext.LLx < bbox.LLx {
			bbox.LLx = ext.LLx
		}
		if first || ext.LLy < bbox.LLy {
			bbox.LLy = ext.LLy
		}
		if first || ext.URx > bbox.URx {
			bbox.URx = ext.URx
		}
		if first || ext.URy > bbox.URy {
			bbox.URy = ext.URy
		}
		first = false
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
	}
	return headInfo.Encode()
}

func makeHmtx(info *Info) ([]byte, []byte) {
	widths := make([]uint16, len(info.Glyphs))
	extents := make([]font.Rect, len(info.Glyphs))
	for i, g := range info.Glyphs {
		widths[i] = uint16(g.Width)
		extents[i] = g.Extent()
	}

	hmtxInfo := &hmtx.Info{
		Width:       widths,
		GlyphExtent: extents,
		Ascent:      info.Ascent,
		Descent:     info.Descent,
		LineGap:     info.LineGap,
		CaretAngle:  info.ItalicAngle / 180 * math.Pi,
	}

	return hmtxInfo.Encode()
}

func makeOS2(info *Info) []byte {
	os2Info := &os2.Info{
		WeightClass: info.Weight,
		WidthClass:  info.Width,
		IsBold:      info.IsBold,
		IsItalic:    info.ItalicAngle != 0,
		IsRegular:   info.IsRegular,
		IsOblique:   info.IsOblique,
		Ascent:      info.Ascent,
		Descent:     info.Descent,
		LineGap:     info.LineGap,
		PermUse:     info.PermUse,
	}
	return os2Info.Encode(info.CMap)
}

func makeName(info *Info, ss cmap.Table) []byte {
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
			PostScriptName: info.PostscriptName(),
		}
	}

	return nameInfo.Encode(ss)
}

func makePost(info *Info) []byte {
	postInfo := &post.Info{
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       isFixedPitch(info.Glyphs),
	}
	return postInfo.Encode()
}

func makeCFF(info *Info) ([]byte, error) {
	q := 1 / float64(info.UnitsPerEm)
	cffInfo := &type1.FontInfo{
		FullName:   info.FullName(),
		FamilyName: info.FamilyName,
		Weight:     info.Weight.String(),
		FontName:   pdf.Name(info.PostscriptName()),
		Version:    info.Version.String(),

		Copyright: info.Copyright,
		Notice:    info.Trademark,

		FontMatrix: []float64{q, 0, 0, q, 0, 0},

		ItalicAngle:  info.ItalicAngle,
		IsFixedPitch: isFixedPitch(info.Glyphs),

		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
	}
	myCff := &cff.Font{
		Glyphs:   info.Glyphs,
		Info:     cffInfo,
		Private:  info.Private,
		FdSelect: info.FdSelect,
		Encoding: info.Encoding,
		Gid2cid:  info.Gid2cid,
		ROS:      info.ROS,
	}

	buf := &bytes.Buffer{}
	err := myCff.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
