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
	"errors"
	"fmt"
	"io"
	"os"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// Font describes a TrueType font file.
type Font struct {
	Fd     *os.File
	Header *table.Header
	Head   *table.Head

	NumGlyphs int // TODO(voss): should this be here?
}

// Open loads a TrueType font into memory.
func Open(fname string) (*Font, error) {
	fd, err := os.Open(fname)
	if err != nil {
		return nil, err
	}

	header, err := table.ReadHeader(fd)
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			err = errors.New("malformed/corrupted font file")
		}
		return nil, err
	}

	head := &table.Head{}
	_, err = header.ReadTableHead(fd, "head", head)
	if err != nil {
		return nil, err
	}
	if head.Version != 0x00010000 {
		return nil, fmt.Errorf(
			"sfnt/head: unsupported version 0x%08X", head.Version)
	}
	if head.MagicNumber != 0x5F0F3CF5 {
		return nil, errors.New("sfnt/head: wrong magic number")
	}

	tt := &Font{
		Fd:     fd,
		Header: header,
		Head:   head,
	}

	maxp, err := tt.getMaxpInfo()
	if err != nil {
		return nil, err
	}
	if maxp.NumGlyphs < 2 {
		// glyph index 0 denotes a missing character and is always included
		return nil, errors.New("no glyphs found")
	}
	tt.NumGlyphs = int(maxp.NumGlyphs)

	return tt, nil
}

// Close frees all resources associated with the font.  The Font object
// cannot be used any more after Close() has been called.
func (tt *Font) Close() error {
	return tt.Fd.Close()
}

// HasTables returns true, if all the given tables are present in the font.
func (tt *Font) HasTables(names ...string) bool {
	for _, name := range names {
		if tt.Header.Find(name) == nil {
			return false
		}
	}
	return true
}

// IsTrueType checks whether all required tables for a TrueType font are
// present.
func (tt *Font) IsTrueType() bool {
	return tt.HasTables("cmap", "glyf", "head", "hhea", "hmtx", "loca", "maxp", "name", "post")
}

// IsOpenType checks whether all required tables for an OpenType font are
// present.
func (tt *Font) IsOpenType() bool {
	if !tt.HasTables("cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post") {
		return false
	}
	if tt.HasTables("glyf", "loca") || tt.HasTables("CFF ") {
		return true
	}
	return false
}

// SelectCmap chooses one of the sub-tables of the cmap table and reads the
// fonts character encoding from there.  If a full unicode mapping is found,
// this is used.  Otherwise, the function tries to use a 16 bit BMP encoding.
// If this fails, a legacy 1,0 record is used as a last resort.
func (tt *Font) SelectCmap() (map[rune]font.GlyphIndex, error) {
	rec := tt.Header.Find("cmap")
	if rec == nil {
		return nil, &table.ErrNoTable{Name: "cmap"}
	}
	cmapFd := io.NewSectionReader(tt.Fd, int64(rec.Offset), int64(rec.Length))
	_ = cmapFd

	cmapTable, err := table.ReadCmapTable(cmapFd)
	if err != nil {
		return nil, err
	}

	unicode := func(idx int) rune {
		return rune(idx)
	}
	macRoman := func(idx int) rune {
		return macintosh[idx]
	}
	candidates := []struct {
		PlatformID uint16
		EncodingID uint16
		IdxToRune  func(int) rune
	}{
		{3, 10, unicode}, // full unicode
		{0, 4, unicode},
		{3, 1, unicode}, // BMP
		{0, 3, unicode},
		{1, 0, macRoman}, // vintage Apple format
	}

	for _, cand := range candidates {
		encRec := cmapTable.Find(cand.PlatformID, cand.EncodingID)
		if encRec == nil {
			continue
		}

		cmap, err := encRec.LoadCmap(cmapFd, cand.IdxToRune)
		if err != nil {
			continue
		}

		return cmap, nil
	}
	return nil, errors.New("sfnt/cmap: no supported character encoding found")
}
