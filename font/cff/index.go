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

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"seehuhn.de/go/pdf/font/parser"
)

// cffIndex is a CFF INDEX, i.e. an ordered sequence of binary blobs.
type cffIndex [][]byte

func readIndex(p *parser.Parser) (cffIndex, error) {
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

func (data cffIndex) writeTo(w io.Writer) (int, error) {
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

// encode converts a CFF INDEX to its binary representation.
func (data cffIndex) encode() ([]byte, error) {
	buf := &bytes.Buffer{}
	_, err := data.writeTo(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Copy makes a shallow copy of the INDEX.
func (data cffIndex) Copy() cffIndex {
	res := make(cffIndex, len(data))
	copy(res, data)
	return res
}
