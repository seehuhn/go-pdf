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
	"io"
	"math"
	"regexp"
	"strconv"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/sfnt/cmap"
	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/hmtx"
	"seehuhn.de/go/pdf/font/sfnt/name"
	"seehuhn.de/go/pdf/font/sfnt/os2"
	"seehuhn.de/go/pdf/font/sfnt/post"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

type ReaderReaderAt interface {
	io.Reader
	io.ReaderAt
}

// Read reads an OpenType font from a file.
// TODO(voss): make this work with an io.ReaderAt
func Read(r ReaderReaderAt) (*Info, error) {
	header, err := table.ReadHeader(r)
	if err != nil {
		return nil, err
	}

	tableReader := func(name string) (*io.SectionReader, error) {
		rec := header.Find(name)
		if rec == nil {
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

	var cffInfo *cff.Font
	cffFd, err := tableReader("CFF ")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	if cffFd != nil {
		cffInfo, err = cff.Read(cffFd)
		if err != nil {
			return nil, err
		}
	}

	// TODO(voss)
	if cffInfo == nil || cffInfo.IsCIDFont {
		return nil, &font.NotSupportedError{
			SubSystem: "sfntcff",
			Feature:   "CID fonts",
		}
	}

	info := &Info{}

	if nameTable != nil {
		info.FamilyName = nameTable.Family
	} else if cffInfo != nil {
		info.FamilyName = cffInfo.Info.FamilyName
	}
	if os2Info != nil {
		info.Width = os2Info.WidthClass
		info.Weight = os2Info.WeightClass
		//   ALT: info.Weight = os2.WeightFromString(cffInfo.Info.Weight)
	}

	if ver, ok := getNameTableVersion(nameTable); ok {
		info.Version = ver
	} else if headInfo != nil {
		info.Version = headInfo.FontRevision
	} else if ver, ok := getCFFVersion(cffInfo); ok {
		info.Version = ver
	}
	if headInfo != nil {
		info.CreationTime = headInfo.Created
		info.ModificationTime = headInfo.Modified
	}

	if nameTable != nil {
		info.Copyright = nameTable.Copyright
		info.Trademark = nameTable.Trademark
	} else if cffInfo != nil {
		info.Copyright = cffInfo.Info.Copyright
		info.Trademark = cffInfo.Info.Notice
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
	} else if cffInfo != nil {
		info.ItalicAngle = cffInfo.Info.ItalicAngle
	}
	if postInfo != nil {
		info.UnderlinePosition = postInfo.UnderlinePosition
		info.UnderlineThickness = postInfo.UnderlineThickness
	} else if cffInfo != nil {
		info.UnderlinePosition = cffInfo.Info.UnderlinePosition
		info.UnderlineThickness = cffInfo.Info.UnderlineThickness
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
	if cffInfo != nil {
		info.Glyphs = cffInfo.Glyphs
	}
	info.CMap = cmapSubtable

	// TODO(voss): glyph widths

	return info, nil
}

var nameTableVersionPat = regexp.MustCompile(`^Version (\d+\.?\d+)`)

func getNameTableVersion(t *name.Table) (head.Version, bool) {
	if t == nil {
		return 0, false
	}
	m := nameTableVersionPat.FindStringSubmatch(t.Version)
	if len(m) != 2 {
		return 0, false
	}
	ver, err := strconv.ParseFloat(m[1], 64)
	if err != nil || ver >= 65536 {
		return 0, false
	}
	return head.Version(ver*65536 + 0.5), true
}

var cffVersionPat = regexp.MustCompile(`^(?:Version )?(\d+\.?\d+)`)

func getCFFVersion(info *cff.Font) (head.Version, bool) {
	if info == nil || info.Info.Version == "" {
		return 0, false
	}
	m := cffVersionPat.FindStringSubmatch(info.Info.Version)
	if len(m) != 2 {
		return 0, false
	}
	ver, err := strconv.ParseFloat(m[1], 64)
	if err != nil || ver >= 65536 {
		return 0, false
	}
	return head.Version(ver*65536 + 0.5), true
}
