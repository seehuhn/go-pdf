package sfnt

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/bits"
	"sort"

	"seehuhn.de/go/pdf/font/sfnt/head"
	"seehuhn.de/go/pdf/font/sfnt/table"
)

// WriteTables writes an sfnt file containing the given tables.
// Tables where the data is nil are not written, use a zero-length slice
// to write a table with no data.
// This changes the checksum in the "head" table in place.
func WriteTables(w io.Writer, scalerType uint32, tables map[string][]byte) (int64, error) {
	numTables := len(tables)

	tableNames := make([]string, 0, numTables)
	for name, data := range tables {
		if data != nil {
			tableNames = append(tableNames, name)
		}
	}

	// TODO(voss): sort the table names in the recommended order
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
	offsets := &offsets{
		ScalerType:    scalerType,
		NumTables:     uint16(numTables),
		SearchRange:   1 << (entrySelector + 4),
		EntrySelector: uint16(entrySelector),
		RangeShift:    uint16(16 * (numTables - 1<<entrySelector)),
	}

	// temporarily clear the checksum in the "head" table
	if headData, ok := tables["head"]; ok {
		head.ClearChecksum(headData)
	}

	var totalSum uint32
	offset := uint32(12 + 16*numTables)
	records := make([]record, numTables)
	for i, name := range tableNames {
		body := tables[name]
		length := uint32(len(body))
		checksum := checksum(body)

		records[i].Tag = table.MakeTag(name)
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
	binary.Write(buf, binary.BigEndian, offsets)
	binary.Write(buf, binary.BigEndian, records)
	headerBytes := buf.Bytes()
	totalSum += checksum(headerBytes)

	// set the final checksum in the "head" table
	if headData, ok := tables["head"]; ok {
		head.PatchChecksum(headData, totalSum)
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

// The offsets sub-table forms the first part of Header.
type offsets struct {
	ScalerType    uint32
	NumTables     uint16
	SearchRange   uint16
	EntrySelector uint16
	RangeShift    uint16
}

// A record is part of the file Header.  It contains data about a single sfnt
// table.
type record struct {
	Tag      table.Tag
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
