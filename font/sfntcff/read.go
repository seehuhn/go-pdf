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
	"errors"
	"io"
	"math"
	"regexp"
	"strings"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/funit"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/glyf"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/maxp"
	"seehuhn.de/go/pdf/font/sfnt/name"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/post"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/font/type1"
)

// Read reads and decodes a TrueType or OpenType font file.
func Read(r io.ReaderAt) (*Info, error) {
	header, err := table.ReadHeader(r)
	if err != nil {
		return nil, err
	}

	if !(header.Has("glyf", "loca") || header.Has("CFF ")) {
		if header.Has("CFF2") {
			return nil, &font.NotSupportedError{
				SubSystem: "sfnt",
				Feature:   "CFF2-based fonts",
			}
		}
		return nil, errors.New("sfntcff: no TrueType/OpenType glyph data found")
	}

	tableReader := func(name string) (*io.SectionReader, error) {
		rec, ok := header.Toc[name]
		if !ok {
			return nil, &table.ErrNoTable{Name: name}
		}
		return io.NewSectionReader(r, int64(rec.Offset), int64(rec.Length)), nil
	}

	// we try to read the tables in the order given by
	// https://docs.microsoft.com/en-us/typography/opentype/spec/recom#optimized-table-ordering

	var headInfo *head.Info
	headFd, err := tableReader("head")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if headFd != nil {
		headInfo, err = head.Read(headFd)
		if err != nil {
			return nil, err
		}
	}

	hheaData, err := header.ReadTableBytes(r, "hhea")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	// decoded below when reading "hmtx"

	maxpFd, err := tableReader("maxp")
	if err != nil {
		return nil, err
	}
	maxpInfo, err := maxp.Read(maxpFd)
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}

	var os2Info *os2.Info
	os2Fd, err := tableReader("OS/2")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if os2Fd != nil {
		os2Info, err = os2.Read(os2Fd)
		if err != nil {
			return nil, err
		}
	}

	var hmtxInfo *hmtx.Info
	hmtxData, err := header.ReadTableBytes(r, "hmtx")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if hheaData != nil {
		hmtxInfo, err = hmtx.Decode(hheaData, hmtxData)
		if err != nil {
			return nil, err
		}
	}

	var cmapSubtable cmap.Subtable
	cmapData, err := header.ReadTableBytes(r, "cmap")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if cmapData != nil {
		cmapTable, err := cmap.Decode(cmapData)
		if err != nil {
			return nil, err
		}
		cmapSubtable, _ = cmapTable.GetBest()
	}

	var nameTable *name.Table
	nameData, err := header.ReadTableBytes(r, "name")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if nameData != nil {
		nameInfo, err := name.Decode(nameData)
		if err != nil {
			return nil, err
		}
		nameTable = nameInfo.Tables.Get()
	}

	var postInfo *post.Info
	postFd, err := tableReader("post")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if postFd != nil {
		postInfo, err = post.Read(postFd)
		if err != nil {
			return nil, err
		}
	}

	var numGlyphs int
	if maxpInfo != nil {
		numGlyphs = maxpInfo.NumGlyphs
	}
	if hmtxInfo != nil && len(hmtxInfo.Widths) > 0 {
		if numGlyphs == 0 {
			numGlyphs = len(hmtxInfo.Widths)
		} else if len(hmtxInfo.Widths) > numGlyphs {
			// Fix up a problem founs in some fonts
			hmtxInfo.Widths = hmtxInfo.Widths[:numGlyphs]
		} else if len(hmtxInfo.Widths) != numGlyphs {
			return nil, errors.New("sfnt: hmtx and maxp glyph count mismatch")
		}
	}

	// Read the glyph data.
	var Outlines interface{}
	var fontInfo *type1.FontInfo
	switch header.ScalerType {
	case table.ScalerTypeCFF:
		var cffInfo *cff.Font
		cffFd, err := tableReader("CFF ")
		if err != nil {
			return nil, err
		}
		cffInfo, err = cff.Read(cffFd)
		if err != nil {
			return nil, err
		}
		fontInfo = cffInfo.FontInfo
		Outlines = cffInfo.Outlines

		if numGlyphs == 0 {
			numGlyphs = len(cffInfo.Glyphs)
		} else if len(cffInfo.Glyphs) != numGlyphs {
			return nil, errors.New("sfnt: cff glyph count mismatch")
		} else if hmtxInfo != nil && len(hmtxInfo.Widths) > 0 {
			for i, w := range hmtxInfo.Widths {
				cffInfo.Glyphs[i].Width = w
			}
		}
	case table.ScalerTypeTrueType, table.ScalerTypeApple:
		if headInfo == nil {
			return nil, &table.ErrNoTable{Name: "head"}
		}
		if maxpInfo == nil {
			return nil, &table.ErrNoTable{Name: "maxp"}
		}

		locaData, err := header.ReadTableBytes(r, "loca")
		if err != nil {
			return nil, err
		}
		glyfData, err := header.ReadTableBytes(r, "glyf")
		if err != nil {
			return nil, err
		}
		enc := &glyf.Encoded{
			GlyfData:   glyfData,
			LocaData:   locaData,
			LocaFormat: headInfo.LocaFormat,
		}
		ttGlyphs, err := glyf.Decode(enc)
		if err != nil {
			return nil, err
		}

		tables := make(map[string][]byte)
		for _, name := range []string{"cvt ", "fpgm", "prep", "gasp"} {
			if !header.Has(name) {
				continue
			}
			data, err := header.ReadTableBytes(r, name)
			if err != nil {
				return nil, err
			}
			tables[name] = data
		}

		if numGlyphs == 0 {
			numGlyphs = len(ttGlyphs)
		} else if len(ttGlyphs) != numGlyphs {
			return nil, errors.New("sfnt: ttf glyph count mismatch")
		}

		var widths []funit.Uint16
		if hmtxInfo != nil && len(hmtxInfo.Widths) > 0 {
			widths = hmtxInfo.Widths
		}

		var names []string
		if postInfo != nil {
			names = postInfo.Names
		}
		Outlines = &GlyfOutlines{
			Widths: widths,
			Glyphs: ttGlyphs,
			Tables: tables,
			Maxp:   maxpInfo.TTF,
			Names:  names,
		}
	default:
		panic("unexpected scaler type")
	}

	// Merge the information from the various tables.
	info := &Info{
		Outlines: Outlines,
		CMap:     cmapSubtable,
	}

	if nameTable != nil {
		info.FamilyName = nameTable.Family
	}
	if info.FamilyName == "" && fontInfo != nil {
		info.FamilyName = fontInfo.FamilyName
	}
	if os2Info != nil {
		info.Width = os2Info.WidthClass
		info.Weight = os2Info.WeightClass
	}
	if info.Weight == 0 && fontInfo != nil {
		info.Weight = font.WeightFromString(fontInfo.Weight)
	}

	if ver, ok := getNameTableVersion(nameTable); ok {
		info.Version = ver
	} else if headInfo != nil {
		info.Version = headInfo.FontRevision.Round()
	} else if ver, ok := getCFFVersion(fontInfo); ok {
		info.Version = ver
	}
	if headInfo != nil {
		info.CreationTime = headInfo.Created
		info.ModificationTime = headInfo.Modified
	}

	if nameTable != nil {
		info.Copyright = nameTable.Copyright
		info.Trademark = nameTable.Trademark
	} else if fontInfo != nil {
		info.Copyright = fontInfo.Copyright
		info.Trademark = fontInfo.Notice
	}
	if os2Info != nil {
		info.PermUse = os2Info.PermUse
	}

	if headInfo != nil {
		info.UnitsPerEm = headInfo.UnitsPerEm
		// TODO(voss): check Info.FontMatrix (and private dicts?)
	} else {
		info.UnitsPerEm = 1000
	}

	if os2Info != nil {
		info.Ascent = os2Info.Ascent
		info.Descent = os2Info.Descent
		info.LineGap = os2Info.LineGap
	} else if hmtxInfo != nil {
		info.Ascent = hmtxInfo.Ascent
		info.Descent = hmtxInfo.Descent
		info.LineGap = hmtxInfo.LineGap
	}

	if os2Info != nil {
		info.CapHeight = os2Info.CapHeight
		info.XHeight = os2Info.XHeight
	}
	if info.CapHeight == 0 && cmapSubtable != nil {
		gid := cmapSubtable.Lookup('H')
		if gid != 0 {
			info.CapHeight = info.fGlyphHeight(gid)
		}
	}
	if info.XHeight == 0 && cmapSubtable != nil {
		gid := cmapSubtable.Lookup('x')
		if gid != 0 {
			info.XHeight = info.fGlyphHeight(gid)
		}
	}

	if postInfo != nil {
		info.ItalicAngle = postInfo.ItalicAngle
	} else if fontInfo != nil {
		info.ItalicAngle = fontInfo.ItalicAngle
	} else if hmtxInfo != nil {
		info.ItalicAngle = hmtxInfo.CaretAngle * 180 / math.Pi
	}

	if postInfo != nil {
		info.UnderlinePosition = postInfo.UnderlinePosition
		info.UnderlineThickness = postInfo.UnderlineThickness
	} else if fontInfo != nil {
		info.UnderlinePosition = fontInfo.UnderlinePosition
		info.UnderlineThickness = fontInfo.UnderlineThickness
	}

	// Currently we set IsItalic if there is any evidence of the font being
	// slanted.  Are we too eager setting this?
	// TODO(voss): check that this gives good results
	info.IsItalic = info.ItalicAngle != 0
	if headInfo != nil && headInfo.IsItalic {
		info.IsItalic = true
	}
	if os2Info != nil && (os2Info.IsItalic || os2Info.IsOblique) {
		info.IsItalic = true
	}
	if nameTable != nil && strings.Contains(nameTable.Subfamily, "Italic") {
		info.IsItalic = true
	}

	if os2Info != nil {
		info.IsBold = os2Info.IsBold
	} else if headInfo != nil {
		info.IsBold = headInfo.IsBold
	}
	// TODO(voss): we could also check info.Weight == font.WeightBold
	if nameTable != nil &&
		strings.Contains(nameTable.Subfamily, "Bold") &&
		!strings.Contains(nameTable.Subfamily, "Semi Bold") &&
		!strings.Contains(nameTable.Subfamily, "Extra Bold") {
		info.IsBold = true
	}

	if os2Info != nil {
		info.IsRegular = os2Info.IsRegular
	}
	// if !(info.IsItalic || info.IsBold) && nameTable != nil && nameTable.Subfamily == "Regular" {
	// 	info.IsRegular = true
	// }

	if os2Info != nil {
		info.IsOblique = os2Info.IsOblique
	}

	if os2Info != nil {
		switch os2Info.FamilyClass >> 8 {
		case 1, 2, 3, 4, 5, 7:
			info.IsSerif = true
		case 10:
			info.IsScript = true
		}
	}

	return info, nil
}

var nameTableVersionPat = regexp.MustCompile(`^Version (\d+\.?\d+)`)

func getNameTableVersion(t *name.Table) (head.Version, bool) {
	if t == nil {
		return 0, false
	}
	v, err := head.VersionFromString(t.Version)
	if err != nil {
		return 0, false
	}
	return v, true
}

var cffVersionPat = regexp.MustCompile(`^(?:Version )?(\d+\.?\d+)`)

func getCFFVersion(fontInfo *type1.FontInfo) (head.Version, bool) {
	if fontInfo == nil || fontInfo.Version == "" {
		return 0, false
	}
	v, err := head.VersionFromString(fontInfo.Version)
	if err != nil {
		return 0, false
	}
	return v, true
}
