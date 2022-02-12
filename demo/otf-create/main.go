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
	"math/bits"
	"os"
	"sort"
	"time"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/post"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/font/type1"
)

type Info struct {
	Version head.Version
	Weight  os2.Weight
	Width   os2.Width

	UnitsPerEm uint16

	Ascent  int16
	Descent int16
	LineGap int16

	ItalicAngle        int32 // Italic angle (degrees counterclockwise from vertical)
	UnderlinePosition  int16 // Underline position (negative)
	UnderlineThickness int16 // Underline thickness

	IsBold       bool
	IsItalic     bool
	HasUnderline bool
	IsOutlined   bool
	IsRegular    bool
	IsOblique    bool
	IsFixedPitch bool
}

func makeCFF(info *Info, glyphs []*cff.Glyph) ([]byte, error) {
	q := 1 / float64(info.UnitsPerEm)
	cffInfo := type1.FontInfo{
		FontName:   "Test",
		Version:    info.Version.String(),
		Notice:     "https://scripts.sil.org/OFL",
		Copyright:  "Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>",
		FullName:   "Test",
		FamilyName: "Test",
		Weight:     info.Weight.String(),
		FontMatrix: []float64{q, 0, 0, q, 0, 0},

		ItalicAngle: info.ItalicAngle,

		IsFixedPitch: info.IsFixedPitch,

		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,

		IsOutlined: info.IsOutlined,

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

func makeHead(info *Info, glyphs []*cff.Glyph) ([]byte, error) {
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
		LowestRecPPEM: 7,
	}
	return headInfo.Encode()
}

func makeHmtx(info *Info, glyphs []*cff.Glyph) ([]byte, []byte) {
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
	}

	return hmtxInfo.Encode()
}

func makeOS2(info *Info) []byte {
	os2Info := &os2.Info{
		WeightClass:  info.Weight,
		WidthClass:   info.Width,
		IsBold:       info.IsBold,
		IsItalic:     info.IsItalic,
		HasUnderline: info.HasUnderline,
		IsOutlined:   info.IsOutlined,
		IsRegular:    info.IsRegular,
		IsOblique:    info.IsOblique,
		Ascent:       info.Ascent,
		Descent:      info.Descent,
		LineGap:      info.LineGap,
		PermUse:      os2.PermInstall,
	}
	return os2Info.Encode()
}

func makePost(info *Info) []byte {
	postInfo := &post.Info{
		ItalicAngle:        info.ItalicAngle,
		UnderlinePosition:  info.UnderlinePosition,
		UnderlineThickness: info.UnderlineThickness,
		IsFixedPitch:       info.IsFixedPitch,
	}
	return postInfo.Encode()
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

func main() {
	blobs := make(map[string][]byte)

	info := &Info{
		Version: 0x00010000,
		Weight:  os2.WeightNormal,
		Width:   os2.WidthNormal,

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

	g = cff.NewGlyph("A", 550)
	g.MoveTo(0, 0)
	g.LineTo(500, 0)
	g.MoveTo(250, 710)
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

	os2Data := makeOS2(info)
	blobs["OS/2"] = os2Data

	postData := makePost(info)
	blobs["post"] = postData

	cffData, err := makeCFF(info, glyphs)
	if err != nil {
		log.Fatal(err)
	}
	blobs["CFF "] = cffData

	// ----------------------------------------------------------------------

	// "head", "hhea", "maxp", "OS/2", "name", "cmap", "post", "CFF "
	tableNames := []string{"head", "hhea", "hmtx", "maxp", "OS/2", "post", "CFF "}

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
