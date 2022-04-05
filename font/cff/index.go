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
	"bytes"
	"errors"

	"seehuhn.de/go/pdf/font/parser"
)

// cffIndex is a CFF INDEX, i.e. an ordered sequence of binary blobs.
type cffIndex [][]byte

func readIndexAt(p *parser.Parser, pos int32, name string) (cffIndex, error) {
	if pos < 4 {
		return nil, errors.New("cff: missing " + name + " INDEX")
	}
	err := p.SeekPos(int64(pos))
	if err != nil {
		return nil, err
	}

	return readIndex(p)
}

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
		blob, err := p.ReadBytes(int(offSize))
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

// encode converts a CFF INDEX to its binary representation.
func (data cffIndex) encode() []byte {
	count := len(data)
	if count >= 1<<16 {
		panic("cff: too many items for INDEX")
	}
	if count == 0 {
		return []byte{0, 0}
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
		panic("cff: too much data for INDEX")
	}

	out := &bytes.Buffer{}
	out.Write([]byte{
		byte(count >> 8), byte(count), // count
		byte(offSize), // offSize
	})

	// offset
	var offsetBuf [4]byte
	pos := uint32(1)
	for i := 0; i <= count; i++ {
		for j := 0; j < offSize; j++ {
			offsetBuf[j] = byte(pos >> (8 * (offSize - j - 1)))
		}
		out.Write(offsetBuf[:offSize])
		if i < count {
			pos += uint32(len(data[i]))
		}
	}

	// data
	for i := 0; i < count; i++ {
		out.Write(data[i])
	}

	return out.Bytes()
}
