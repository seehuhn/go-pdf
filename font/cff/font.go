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

package cff

// TODO(voss): post answer to
// https://stackoverflow.com/questions/18351580/is-there-any-library-for-subsetting-opentype-ps-cff-fonts
// once this is working

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf/font/parser"
)

type cffFont struct {
	Name string
}

func readCFF(r io.ReadSeeker, length int64) (*cffFont, error) {
	p := parser.New(r)
	err := p.SetRegion("CFF", 0, length)
	if err != nil {
		return nil, err
	}
	x, err := p.ReadUInt32()
	if err != nil {
		return nil, err
	}
	major := x >> 24
	minor := (x >> 16) & 0xFF
	nameIndexPos := int64((x >> 8) & 0xFF)
	offSize := x & 0xFF
	_ = offSize // TODO (what is this used for?)
	if major != 1 {
		return nil, fmt.Errorf("unsupported CFF version %d.%d", major, minor)
	}

	res := &cffFont{}

	err = p.SeekPos(nameIndexPos)
	if err != nil {
		return nil, err
	}
	names, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(names) != 1 {
		return nil, errors.New("CFF with multiple fonts not supported")
	}
	res.Name = string(names[0])

	topDicts, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(topDicts) != len(names) {
		return nil, errors.New("invalid CFF")
	}

	for i, topDict := range topDicts {
		dd, err := parseDict(topDict)
		if err != nil {
			return nil, err
		}
		fmt.Printf("Top DICT for %q:\n", string(names[i]))
		for _, entry := range dd {
			fmt.Printf("  - 0x%04x %v\n", entry.op, entry.args)
		}
	}

	// strings, err := readIndex(p)
	// if err != nil {
	// 	return nil, err
	// }
	// for i, s := range strings {
	// 	fmt.Printf("string %d = %q\n", i+nStdString, string(s))
	// }

	return res, nil
}

var errCorruptDict = errors.New("invalid CFF DICT")

type cffDictEntry struct {
	op   uint16
	args []interface{}
}
type cffDict []cffDictEntry

func parseDict(buf []byte) (cffDict, error) {
	var res cffDict
	var stack []interface{}

	flush := func(op uint16) {
		res = append(res, cffDictEntry{
			op:   op,
			args: stack,
		})
		stack = nil
	}

	for len(buf) > 0 {
		b0 := buf[0]
		switch {
		case b0 == 12:
			if len(buf) < 2 {
				return nil, errCorruptDict
			}
			flush(uint16(b0)<<8 + uint16(buf[1]))
			buf = buf[2:]
		case b0 <= 21:
			flush(uint16(b0))
			buf = buf[1:]
		case b0 <= 27: // values 22–27, 31, and 255 are reserved
			return nil, errCorruptDict
		case b0 == 28:
			if len(buf) < 3 {
				return nil, errCorruptDict
			}
			stack = append(stack, int32(int16(uint16(buf[1])<<8+uint16(buf[2]))))
			buf = buf[3:]
		case b0 == 29:
			if len(buf) < 5 {
				return nil, errCorruptDict
			}
			stack = append(stack,
				int32(uint32(buf[1])<<24+uint32(buf[2])<<16+uint32(buf[3])<<8+uint32(buf[4])))
			buf = buf[5:]
		case b0 == 30:
			panic("floating point arguments not implemented")
		case b0 == 31: // values 22–27, 31, and 255 are reserved
			return nil, errCorruptDict
		case b0 <= 246:
			stack = append(stack, int32(b0)-139)
			buf = buf[1:]
		case b0 <= 250:
			if len(buf) < 2 {
				return nil, errCorruptDict
			}
			stack = append(stack, int32(b0)*256+int32(buf[1])+(108-247*256))
			buf = buf[2:]
		case b0 <= 254:
			if len(buf) < 2 {
				return nil, errCorruptDict
			}
			stack = append(stack, -int32(b0)*256-int32(buf[1])-(108-251*256))
			buf = buf[2:]
		default: // values 22–27, 31, and 255 are reserved
			return nil, errCorruptDict
		}
	}
	return res, nil
}

func readIndex(p *parser.Parser) ([][]byte, error) {
	count, err := p.ReadUInt16()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	offSize, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	var offsets []uint32
	prevOffset := uint32(1)
	size := p.Size()
	for i := 0; i <= int(count); i++ {
		blob, err := p.ReadBlob(int(offSize))
		if err != nil {
			return nil, err
		}

		var offs uint32
		for _, x := range blob {
			offs = offs<<8 + uint32(x)
		}
		if offs < prevOffset || int64(offs) >= size {
			return nil, p.Error("invalid CFF INDEX")
		}
		offsets = append(offsets, offs-1)
		prevOffset = offs
	}

	buf := make([]byte, offsets[count])
	_, err = p.Read(buf)
	if err != nil {
		return nil, err
	}

	res := make([][]byte, count)
	for i := 0; i < int(count); i++ {
		res[i] = buf[offsets[i]:offsets[i+1]]
	}

	return res, nil
}

func writeIndex(w io.Writer, data [][]byte) (int, error) {
	count := len(data)
	if count >= 1<<16 {
		return 0, errors.New("too many items for CFF INDEX")
	}
	if count == 0 {
		return w.Write([]byte{0, 0})
	}

	bodyLength := 0
	for _, blob := range data {
		bodyLength += len(blob)
	}

	offSize := 1
	for bodyLength+1 >= 1<<(8*offSize) {
		offSize++
	}
	if offSize > 4 {
		return 0, errors.New("too much data for CFF INDEX")
	}

	total := 0
	out := bufio.NewWriter(w)

	n, _ := out.Write([]byte{
		byte(count >> 8), byte(count), // count
		byte(offSize), // offSize
	})
	total += n

	// offset
	var buf [4]byte
	pos := uint32(1)
	for i := 0; i <= count; i++ {
		for j := 0; j < offSize; j++ {
			buf[j] = byte(pos >> (8 * (offSize - j - 1)))
		}
		n, _ = out.Write(buf[:offSize])
		total += n
		if i < count {
			pos += uint32(len(data[i]))
		}
	}

	// data
	for i := 0; i < count; i++ {
		n, _ = out.Write(data[i])
		total += n
	}

	return total, out.Flush()
}