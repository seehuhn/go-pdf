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
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"

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

type record struct {
	Offset uint32
	Length uint32
}

type alloc struct {
	Start uint32
	End   uint32
}

type header struct {
	scalerType uint32
	toc        map[string]record
}

func readHeader(r io.ReaderAt) (*header, error) {
	var buf [16]byte
	_, err := r.ReadAt(buf[:6], 0)
	if err != nil {
		return nil, err
	}
	scalerType := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	numTables := int(buf[4])<<8 | int(buf[5])

	if scalerType != table.ScalerTypeTrueType &&
		scalerType != table.ScalerTypeCFF &&
		scalerType != table.ScalerTypeApple {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/header",
			Feature:   fmt.Sprintf("scaler type 0x%x", scalerType),
		}
	}
	if numTables > 280 {
		// the largest value observed on my laptop is 28
		return nil, errors.New("sfnt/header: too many tables")
	}

	res := &header{
		scalerType: scalerType,
		toc:        make(map[string]record),
	}
	var coverage []alloc
	for i := 0; i < numTables; i++ {
		_, err := r.ReadAt(buf[:], int64(12+i*16))
		if err != nil {
			return nil, err
		}
		name := string(buf[:4])
		offset := uint32(buf[8])<<24 + uint32(buf[9])<<16 + uint32(buf[10])<<8 + uint32(buf[11])
		length := uint32(buf[12])<<24 + uint32(buf[13])<<16 + uint32(buf[14])<<8 + uint32(buf[15])
		if offset >= 1<<28 || length >= 1<<28 { // 256MB size limit
			return nil, errors.New("sfnt/header: invalid offset or length")
		}
		if length == 0 || !isKnownTable[name] {
			continue
		}
		res.toc[name] = record{
			Offset: offset,
			Length: length,
		}
		coverage = append(coverage, alloc{
			Start: offset,
			End:   offset + length,
		})
	}
	if len(res.toc) == 0 {
		return nil, errors.New("sfnt/header: no tables found")
	}

	// perform some sanity checks
	sort.Slice(coverage, func(i, j int) bool {
		return coverage[i].Start < coverage[j].Start
	})
	if coverage[0].Start < 12 {
		return nil, errors.New("sfnt/header: invalid table offset")
	}
	for i := 1; i < len(coverage); i++ {
		if coverage[i-1].End > coverage[i].Start {
			return nil, errors.New("sfnt/header: overlapping tables")
		}
	}
	_, err = r.ReadAt(buf[:1], int64(coverage[len(coverage)-1].End)-1)
	if err == io.EOF {
		return nil, errors.New("sfnt/header: table extends beyond EOF")
	} else if err != nil {
		return nil, err
	}

	return res, nil
}

// Read reads an OpenType font from a file.
func Read(r io.ReaderAt) (*Info, error) {
	header, err := readHeader(r)
	if err != nil {
		return nil, err
	}

	tableReader := func(name string) (*io.SectionReader, error) {
		rec, ok := header.toc[name]
		if !ok {
			return nil, &table.ErrNoTable{Name: name}
		}
		return io.NewSectionReader(r, int64(rec.Offset), int64(rec.Length)), nil
	}

	tableBytes := func(name string) ([]byte, error) {
		rec, ok := header.toc[name]
		if !ok {
			return nil, &table.ErrNoTable{Name: name}
		}
		res := make([]byte, rec.Length)
		_, err := r.ReadAt(res, int64(rec.Offset))
		if err != nil {
			return nil, err
		}
		return res, nil
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
	hheaData, err := tableBytes("hhea")
	if err != nil && !table.IsMissing(err) {
		return nil, err
	}
	hmtxData, err := tableBytes("hmtx")
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
	nameData, err := tableBytes("name")
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

	cmapData, err := tableBytes("cmap")
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
	if err != nil {
		return nil, err
	}
	cffInfo, err = cff.Read(cffFd)
	if err != nil {
		return nil, err
	}

	if hmtxInfo != nil && len(hmtxInfo.Width) > 0 {
		if len(hmtxInfo.Width) != len(cffInfo.Glyphs) {
			return nil, errors.New("sfnt/header: hmtx and cff glyph count mismatch")
		}
		for i, w := range hmtxInfo.Width {
			cffInfo.Glyphs[i].Width = int16(w)
		}
	}

	info := &Info{}

	if nameTable != nil {
		info.FamilyName = nameTable.Family
	} else {
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
		info.Version = headInfo.FontRevision.Round()
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
	} else {
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
	} else {
		info.ItalicAngle = cffInfo.Info.ItalicAngle
	}
	if postInfo != nil {
		info.UnderlinePosition = postInfo.UnderlinePosition
		info.UnderlineThickness = postInfo.UnderlineThickness
	} else {
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

	info.Glyphs = cffInfo.Glyphs
	info.Private = cffInfo.Private
	info.FdSelect = cffInfo.FdSelect
	info.Encoding = cffInfo.Encoding
	info.Gid2cid = cffInfo.Gid2cid
	info.ROS = cffInfo.ROS

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

func getCFFVersion(info *cff.Font) (head.Version, bool) {
	if info == nil || info.Info.Version == "" {
		return 0, false
	}
	v, err := head.VersionFromString(info.Info.Version)
	if err != nil {
		return 0, false
	}
	return v, true
}

var isKnownTable = map[string]bool{
	"BASE": true,
	"CBDT": true,
	"CBLC": true,
	"CFF ": true,
	"cmap": true,
	"cvt ": true,
	"DSIG": true,
	"feat": true,
	"FFTM": true,
	"fpgm": true,
	"fvar": true,
	"gasp": true,
	"GDEF": true,
	"glyf": true,
	"GPOS": true,
	"GSUB": true,
	"gvar": true,
	"hdmx": true,
	"head": true,
	"hhea": true,
	"hmtx": true,
	"HVAR": true,
	"kern": true,
	"loca": true,
	"LTSH": true,
	"maxp": true,
	"meta": true,
	"morx": true,
	"name": true,
	"OS/2": true,
	"post": true,
	"prep": true,
	"STAT": true,
	"VDMX": true,
	"vhea": true,
	"vmtx": true,
	"VORG": true,
}
