// seehuhn.de/go/pdf - support for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"encoding/binary"
	"errors"
	"io"
	"strconv"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// TODO(voss): add better protections against malicious font files

func (tt *Font) getMaxpInfo() (*table.MaxpHead, error) {
	maxp := &table.MaxpHead{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "maxp", maxp)
	if err != nil {
		return nil, err
	}
	if maxp.Version != 0x00005000 && maxp.Version != 0x00010000 {
		return nil, errors.New("unknown maxp version 0x" +
			strconv.FormatInt(int64(maxp.Version), 16))
	}
	return maxp, nil
}

// GetFontName reads the PostScript name of a font from the "name" table.
func (tt *Font) GetFontName() (string, error) {
	nameHeader := &table.NameHeader{}
	nameFd, err := tt.Header.ReadTableHead(tt.Fd, "name", nameHeader)
	if err != nil {
		return "", err
	}

	record := &table.NameRecord{}
	for i := 0; i < int(nameHeader.Count); i++ {
		err := binary.Read(nameFd, binary.BigEndian, record)
		if err != nil {
			return "", err
		}
		if record.NameID != 6 {
			continue
		}

		switch {
		case record.PlatformID == 1 && record.PlatformSpecificID == 0 &&
			record.LanguageID == 0:
			_, err = nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
			if err != nil {
				return "", err
			}
			buf := make([]byte, record.Length)
			_, err := io.ReadFull(nameFd, buf)
			if err != nil {
				return "", err
			}
			rr := make([]rune, len(buf))
			for i, c := range buf {
				rr[i] = macintosh[c]
			}
			return string(rr), nil
		case record.PlatformID == 3 && record.PlatformSpecificID == 1:
			_, err = nameFd.Seek(int64(nameHeader.Offset)+int64(record.Offset),
				io.SeekStart)
			if err != nil {
				return "", err
			}
			buf := make([]uint16, record.Length/2)
			err := binary.Read(nameFd, binary.BigEndian, buf)
			if err != nil {
				return "", err
			}
			rr := make([]rune, len(buf))
			for i, c := range buf {
				rr[i] = rune(c)
			}
			return string(rr), nil
		}
	}

	return "", errors.New("no usable font name found")
}

// GetPostInfo reads the "post" table of a sfnt file.
func (tt *Font) GetPostInfo() (*table.PostInfo, error) {
	postHeader := &table.PostHeader{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "post", postHeader)
	if err != nil {
		return nil, err
	}

	// TODO(voss): check the format
	// fmt.Printf("format = 0x%08X\n", postHeader.Format)

	// TODO(voss): make this more similar to the other functions in this file.
	res := &table.PostInfo{
		ItalicAngle:        float64(postHeader.ItalicAngle) / 65536,
		UnderlinePosition:  postHeader.UnderlinePosition,
		UnderlineThickness: postHeader.UnderlineThickness,
		IsFixedPitch:       postHeader.IsFixedPitch != 0,
	}
	return res, nil
}

// GetHHeaInfo reads the "hhea" table of a sfnt file.
// TODO(voss): use caching?
func (tt *Font) GetHHeaInfo() (*table.Hhea, error) {
	hhea := &table.Hhea{}
	_, err := tt.Header.ReadTableHead(tt.Fd, "hhea", hhea)
	if err != nil {
		return nil, err
	}
	return hhea, nil
}

// GetHMtxInfo reads the "hmtx" table of a sfnt file.
func (tt *Font) GetHMtxInfo(NumOfLongHorMetrics uint16) (*table.Hmtx, error) {
	hmtx := &table.Hmtx{
		HMetrics:        make([]table.LongHorMetric, NumOfLongHorMetrics),
		LeftSideBearing: make([]int16, tt.NumGlyphs-int(NumOfLongHorMetrics)),
	}
	fd, err := tt.Header.ReadTableHead(tt.Fd, "hmtx", hmtx.HMetrics)
	if err != nil {
		return nil, err
	}
	err = binary.Read(fd, binary.BigEndian, hmtx.LeftSideBearing)
	if err != nil {
		return nil, err
	}
	return hmtx, nil
}

// GetOS2Info reads the "OS/2" table of a sfnt file.
func (tt *Font) GetOS2Info() (*table.OS2, error) {
	os2 := &table.OS2{}
	os2Fd, err := tt.Header.ReadTableHead(tt.Fd, "OS/2", &os2.V0)
	if err != nil {
		return nil, err
	}

	if os2.V0.Version > 0 || tt.Header.Find("OS/2").Length > 68 {
		os2.V0MSValid = true
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V0MS)
		if err != nil {
			return nil, err
		}
	}
	if os2.V0.Version >= 1 {
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V1)
		if err != nil {
			return nil, err
		}
	}
	if os2.V0.Version >= 4 {
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V4)
		if err != nil {
			return nil, err
		}
	}
	if os2.V0.Version >= 5 {
		err := binary.Read(os2Fd, binary.BigEndian, &os2.V5)
		if err != nil {
			return nil, err
		}
	}
	return os2, nil
}

// ReadKernInfo reads kerning information from the "kern" table.
//
// TODO(voss): use a gpos2_1 structure instead.
func (tt *Font) ReadKernInfo() (map[font.GlyphPair]int, error) {
	var Header struct {
		Version   uint16
		NumTables uint16
	}
	kernFd, err := tt.Header.ReadTableHead(tt.Fd, "kern", &Header)
	if err != nil {
		return nil, err
	} else if Header.Version != 0 {
		return nil, errors.New("unsupported kern table version")
	}

	kerning := make(map[font.GlyphPair]int)
	for i := 0; i < int(Header.NumTables); i++ {
		var subHeader struct {
			Version  uint16
			Length   uint16
			Coverage uint16
		}
		err = binary.Read(kernFd, binary.BigEndian, &subHeader)
		if err != nil {
			return nil, err
		}
		if subHeader.Version != 0 ||
			subHeader.Coverage != 1 ||
			subHeader.Length < 6+8 ||
			subHeader.Length%2 != 0 {
			// skip unsupported and mal-formed subtables
			_, err = io.CopyN(io.Discard, kernFd, int64(subHeader.Length-6))
			if err != nil {
				return nil, err
			}
			continue
		}

		buf := make([]uint16, (subHeader.Length-6)/2)
		err = binary.Read(kernFd, binary.BigEndian, buf)
		if err != nil {
			return nil, err
		}

		nPairs := int(buf[0])
		buf = buf[4:] // skip the header
		for nPairs > 0 && len(buf) >= 3 {
			LR := font.GlyphPair{
				font.GlyphID(buf[0]), font.GlyphID(buf[1])}
			kern := int16(buf[2])
			kerning[LR] = int(kern)
			buf = buf[3:]
		}
	}

	return kerning, nil
}

// GetGlyfOffsets returns the locations of the glyphs in the "glyf" table.
func (tt *Font) GetGlyfOffsets() ([]uint32, error) {
	var err error
	offsets := make([]uint32, tt.NumGlyphs+1)
	if tt.Head.IndexToLocFormat == 0 {
		shortOffsets := make([]uint16, tt.NumGlyphs+1)
		_, err = tt.Header.ReadTableHead(tt.Fd, "loca", shortOffsets)
		for i, x := range shortOffsets {
			offsets[i] = uint32(x) * 2
		}
	} else {
		_, err = tt.Header.ReadTableHead(tt.Fd, "loca", offsets)
	}
	if err != nil {
		return nil, err
	}
	return offsets, nil
}

// GetGlyfInfo reads the glyph bounding boxes from the "glyf" table.
func (tt *Font) GetGlyfInfo() (*table.Glyf, error) {
	offset, err := tt.GetGlyfOffsets()
	if err != nil {
		return nil, err
	}

	res := &table.Glyf{
		Data: make([]table.GlyphHeader, tt.NumGlyphs),
	}
	glyfFd, err := tt.Header.ReadTableHead(tt.Fd, "glyf", nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < tt.NumGlyphs; i++ {
		offs := offset[i]
		if offs == offset[i+1] {
			continue
		}
		_, err := glyfFd.Seek(int64(offs), io.SeekStart)
		if err != nil {
			return nil, err
		}
		err = binary.Read(glyfFd, binary.BigEndian, &res.Data[i])
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}
