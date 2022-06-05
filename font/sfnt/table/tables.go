// seehuhn.de/go/pdf - a library for reading and writing PDF files
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

package table

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"seehuhn.de/go/pdf/font"
)

const (
	ScalerTypeTrueType = 0x00010000
	ScalerTypeCFF      = 0x4F54544F
	ScalerTypeApple    = 0x74727565
)

type Header struct {
	ScalerType uint32
	Toc        map[string]Record
}

type Record struct {
	Offset uint32
	Length uint32
}

// ReadHeader reads the file header of an sfnt font file.
func ReadHeader(r io.ReaderAt) (*Header, error) {
	var buf [16]byte
	_, err := r.ReadAt(buf[:6], 0)
	if err != nil {
		return nil, err
	}
	scalerType := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
	numTables := int(buf[4])<<8 | int(buf[5])

	if scalerType != ScalerTypeTrueType &&
		scalerType != ScalerTypeCFF &&
		scalerType != ScalerTypeApple {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/header",
			Feature:   fmt.Sprintf("scaler type 0x%x", scalerType),
		}
	}
	if numTables > 280 {
		// the largest value observed on my laptop is 28
		return nil, errors.New("sfnt/header: too many tables")
	}

	h := &Header{
		ScalerType: scalerType,
		Toc:        make(map[string]Record),
	}
	type alloc struct {
		Start uint32
		End   uint32
	}
	var coverage []alloc
	for i := 0; i < numTables; i++ {
		_, err := r.ReadAt(buf[:], int64(12+i*16))
		if err != nil {
			return nil, err
		}
		name := string(buf[:4])
		offset := uint32(buf[8])<<24 | uint32(buf[9])<<16 | uint32(buf[10])<<8 | uint32(buf[11])
		length := uint32(buf[12])<<24 | uint32(buf[13])<<16 | uint32(buf[14])<<8 | uint32(buf[15])
		if !isKnownTable[name] {
			continue
		}
		h.Toc[name] = Record{
			Offset: offset,
			Length: length,
		}
		coverage = append(coverage, alloc{
			Start: offset,
			End:   offset + length,
		})
	}
	if len(h.Toc) == 0 {
		return nil, errors.New("sfnt/header: no tables found")
	}

	// perform some sanity checks
	sort.Slice(coverage, func(i, j int) bool {
		if coverage[i].Start != coverage[j].Start {
			return coverage[i].Start < coverage[j].Start
		}
		return coverage[i].End < coverage[j].End
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

	return h, nil
}

func (h *Header) Has(names ...string) bool {
	for _, name := range names {
		if _, ok := h.Toc[name]; !ok {
			return false
		}
	}
	return true
}

func (h *Header) Find(tableName string) (Record, error) {
	rec, ok := h.Toc[tableName]
	if !ok {
		return rec, &ErrNoTable{Name: tableName}
	}
	return rec, nil
}

func (h *Header) ReadTableBytes(r io.ReaderAt, tableName string) ([]byte, error) {
	rec, err := h.Find(tableName)
	if err != nil {
		return nil, err
	}
	res := make([]byte, rec.Length)
	n, err := r.ReadAt(res, int64(rec.Offset))
	if n < len(res) && err != nil {
		return nil, err
	}
	return res[:n], nil
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

// Tag represents a tag string composed of 4 ASCII bytes
type Tag [4]byte

// MakeTag converts a string of length 4 bytes to a Tag.
func MakeTag(s string) Tag {
	if len(s) != 4 {
		panic("tag must be 4 bytes")
	}
	return Tag{s[0], s[1], s[2], s[3]}
}

func (tag Tag) String() string {
	return string(tag[:])
}
