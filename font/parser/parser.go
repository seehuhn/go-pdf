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
)

const bufferSize = 1024

// State stores the current register values of the interpreter.
type State struct {
	A     int64
	R     [2]int64 // TODO(voss): use names A, B, C instead?
	Tag   string
	Stash []uint16
}

// GetStash return the slice of stashed values and clears the stash.
func (s *State) GetStash() []uint16 {
	res := s.Stash
	s.Stash = nil
	return res
}

// Parser allows to read data from an sfnt file.
type Parser struct {
	r io.ReadSeeker

	start, end int64
	tableName  string

	buf       []byte
	from      int64
	pos, used int
	lastRead  int
}

// New allocates a new Parser.  SetRegion() must be called before the
// parser can be used.
func New(r io.ReadSeeker) *Parser {
	return &Parser{r: r}
}

// SetRegion sets up the parser to read from the given region in the file.
// The current reading position is moved to the start of the region.
// The tableName is only used in error messages.
func (p *Parser) SetRegion(tableName string, start, length int64) error {
	p.tableName = tableName
	p.start = start
	p.end = start + length

	return p.SeekPos(0)
}

// Size returns the total length of the current region.
func (p *Parser) Size() int64 {
	return p.end - p.start
}

// Pos returns the current reading position within the current region.
func (p *Parser) Pos() int64 {
	return p.from + int64(p.pos) - p.start
}

// SeekPos changes the reading position within the current region.
func (p *Parser) SeekPos(posInRegion int64) error {
	filePos := p.start + posInRegion
	if filePos < p.start || filePos > p.end {
		return p.Error("seek target %d+%d is outside [%d,%d+%d]",
			p.start, posInRegion, p.start, p.start, p.end-p.start)
	}

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

// Exec runs the given commands, updating the state s.
func (p *Parser) Exec(s *State, cmds ...Command) error {
	var PC int
	var loopStartPC int
	var loopCount int

CommandLoop:
	for PC < len(cmds) {
		cmd := cmds[PC]
		PC++

		var arg Command
		if cmd < 128 {
			arg = cmds[PC]
			PC++
		}

		switch cmd {
		case CmdRead8:
			buf, err := p.ReadBlob(1)
			if err != nil {
				return err
			}
			switch arg {
			case TypeUInt:
				s.A = int64(buf[0])
			case TypeInt:
				s.A = int64(int8(buf[0]))
			default:
				panic("unknown type for CmdRead16")
			}
		case CmdRead16:
			buf, err := p.ReadBlob(2)
			if err != nil {
				return err
			}
			val := uint16(buf[0])<<8 + uint16(buf[1])
			switch arg {
			case TypeUInt:
				s.A = int64(val)
			case TypeInt:
				s.A = int64(int16(val))
			default:
				panic("unknown type for CmdRead16")
			}
		case CmdRead32:
			buf, err := p.ReadBlob(4)
			if err != nil {
				return err
			}
			val := uint32(buf[0])<<24 + uint32(buf[1])<<16 + uint32(buf[2])<<8 + uint32(buf[3])
			switch arg {
			case TypeUInt:
				s.A = int64(val)
			case TypeInt:
				s.A = int64(int32(val))
			case TypeTag:
				s.Tag = string(buf)
			default:
				panic("unknown type for CmdRead32")
			}
		case CmdSeek:
			err := p.SeekPos(s.A)
			if err != nil {
				return err
			}

		case CmdStoreInto:
			s.R[arg] = s.A
		case CmdLoadI:
			s.A = int64(int8(arg))
		case CmdLoadFrom:
			s.A = s.R[arg]
		case CmdStash:
			buf, err := p.ReadBlob(2)
			if err != nil {
				return err
			}
			s.Stash = append(s.Stash, uint16(buf[0])<<8+uint16(buf[1]))

		case CmdDec:
			s.A--
		case CmdIAdd:
			s.A += int64(int8(arg))
		case CmdAdd:
			s.A += s.R[arg]
		case CmdIMult:
			s.A *= int64(int8(arg))
		case CmdMult:
			s.A *= s.R[arg]

		case CmdExitIfLt:
			target := int64(int8(arg))
			if s.A < target {
				break CommandLoop
			}

		case CmdAssertEq:
			target := int64(arg)
			if s.A != target {
				return p.Error("expected 0x%04x, but got 0x%04x (%d)",
					target, s.A, PC)
			}
		case CmdAssertGt:
			target := int64(arg)
			if s.A <= target {
				return p.Error("expected >0x%04x, but got 0x%04x (%d)",
					target, s.A, PC)
			}

		case CmdLoop:
			loopStartPC = PC
			loopCount = int(s.A)
			if loopCount <= 0 {
				for cmds[PC] != CmdEndLoop {
					if cmds[PC] < 128 {
						PC += 2
					} else {
						PC++
					}
				}
			}
		case CmdEndLoop:
			loopCount--
			if loopCount > 0 {
				PC = loopStartPC
			}

		default:
			panic(fmt.Sprintf("unknown command 0x%02x", cmd))
		}
	}
	return nil
}

func (p *Parser) Read(buf []byte) (int, error) {
	total := 0
	for {
		k := len(buf)
		if k == 0 {
			break
		}

		if k > bufferSize {
			k = bufferSize
		}
		tmp, err := p.ReadBlob(k)
		k = copy(buf, tmp)
		total += k
		buf = buf[k:]
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// ReadUInt8 reads a single uint8 value from the current position.
func (p *Parser) ReadUInt8() (uint8, error) {
	buf, err := p.ReadBlob(1)
	if err != nil {
		return 0, err
	}
	return uint8(buf[0]), nil
}

// ReadUInt16 reads a single uint16 value from the current position.
func (p *Parser) ReadUInt16() (uint16, error) {
	buf, err := p.ReadBlob(2)
	if err != nil {
		return 0, err
	}
	return uint16(buf[0])<<8 + uint16(buf[1]), nil
}

// ReadInt16 reads a single int16 value from the current position.
func (p *Parser) ReadInt16() (int16, error) {
	val, err := p.ReadUInt16()
	return int16(val), err
}

// ReadUInt32 reads a single uint32 value from the current position.
func (p *Parser) ReadUInt32() (uint32, error) {
	buf, err := p.ReadBlob(4)
	if err != nil {
		return 0, err
	}
	return uint32(buf[0])<<24 + uint32(buf[1])<<16 + uint32(buf[2])<<8 + uint32(buf[3]), nil
}

// ReadBlob reads n bytes from the file, starting at the current position.  The
// returned slice points into the internal buffer, slice contents must not be
// modified by the caller and are only valid until the next call to one of the
// parser methods.
//
// The read size n must be <= 1024.
func (p *Parser) ReadBlob(n int) ([]byte, error) {
	p.lastRead = int(p.from + int64(p.pos) - p.start)
	if n < 0 {
		n = 0
	} else if n > bufferSize {
		panic("buffer size exceeded")
	}
	if p.from+int64(p.pos+n) > p.end {
		return nil, io.ErrUnexpectedEOF
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

// Command represents a command (or an argument for a command) for the
// interpreter.
type Command uint8

// Commands which take one argument
const (
	CmdRead8     = iota // arg: type
	CmdRead16           // arg: type
	CmdRead32           // arg: type
	CmdStoreInto        // arg: register
	CmdLoadI            // arg: new int8 value for A
	CmdLoadFrom         // arg: register
	CmdIAdd             // arg: int8 literal value
	CmdAdd              // arg: register
	CmdIMult            // arg: int8 literal value
	CmdMult             // arg: register
	CmdExitIfLt         // arg: comparison value
	CmdAssertEq         // arg: comparison value
	CmdAssertGt         // arg: comparison value
)

// Commands which take no arguments
const (
	CmdSeek = iota + 128
	CmdDec
	CmdLoop
	CmdEndLoop
	CmdStash
)

// Types for the CmdRead* commands
const (
	TypeUInt Command = iota + 224
	TypeInt
	TypeTag
)
