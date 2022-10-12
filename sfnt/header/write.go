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

package header

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/bits"
	"sort"
)

// Write writes an sfnt file containing the given tables.
// Tables where the data is nil are not written, use a zero-length slice
// to write a table with no data.
// This changes the checksum in the "head" table in place.
func Write(w io.Writer, scalerType uint32, tables map[string][]byte) (int64, error) {
	numTables := len(tables)

	tableNames := make([]string, 0, numTables)
	for name, data := range tables {
		if data != nil && len(name) == 4 && isASCII(name) {
			tableNames = append(tableNames, name)
		}
	}

	// sort the table names in the recommended order
	sort.Slice(tableNames, func(i, j int) bool {
		iPrio := ttTableOrder[tableNames[i]]
		jPrio := ttTableOrder[tableNames[j]]
		if iPrio != jPrio {
			return iPrio > jPrio
		}
		return tableNames[i] < tableNames[j]
	})

	// prepare the header
	entrySelector := bits.Len(uint(numTables)) - 1
	header := &offsets{
		ScalerType:    scalerType,
		NumTables:     uint16(numTables),
		SearchRange:   1 << (entrySelector + 4),
		EntrySelector: uint16(entrySelector),
		RangeShift:    uint16(16 * (numTables - 1<<entrySelector)),
	}

	// temporarily clear the checksum in the "head" table
	if headData, ok := tables["head"]; ok {
		clearChecksum(headData)
	}

	var totalSum uint32
	offset := uint32(12 + 16*numTables)
	records := make([]rawRecord, numTables)
	for i, name := range tableNames {
		body := tables[name]
		length := uint32(len(body))
		checksum := checksum(body)

		records[i].Tag = tag{name[0], name[1], name[2], name[3]}
		records[i].CheckSum = checksum
		records[i].Offset = offset
		records[i].Length = length

		totalSum += checksum
		offset += 4 * ((length + 3) / 4)
	}
	sort.Slice(records, func(i, j int) bool {
		return bytes.Compare(records[i].Tag[:], records[j].Tag[:]) < 0
	})

	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, header)
	_ = binary.Write(buf, binary.BigEndian, records)
	headerBytes := buf.Bytes()
	totalSum += checksum(headerBytes)

	// set the final checksum in the "head" table
	if headData, ok := tables["head"]; ok {
		patchChecksum(headData, totalSum)
	}

	// write the tables
	var totalSize int64
	n, err := w.Write(headerBytes)
	totalSize += int64(n)
	if err != nil {
		return totalSize, err
	}
	var pad [3]byte
	for _, name := range tableNames {
		body := tables[name]
		n, err := w.Write(body)
		totalSize += int64(n)
		if err != nil {
			return totalSize, err
		}
		if k := n % 4; k != 0 {
			l, err := w.Write(pad[:4-k])
			totalSize += int64(l)
			if err != nil {
				return totalSize, err
			}
		}
	}
	return totalSize, nil
}

// clearChecksum zeros the checksum field of the head table.
func clearChecksum(head []byte) {
	binary.BigEndian.PutUint32(head[8:12], 0)
}

// patchChecksum updates the checksum of the head table.
// The argument is the checksum of the entire font before patching.
func patchChecksum(head []byte, checksum uint32) {
	binary.BigEndian.PutUint32(head[8:12], 0xB1B0AFBA-checksum)
}

// The offsets sub-table forms the first part of Header.
type offsets struct {
	ScalerType    uint32
	NumTables     uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

// A rawRecord is part of the file Header.  It contains data about a single
// sfnt table.
type rawRecord struct {
	Tag      tag
	CheckSum uint32
	Offset   uint32
	Length   uint32
}

// https://docs.microsoft.com/en-us/typography/opentype/spec/recom#optimized-table-ordering
var ttTableOrder = map[string]int{
	"head": 95,
	"hhea": 90,
	"maxp": 85,
	"OS/2": 80,
	"hmtx": 75,
	"LTSH": 70,
	"VDMX": 65,
	"hdmx": 60,
	"cmap": 55,
	"fpgm": 50,
	"prep": 45,
	"cvt ": 40,
	"loca": 35,
	"glyf": 30,
	"kern": 25,
	"name": 20,
	"post": 15,
	"gasp": 10,
	"DSIG": 5,
}
