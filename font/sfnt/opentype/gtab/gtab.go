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

package gtab

import (
	"encoding/binary"
	"fmt"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// Info contains the information from a "GSUB" or "GPOS" table.
type Info struct {
	ScriptList ScriptListInfo
}

// Read reads and decodes a "GSUB" or "GPOS" table from r.
// The argument tableName is only used in error messages.
func Read(tableName string, r parser.ReadSeekSizer) (*Info, error) {
	p := parser.New(tableName, r)

	var header struct {
		MajorVersion      uint16
		MinorVersion      uint16
		ScriptListOffset  uint16
		FeatureListOffset uint16
		LookupListOffset  uint16
	}
	var FeatureVariationsOffset uint32

	err := binary.Read(p, binary.BigEndian, &header)
	if err != nil {
		return nil, err
	}
	if header.MajorVersion != 1 || header.MinorVersion > 1 {
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/gtab",
			Feature: fmt.Sprintf("%s table version %d.%d",
				tableName, header.MajorVersion, header.MinorVersion),
		}
	}
	endOfHeader := uint32(10)
	if header.MinorVersion == 1 {
		FeatureVariationsOffset, err = p.ReadUInt32()
		if err != nil {
			return nil, err
		}
		endOfHeader += 4
	}

	fileSize := p.Size()
	for _, offset := range []uint32{
		uint32(header.ScriptListOffset),
		uint32(header.FeatureListOffset),
		uint32(header.LookupListOffset),
		FeatureVariationsOffset,
	} {
		if 0 < offset && offset < endOfHeader || int64(offset) > fileSize {
			return nil, &font.NotSupportedError{
				SubSystem: "sfnt/gtab",
				Feature:   fmt.Sprintf("%s header has invalid offset", tableName),
			}
		}
	}

	info := &Info{}
	if header.ScriptListOffset != 0 {
		info.ScriptList, err = readScriptList(p, int64(header.ScriptListOffset))
	}

	_ = FeatureVariationsOffset
	return info, nil
}
