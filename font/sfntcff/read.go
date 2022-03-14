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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/name"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/post"
	"seehuhn.de/go/pdf/font/sfnt/table"
	"seehuhn.de/go/pdf/font/type1"
)

// Read reads an OpenType font from a file.
func Read(r io.ReaderAt) (*Info, error) {
	header, err := table.ReadHeader(r)
	if err != nil {
		return nil, err
	}

	tableReader := func(name string) (*io.SectionReader, error) {
		rec, ok := header.Toc[name]
		if !ok {
			return nil, &table.ErrNoTable{Name: name}
		}
		return io.NewSectionReader(r, int64(rec.Offset), int64(rec.Length)), nil
	}

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

	var hmtxInfo *hmtx.Info
	hheaData, err := header.ReadTableBytes(r, "hhea")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
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

	// maxpFd, err := tableReader("maxp")
	// if err != nil {
	// 	return nil, err
	// }
	// maxpInfo, err := table.ReadMaxp(maxpFd)
	// if err != nil {
	// 	return nil, err
	// }
	// numGlyphs := maxpInfo.NumGlyphs

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

	cmapData, err := header.ReadTableBytes(r, "cmap")
	if err != nil {
		return nil, err
	}
	cmapTable, err := cmap.Decode(cmapData)
	if err != nil {
		return nil, err
	}
	cmapSubtable, err := cmapTable.GetBest()
	if err != nil {
		return nil, err
	}

	var postInfo *post.Info
	postFd, err := tableReader("post")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if postFd != nil {
		postInfo, err = post.Read(postFd)
		if err != nil && !font.IsUnsupported(err) {
			return nil, err
		}
	}

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

		if hmtxInfo != nil && len(hmtxInfo.Widths) > 0 {
			if len(hmtxInfo.Widths) != len(cffInfo.Glyphs) {
				return nil, errors.New("sfnt: hmtx and cff glyph count mismatch")
			}
			for i, w := range hmtxInfo.Widths {
				cffInfo.Glyphs[i].Width = w
			}
		}
	case table.ScalerTypeTrueType, table.ScalerTypeApple:
		return nil, errors.New("not implemented")
		// panic("not implemented")
	default:
		panic("unexpected scaler type")
	}

	info := &Info{}

	if nameTable != nil {
		info.FamilyName = nameTable.Family
	} else if fontInfo != nil {
		info.FamilyName = fontInfo.FamilyName
	}
	if os2Info != nil {
		info.Width = os2Info.WidthClass
		info.Weight = os2Info.WeightClass
		//   ALT: info.Weight = os2.WeightFromString(cffInfo.Info.Weight)
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
		//   ALT: cffInfo.Info.FontMatrix
	} else {
		info.UnitsPerEm = 1000
	}
	if hmtxInfo != nil {
		info.Ascent = hmtxInfo.Ascent
		info.Descent = hmtxInfo.Descent
		info.LineGap = hmtxInfo.LineGap
	} else if os2Info != nil {
		info.Ascent = os2Info.Ascent
		info.Descent = os2Info.Descent
		info.LineGap = os2Info.LineGap
	}
	if hmtxInfo != nil {
		info.ItalicAngle = hmtxInfo.CaretAngle * 180 / math.Pi
	} else if postInfo != nil {
		info.ItalicAngle = postInfo.ItalicAngle
	} else if fontInfo != nil {
		info.ItalicAngle = fontInfo.ItalicAngle
	}
	if postInfo != nil {
		info.UnderlinePosition = postInfo.UnderlinePosition
		info.UnderlineThickness = postInfo.UnderlineThickness
	} else if fontInfo != nil {
		info.UnderlinePosition = fontInfo.UnderlinePosition
		info.UnderlineThickness = fontInfo.UnderlineThickness
	}
	if headInfo != nil {
		info.IsBold = headInfo.IsBold
	} else if os2Info != nil {
		info.IsBold = os2Info.IsBold
	}
	if os2Info != nil {
		info.IsRegular = os2Info.IsRegular
		info.IsOblique = os2Info.IsOblique
	}

	info.Font = Outlines

	info.CMap = cmapSubtable

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
