package truetype

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/bits"
	"sort"
	"time"
)

func (tt *Font) export(w io.Writer, include func(string) bool) (int64, error) {
	// Fix the order of tables in the body of the files.
	// https://docs.microsoft.com/en-us/typography/opentype/spec/recom#optimized-table-ordering
	var names []string
	seen := make(map[string]bool)
	for _, name := range []string{
		"head", "hhea", "maxp", "OS/2", "hmtx", "LTSH", "VDMX", "hdmx", "cmap",
		"fpgm", "prep", "cvt ", "loca", "glyf", "kern", "name", "post", "gasp",
	} {
		if _, ok := tt.tables[name]; ok {
			if include != nil && !include(name) {
				continue
			}
			names = append(names, name)
			seen[name] = true
		}
	}
	var extra []string
	for name := range tt.tables {
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
	headTable := &headTable{}
	*headTable = *tt.head
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
	offsets := &offsetsTable{
		ScalerType:    tt.offsets.ScalerType,
		NumTables:     uint16(numTables),
		SearchRange:   1 << (sel + 4),
		EntrySelector: uint16(sel),
		RangeShift:    uint16(16 * (numTables - 1<<sel)),
	}
	records := make([]tableRecord, len(names))
	offset := uint32(12 + 16*numTables)
	var total uint32
	for i, name := range names {
		old := tt.tables[name]
		records[i].Tag = old.Tag
		records[i].Offset = offset
		var checksum uint32
		var length uint32
		if name == "head" {
			checksum = headChecksum
			length = uint32(len(headBytes))
		} else {
			checksum = old.CheckSum // TODO(voss): recalculate?
			length = old.Length
		}
		records[i].CheckSum = checksum
		records[i].Length = length
		total += checksum
		offset += 4 * ((length + 3) / 4)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Tag < records[j].Tag
	})

	// generate and write the file header
	buf.Reset()
	err := binary.Write(buf, binary.BigEndian, offsets)
	if err != nil {
		return 0, err
	}
	err = binary.Write(buf, binary.BigEndian, records)
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
			table := tt.tables[name]
			tableFd := io.NewSectionReader(tt.fd, int64(table.Offset), int64(table.Length))
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
