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
	"encoding/binary"
	"io"
	"log"
	"math"
	"math/bits"
	"os"
	"sort"
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
	FullName   string
	FamilyName string
	Weight     os2.Weight
	Width      os2.Width

	// FontName is a condensation of FullName.  It is customary to remove
	// spaces and to limit its length to fewer than 40 characters. The
	// resulting name should be unique.
	FontName string

	Version head.Version

	Copyright string

	Notice string

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

func main() {
	blobs := make(map[string][]byte)

	info := &CFFInfo{
		FullName:   "Test Font",
		FamilyName: "Test",
		Weight:     os2.WeightNormal,
		Width:      os2.WidthNormal,
		Version:    0x00010000,

		Copyright: "Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>",

		UnitsPerEm: 1000,
		Ascent:     700,
		Descent:    -300,
		LineGap:    200,
	}

	var glyphs []*cff.Glyph

	g := cff.NewGlyph(".notdef", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.LineTo(500, 700)
	g.LineTo(0, 700)
	glyphs = append(glyphs, g)

	g = cff.NewGlyph("space", 550)
	glyphs = append(glyphs, g)

	g = cff.NewGlyph("A", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.LineTo(250, 710)
	glyphs = append(glyphs, g)

	g = cff.NewGlyph("B", 550)
	g.MoveTo(0, 0)
	g.LineTo(200, 0)
	g.CurveTo(300, 0, 500, 75, 500, 175)
	g.CurveTo(500, 275, 300, 350, 200, 350)
	g.CurveTo(300, 350, 500, 425, 500, 525)
	g.CurveTo(500, 625, 300, 700, 200, 700)
	g.LineTo(0, 700)
	glyphs = append(glyphs, g)

	// ----------------------------------------------------------------------

	headData, err := makeHead(info, glyphs)
	if err != nil {
		log.Fatal(err)
	}
	blobs["head"] = headData

	maxpInfo := table.MaxpInfo{
		NumGlyphs: len(glyphs),
	}
	maxpData, err := maxpInfo.Encode()
	if err != nil {
		log.Fatal(err)
	}
	blobs["maxp"] = maxpData

	hheaData, hmtxData := makeHmtx(info, glyphs)
	blobs["hhea"] = hheaData
	blobs["hmtx"] = hmtxData

	cc, ss := makeCmap(glyphs)
	buf := &bytes.Buffer{}
	ss.Write(buf)
	blobs["cmap"] = buf.Bytes()

	os2Data := makeOS2(info, cc)
	blobs["OS/2"] = os2Data

	nameData := makeName(info, ss)
	blobs["name"] = nameData

	postData := makePost(info, glyphs)
	blobs["post"] = postData

	cffData, err := makeCFF(info, glyphs)
	if err != nil {
		log.Fatal(err)
	}
	blobs["CFF "] = cffData

	// ----------------------------------------------------------------------

	tableNames := []string{"head", "hhea", "hmtx", "maxp", "OS/2", "name", "cmap", "post", "CFF "}

	out, err := os.Create("test.otf")
	if err != nil {
		log.Fatal(err)
	}
	err = writeFile(out, tableNames, blobs)
	if err != nil {
		log.Fatal(err)
	}
	err = out.Close()
	if err != nil {
		log.Fatal(err)
	}
}

// This changes the checksum in the "head" table in place.
func writeFile(w io.Writer, tableNames []string, blobs map[string][]byte) error {
	numTables := len(tableNames)

	// prepare the header
	sel := bits.Len(uint(numTables)) - 1
	offsets := &table.Offsets{
		ScalerType:    table.ScalerTypeCFF,
		NumTables:     uint16(numTables),
		SearchRange:   1 << (sel + 4),
		EntrySelector: uint16(sel),
		RangeShift:    uint16(16 * (numTables - 1<<sel)),
	}

	var totalSum uint32
	offset := uint32(12 + 16*numTables)
	records := make([]table.Record, numTables)
	for i, name := range tableNames {
		length := uint32(len(blobs[name]))
		sum := sfnt.Checksum(blobs[name])

		records[i].Tag = table.MakeTag(name)
		records[i].Length = length
		records[i].CheckSum = sum
		records[i].Offset = offset

		totalSum += sum
		offset += 4 * ((length + 3) / 4)
	}
	sort.Slice(records, func(i, j int) bool {
		return bytes.Compare(records[i].Tag[:], records[j].Tag[:]) < 0
	})

	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, offsets)
	binary.Write(buf, binary.BigEndian, records)
	headerBytes := buf.Bytes()
	totalSum += sfnt.Checksum(headerBytes)

	// fix the checksum in the "head" table
	if headData, ok := blobs["head"]; ok {
		head.PatchChecksum(headData, totalSum)
	}

	// write the tables
	_, err := w.Write(headerBytes)
	if err != nil {
		return err
	}
	var pad [3]byte
	for _, name := range tableNames {
		body := blobs[name]
		n, err := w.Write(body)
		if err != nil {
			return err
		}
		if k := n % 4; k != 0 {
			_, err := w.Write(pad[:4-k])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func isFixedPitch(glyphs []*cff.Glyph) bool {
	var width int32

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

func makeCFF(info *CFFInfo, glyphs []*cff.Glyph) ([]byte, error) {
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
		IsFixedPitch: isFixedPitch(glyphs),

		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,

		Private: []*type1.PrivateDict{
			{
				BlueValues: []int32{-10, 0, 700, 710},
			},
		},
	}
	myCff := &cff.Font{
		Info:   &cffInfo,
		Glyphs: glyphs,
	}

	buf := &bytes.Buffer{}
	err := myCff.Encode(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func makeHead(info *CFFInfo, glyphs []*cff.Glyph) ([]byte, error) {
	now := time.Now()

	var bbox font.Rect
	first := true
	for _, g := range glyphs {
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
		Created:       now,
		Modified:      now,
		FontBBox:      bbox,
		IsBold:        info.IsBold,
		IsItalic:      info.ItalicAngle != 0,
		LowestRecPPEM: 7,
	}
	return headInfo.Encode()
}

func makeHmtx(info *CFFInfo, glyphs []*cff.Glyph) ([]byte, []byte) {
	widths := make([]uint16, len(glyphs))
	extents := make([]font.Rect, len(glyphs))
	for i, g := range glyphs {
		widths[i] = uint16(g.Width)
		extents[i] = g.Extent()
	}

	hmtxInfo := &hmtx.Info{
		Width:       widths,
		GlyphExtent: extents,
		Ascent:      info.Ascent,
		Descent:     info.Descent,
		LineGap:     info.LineGap,
		CaretAngle:  float64(info.ItalicAngle) / 180 * math.Pi,
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
		PermUse:     os2.PermInstall, // TODO(voss)
	}
	return os2Info.Encode(cc)
}

func makeName(info *CFFInfo, ss cmap.Subtables) []byte {
	nameInfo := &name.Info{
		Tables: map[name.Loc]*name.Table{
			{
				Language: locale.LangEnglish,
				Country:  locale.CountryGBR,
			}: {
				Copyright: info.Copyright,
				Family:    info.FamilyName,
				FullName:  info.FullName,
				Version:   "Version " + info.Version.String(),
			},
		},
	}
	return nameInfo.Encode(ss)
}

func makeCmap(glyphs []*cff.Glyph) (cmap.Format4, cmap.Subtables) {
	cc := cmap.Format4{}
	for i, g := range glyphs {
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

func makePost(info *CFFInfo, glyphs []*cff.Glyph) []byte {
	postInfo := &post.Info{
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       isFixedPitch(glyphs),
	}
	return postInfo.Encode()
}
