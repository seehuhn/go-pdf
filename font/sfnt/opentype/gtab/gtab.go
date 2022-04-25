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
	ScriptList  ScriptListInfo
	FeatureList FeatureListInfo
	LookupList  LookupList
}

// Read reads and decodes a "GSUB" or "GPOS" table from r.
// TableName must be either "GSUB" or "GPOS".
// https://docs.microsoft.com/en-us/typography/opentype/spec/gpos#gpos-header
// https://docs.microsoft.com/en-us/typography/opentype/spec/gsub#gsub-header
func Read(tableName string, r parser.ReadSeekSizer) (*Info, error) {
	var sr subtableReader
	switch tableName {
	case "GPOS":
		sr = readGposSubtable
	case "GSUB":
		sr = readGsubSubtable
	default:
		panic("invalid table name")
	}
	return doRead(tableName, r, sr)
}

func doRead(tableName string, r parser.ReadSeekSizer, sr subtableReader) (*Info, error) {
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
			SubSystem: "sfnt/opentype/gtab",
			Feature: fmt.Sprintf("%s table version %d.%d",
				tableName, header.MajorVersion, header.MinorVersion),
		}
	}
	endOfHeader := uint32(10)
	if header.MinorVersion == 1 {
		FeatureVariationsOffset, err = p.ReadUint32()
		if err != nil {
			return nil, err
		}
		endOfHeader += 4
	}

	if header.ScriptListOffset == 0 || header.LookupListOffset == 0 {
		return &Info{
			ScriptList: make(ScriptListInfo),
		}, nil
	}

	fileSize := p.Size()
	for _, offset := range []uint32{
		uint32(header.ScriptListOffset),
		uint32(header.FeatureListOffset),
		uint32(header.LookupListOffset),
	} {
		if offset < endOfHeader || int64(offset) >= fileSize {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/opentype/gtab",
				Reason: fmt.Sprintf("%s header has invalid offset %d",
					tableName, offset),
			}
		}
	}
	if FeatureVariationsOffset != 0 && FeatureVariationsOffset < endOfHeader ||
		int64(FeatureVariationsOffset) >= fileSize {
		return nil, &font.InvalidFontError{
			SubSystem: "sfnt/opentype/gtab",
			Reason:    fmt.Sprintf("%s header has invalid FeatureVariationsOffset", tableName),
		}
	}

	info := &Info{}
	info.ScriptList, err = readScriptList(p, int64(header.ScriptListOffset))
	if err != nil {
		return nil, err
	}
	info.FeatureList, err = readFeatureList(p, int64(header.FeatureListOffset))
	if err != nil {
		return nil, err
	}
	info.LookupList, err = readLookupList(p, int64(header.LookupListOffset), sr)
	if err != nil {
		return nil, err
	}

	_ = FeatureVariationsOffset // TODO(voss): implement this

	return info, nil
}

// Encode returns the binary representation of a "GSUB" or "GPOS" table.
func (info *Info) Encode() []byte {
	scriptList := info.ScriptList.encode()
	featureList := info.FeatureList.encode()
	lookupList := info.LookupList.encode()

	total := 10
	var scriptListOffset int
	if scriptList != nil {
		scriptListOffset = total
		total += len(scriptList)
	}
	var featureListOffset int
	if featureList != nil {
		featureListOffset = total
		total += len(featureList)
	}
	var lookupListOffset int
	if lookupList != nil {
		lookupListOffset = total
		total += len(lookupList)
	}

	buf := make([]byte, total)
	copy(buf, []byte{
		0, 1, // major version
		0, 0, // minor version
		byte(scriptListOffset >> 8), byte(scriptListOffset),
		byte(featureListOffset >> 8), byte(featureListOffset),
		byte(lookupListOffset >> 8), byte(lookupListOffset),
	})
	copy(buf[scriptListOffset:], scriptList)
	copy(buf[featureListOffset:], featureList)
	copy(buf[lookupListOffset:], lookupList)

	return buf
}
