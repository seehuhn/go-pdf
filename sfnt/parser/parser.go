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

package parser

import (
	"fmt"
	"io"

	"seehuhn.de/go/pdf/sfnt/glyph"
)

const bufferSize = 1024

// Parser allows to read data from an sfnt file.
type Parser struct {
	r         ReadSeekSizer
	tableName string

	buf       []byte
	from      int64
	pos, used int
	lastRead  int
}

// ReadSeekSizer describes the requirements for a reader that can be used
// as the input to a Parser.
type ReadSeekSizer interface {
	io.ReadSeeker
	Size() int64
}

// New allocates a new Parser.
func New(tableName string, r ReadSeekSizer) *Parser {
	p := &Parser{
		r:         r,
		tableName: tableName,
	}
	err := p.SeekPos(0)
	if err != nil {
		panic(err)
	}
	return p
}

// Size returns the total size of the underlying input file.
func (p *Parser) Size() int64 {
	return p.r.Size()
}

// Pos returns the current reading position within the current region.
func (p *Parser) Pos() int64 {
	return p.from + int64(p.pos)
}

// SeekPos changes the reading position within the current region.
func (p *Parser) SeekPos(filePos int64) error {
	if filePos >= p.from && filePos <= p.from+int64(p.used) {
		p.pos = int(filePos - p.from)
	} else {
		_, err := p.r.Seek(filePos, io.SeekStart)
		if err != nil {
			return err
		}
		p.from = filePos
		p.pos = 0
		p.used = 0
	}

	return nil
}

// Discard skips the next n bytes of input.
func (p *Parser) Discard(n int) error {
	if n < 0 {
		panic("negative discard")
	}
	return p.SeekPos(p.Pos() + int64(n))
}

// Read reads len(buf) bytes of data into buf.  It returns the number of bytes
// read and an error, if any.  The error is non-nil if and only if less than
// len(buf) bytes were read.
func (p *Parser) Read(buf []byte) (int, error) {
	total := 0
	for len(buf) > 0 {
		k := len(buf)
		if k > bufferSize {
			k = bufferSize
		}
		tmp, err := p.ReadBytes(k)
		k = copy(buf, tmp)
		total += k
		buf = buf[k:]
		if len(buf) > 0 && err != nil {
			return total, err
		}
	}
	return total, nil
}

// ReadUint8 reads a single uint8 value from the current position.
func (p *Parser) ReadUint8() (uint8, error) {
	buf, err := p.ReadBytes(1)
	if err != nil {
		return 0, err
	}
	return uint8(buf[0]), nil
}

// ReadUint16 reads a single uint16 value from the current position.
func (p *Parser) ReadUint16() (uint16, error) {
	buf, err := p.ReadBytes(2)
	if err != nil {
		return 0, err
	}
	return uint16(buf[0])<<8 | uint16(buf[1]), nil
}

// ReadInt16 reads a single int16 value from the current position.
func (p *Parser) ReadInt16() (int16, error) {
	val, err := p.ReadUint16()
	return int16(val), err
}

// ReadUint32 reads a single uint32 value from the current position.
func (p *Parser) ReadUint32() (uint32, error) {
	buf, err := p.ReadBytes(4)
	if err != nil {
		return 0, err
	}
	return uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]), nil
}

// ReadUint16Slice reads a length followed by a sequence of uint16 values.
func (p *Parser) ReadUint16Slice() ([]uint16, error) {
	n, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	res := make([]uint16, n)
	for i := range res {
		val, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		res[i] = val
	}
	return res, nil
}

// ReadGIDSlice reads a length followed by a sequence of GlyphID values.
func (p *Parser) ReadGIDSlice() ([]glyph.ID, error) {
	n, err := p.ReadUint16()
	if err != nil {
		return nil, err
	}
	res := make([]glyph.ID, n)
	for i := range res {
		val, err := p.ReadUint16()
		if err != nil {
			return nil, err
		}
		res[i] = glyph.ID(val)
	}
	return res, nil
}

// ReadBytes reads n bytes from the file, starting at the current position.  The
// returned slice points into the internal buffer, slice contents must not be
// modified by the caller and are only valid until the next call to one of the
// parser methods.
//
// The read size n must be <= 1024.
func (p *Parser) ReadBytes(n int) ([]byte, error) {
	p.lastRead = int(p.from + int64(p.pos))
	if n < 0 {
		n = 0
	} else if n > bufferSize {
		panic("buffer size exceeded")
	}

	for p.pos+n > p.used {
		if len(p.buf) == 0 {
			p.buf = make([]byte, bufferSize)
		}
		k := copy(p.buf, p.buf[p.pos:p.used])
		p.from += int64(p.pos)
		p.pos = 0
		p.used = k

		l, err := p.r.Read(p.buf[p.used:])
		if err == io.EOF {
			if l > 0 {
				err = nil
			} else {
				err = io.ErrUnexpectedEOF
			}
		}
		if err != nil {
			return nil, p.Error("read failed: %w", err)
		}
		p.used += l
	}

	res := p.buf[p.pos : p.pos+n]
	p.pos += n
	return res, nil
}

func (p *Parser) Error(format string, a ...interface{}) error {
	tableName := p.tableName
	if tableName == "" {
		tableName = "header"
	}
	a = append([]interface{}{tableName, p.lastRead}, a...)
	return fmt.Errorf("%s%+d: "+format, a...)
}
