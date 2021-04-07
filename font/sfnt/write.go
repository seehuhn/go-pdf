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
	"bytes"
	"encoding/binary"
	"io"
	"math/bits"
	"sort"
	"time"

	"seehuhn.de/go/pdf/font/sfnt/table"
)

// Export writes the font to the io.Writer w.  The include argument can be used
// to select a subset of tables.
func (tt *Font) Export(w io.Writer, include func(string) bool) (int64, error) {
	// Fix the order of tables in the body of the files.
	// https://docs.microsoft.com/en-us/typography/opentype/spec/recom#optimized-table-ordering
	var names []string
	seen := make(map[string]bool)
	for _, name := range []string{
		"head", "hhea", "maxp", "OS/2", "hmtx", "LTSH", "VDMX", "hdmx", "cmap",
		"fpgm", "prep", "cvt ", "loca", "glyf", "kern", "name", "post", "gasp",
	} {
		if tt.Header.Find(name) != nil {
			if include != nil && !include(name) {
				continue
			}
			names = append(names, name)
			seen[name] = true
		}
	}
	var extra []string
	for i := 0; i < int(tt.Header.Offsets.NumTables); i++ {
		name := tt.Header.Records[i].Tag.String()
		if name == "DSIG" {
			// Pre-existing digital signatures will no longer be valid after we
			// re-arranged the tables.
			continue
		}
		if include != nil && !include(name) {
			continue
		}
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	names = append(names, extra...)

	// generate the new "head" table
	headTable := &table.Head{}
	*headTable = *tt.Head
	headTable.CheckSumAdjustment = 0
	ttZeroTime := time.Date(1904, time.January, 1, 0, 0, 0, 0, time.UTC)
	headTable.Modified = int64(time.Since(ttZeroTime).Seconds())
	headTable.FontDirectionHint = 2
	buf := &bytes.Buffer{}
	_ = binary.Write(buf, binary.BigEndian, headTable)
	headBytes := append([]byte{}, buf.Bytes()...) // keep a copy
	headChecksum, _ := checksum(buf, true)

	// generate the new header
	numTables := len(names)
	sel := bits.Len(uint(numTables)) - 1
	header := &table.Header{
		Offsets: table.Offsets{
			ScalerType:    tt.Header.Offsets.ScalerType,
			NumTables:     uint16(numTables),
			SearchRange:   1 << (sel + 4),
			EntrySelector: uint16(sel),
			RangeShift:    uint16(16 * (numTables - 1<<sel)),
		},
		Records: make([]table.Record, len(names)),
	}
	offset := uint32(12 + 16*numTables)
	var total uint32
	for i, name := range names {
		old := tt.Header.Find(name)
		header.Records[i].Tag = old.Tag
		header.Records[i].Offset = offset
		var checksum uint32
		var length uint32
		if name == "head" {
			checksum = headChecksum
			length = uint32(len(headBytes))
		} else {
			checksum = old.CheckSum // TODO(voss): recalculate?
			length = old.Length
		}
		header.Records[i].CheckSum = checksum
		header.Records[i].Length = length
		total += checksum
		offset += 4 * ((length + 3) / 4)
	}
	sort.Slice(header.Records, func(i, j int) bool {
		return bytes.Compare(header.Records[i].Tag[:], header.Records[j].Tag[:]) < 0
	})

	// generate and write the file header
	buf.Reset()
	err := binary.Write(buf, binary.BigEndian, header.Offsets)
	if err != nil {
		return 0, err
	}
	err = binary.Write(buf, binary.BigEndian, header.Records)
	if err != nil {
		return 0, err
	}
	totalSize := int64(buf.Len())
	n, err := w.Write(buf.Bytes())
	if err != nil {
		return 0, err
	}
	if n%4 != 0 {
		panic("wrong header length")
	}

	// fix the checksum in the "head" table
	headerChecksum, _ := checksum(buf, false)
	total += headerChecksum
	binary.BigEndian.PutUint32(headBytes[8:12], 0xB1B0AFBA-total)

	// write the tables
	var pad [3]byte
	for _, name := range names {
		var n int
		if name == "head" {
			n, err = w.Write(headBytes)
		} else {
			table := tt.Header.Find(name)
			tableFd := io.NewSectionReader(tt.Fd, int64(table.Offset), int64(table.Length))
			n64, e2 := io.Copy(w, tableFd)
			n = int(n64)
			err = e2
		}
		totalSize += int64(n)
		if err != nil {
			return 0, err
		}
		if k := n % 4; k != 0 {
			_, err := w.Write(pad[:4-k])
			if err != nil {
				return 0, err
			}
			totalSize += int64(4 - k)
		}
	}

	return totalSize, nil
}
