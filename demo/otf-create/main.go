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

package main

import (
	"bytes"
	"io"
	"log"
	"math"
	"os"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/names"
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

// CFFInfo contains information about the font.
type CFFInfo struct {
	Glyphs []*cff.Glyph

	FullName   string
	FamilyName string
	Weight     os2.Weight
	Width      os2.Width

	// FontName is a condensation of FullName.  It is customary to remove
	// spaces and to limit its length to fewer than 40 characters. The
	// resulting name should be unique.
	FontName string

	Version          head.Version
	CreationTime     time.Time
	ModificationTime time.Time

	Copyright string
	Notice    string
	PermUse   os2.Permissions

	UnitsPerEm uint16

	Ascent  int16
	Descent int16
	LineGap int16

	ItalicAngle        float64 // Italic angle (degrees counterclockwise from vertical)
	UnderlinePosition  int16   // Underline position (negative)
	UnderlineThickness int16   // Underline thickness

	IsBold    bool
	IsRegular bool
	IsOblique bool
}

func Read(fname string) (*CFFInfo, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	header, err := table.ReadHeader(fd)
	if err != nil {
		return nil, err
	}

	tableReader := func(name string) (*io.SectionReader, error) {
		rec := header.Find(name)
		if rec == nil {
			return nil, &table.ErrNoTable{Name: name}
		}
		return io.NewSectionReader(fd, int64(rec.Offset), int64(rec.Length)), nil
	}

	info := &CFFInfo{}

	// TODO(voss): handle missing tables

	headFd, err := tableReader("head")
	if err != nil {
		return nil, err
	}
	headInfo, err := head.Read(headFd)
	if err != nil {
		return nil, err
	}

	hheaData, err := header.ReadTableBytes(fd, "hhea")
	if err != nil {
		return nil, err
	}
	hmtxData, err := header.ReadTableBytes(fd, "hmtx")
	if err != nil {
		return nil, err
	}
	hmtxInfo, err := hmtx.Decode(hheaData, hmtxData)
	if err != nil {
		return nil, err
	}

	maxpFd, err := tableReader("maxp")
	if err != nil {
		return nil, err
	}
	maxpInfo, err := table.ReadMaxp(maxpFd)
	if err != nil {
		return nil, err
	}
	numGlyphs := maxpInfo.NumGlyphs

	os2Fd, err := tableReader("OS/2")
	if err != nil {
		return nil, err
	}
	os2Info, err := os2.Read(os2Fd)
	if err != nil {
		return nil, err
	}

	nameData, err := header.ReadTableBytes(fd, "name")
	if err != nil {
		return nil, err
	}
	nameInfo, err := name.Decode(nameData)
	if err != nil {
		return nil, err
	}

	// info.Glyphs =
	// info.FullName =
	// info.FamilyName =
	info.Weight = os2Info.WeightClass
	info.Width = os2Info.WidthClass
	// info.FontName =
	info.Version = headInfo.FontRevision
	info.CreationTime = headInfo.Created
	info.ModificationTime = headInfo.Modified
	// info.Copyright =
	// info.Notice =
	info.PermUse = os2Info.PermUse
	info.UnitsPerEm = headInfo.UnitsPerEm
	info.Ascent = hmtxInfo.Ascent
	// ALT: info.Ascent = os2Info.Ascent
	info.Descent = hmtxInfo.Descent
	// ALT: info.Descent = os2Info.Descent
	info.LineGap = hmtxInfo.LineGap
	// ALT: info.LineGap = os2Info.LineGap
	info.ItalicAngle = hmtxInfo.CaretAngle * 180 / math.Pi
	// info.UnderlinePosition =
	// info.UnderlineThickness =
	info.IsBold = headInfo.IsBold
	// ALT: info.IsBold = os2Info.IsBold
	info.IsRegular = os2Info.IsRegular
	info.IsOblique = os2Info.IsOblique

	_ = nameInfo
	_ = numGlyphs

	return info, nil
}

func main() {
	blobs := make(map[string][]byte)

	now := time.Now()
	info := &CFFInfo{
		FullName:   "Test Font",
		FamilyName: "Test",
		Weight:     os2.WeightNormal,
		Width:      os2.WidthNormal,

		Version:          0x00010000,
		CreationTime:     now,
		ModificationTime: now,

		Copyright: "Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>",
		PermUse:   os2.PermInstall,

		UnitsPerEm: 1000,
		Ascent:     700,
		Descent:    -300,
		LineGap:    200,
	}

	g := cff.NewGlyph(".notdef", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.LineTo(500, 700)
	g.LineTo(0, 700)
	info.Glyphs = append(info.Glyphs, g)

	g = cff.NewGlyph("space", 550)
	info.Glyphs = append(info.Glyphs, g)

	g = cff.NewGlyph("A", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.LineTo(250, 710)
	info.Glyphs = append(info.Glyphs, g)

	g = cff.NewGlyph("B", 550)
	g.MoveTo(0, 0)
	g.LineTo(200, 0)
	g.CurveTo(300, 0, 500, 75, 500, 175)
	g.CurveTo(500, 275, 300, 350, 200, 350)
	g.CurveTo(300, 350, 500, 425, 500, 525)
	g.CurveTo(500, 625, 300, 700, 200, 700)
	g.LineTo(0, 700)
	info.Glyphs = append(info.Glyphs, g)

	// ----------------------------------------------------------------------

	headData, err := makeHead(info)
	if err != nil {
		log.Fatal(err)
	}
	blobs["head"] = headData

	hheaData, hmtxData := makeHmtx(info)
	blobs["hhea"] = hheaData
	blobs["hmtx"] = hmtxData

	maxpInfo := table.MaxpInfo{
		NumGlyphs: len(info.Glyphs),
	}
	maxpData, err := maxpInfo.Encode()
	if err != nil {
		log.Fatal(err)
	}
	blobs["maxp"] = maxpData

	cc, ss := makeCmap(info)
	buf := &bytes.Buffer{}
	ss.Write(buf)
	blobs["cmap"] = buf.Bytes()

	os2Data := makeOS2(info, cc)
	blobs["OS/2"] = os2Data

	nameData := makeName(info, ss)
	blobs["name"] = nameData

	postData := makePost(info)
	blobs["post"] = postData

	cffData, err := makeCFF(info)
	if err != nil {
		log.Fatal(err)
	}
	blobs["CFF "] = cffData

	// ----------------------------------------------------------------------

	tables := []string{"head", "hhea", "hmtx", "maxp", "OS/2", "name", "cmap", "post", "CFF "}

	out, err := os.Create("test.otf")
	if err != nil {
		log.Fatal(err)
	}
	_, err = sfnt.WriteTables(out, table.ScalerTypeCFF, tables, blobs)
	if err != nil {
		log.Fatal(err)
	}
	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
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

func makeCFF(info *CFFInfo) ([]byte, error) {
	q := 1 / float64(info.UnitsPerEm)
	cffInfo := type1.FontInfo{
		FullName:   info.FullName,
		FamilyName: info.FamilyName,
		Weight:     info.Weight.String(),
		FontName:   pdf.Name(info.FontName),
		Version:    info.Version.String(),

		Copyright: info.Copyright,
		Notice:    info.Notice, // TODO(voss)

		FontMatrix: []float64{q, 0, 0, q, 0, 0},

		ItalicAngle:  info.ItalicAngle,
		IsFixedPitch: isFixedPitch(info.Glyphs),

		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,

		Private: []*type1.PrivateDict{
			{
				BlueValues: []int32{-10, 0, 700, 710}, // TODO(voss)
			},
		},
	}
	myCff := &cff.Font{
		Info:   &cffInfo,
		Glyphs: info.Glyphs,
	}

	buf := &bytes.Buffer{}
	err := myCff.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func makeHead(info *CFFInfo) ([]byte, error) {
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

func makeHmtx(info *CFFInfo) ([]byte, []byte) {
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

func makeOS2(info *CFFInfo, cc cmap.Subtable) []byte {
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
	return os2Info.Encode(cc)
}

func makeName(info *CFFInfo, ss cmap.Subtables) []byte {
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
	// TODO(voss): make this more convenient
	for _, country := range []locale.Country{locale.CountryUSA, locale.CountryGBR, locale.CountryUndefined} {
		nameInfo.Tables[name.Loc{Language: locale.LangEnglish, Country: country}] = &name.Table{
			Copyright:      info.Copyright,
			Family:         info.FamilyName,
			Subfamily:      "Regular",
			Identifier:     info.FullName + "; " + info.Version.String() + "; " + dayString,
			FullName:       info.FullName,
			Version:        "Version " + info.Version.String(),
			PostScriptName: "Test",
		}
	}

	return nameInfo.Encode(ss)
}

func makeCmap(info *CFFInfo) (cmap.Format4, cmap.Subtables) {
	cc := cmap.Format4{}
	for i, g := range info.Glyphs {
		rr := names.ToUnicode(string(g.Name), false)
		if len(rr) == 1 {
			r := uint16(rr[0])
			if _, ok := cc[r]; !ok {
				cc[r] = font.GlyphID(i)
			}
		}
	}
	cmapSubtable := cc.Encode(0)
	ss := cmap.Subtables{
		{PlatformID: 1, EncodingID: 0, Language: 0, Data: cmapSubtable},
		{PlatformID: 3, EncodingID: 1, Language: 0, Data: cmapSubtable},
	}
	return cc, ss
}

func makePost(info *CFFInfo) []byte {
	postInfo := &post.Info{
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       isFixedPitch(info.Glyphs),
	}
	return postInfo.Encode()
}
