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

package parser

import (
	"fmt"
	"io"
	"math"

	"seehuhn.de/go/pdf/font/sfnt"
)

const bufferSize = 1024

// State stores the current register values of the interpreter.
type State struct {
	A int64
	R [8]int64

	Stash16 []uint16
}

// GetStash return the slice of stashed values and clears the stash.
func (s *State) GetStash() []uint16 {
	res := s.Stash16
	s.Stash16 = nil
	return res
}

// Parser allows to read data from a sfnt table.
type Parser struct {
	tt         *sfnt.Font
	start, end int64
	tableName  string

	buf       []byte
	from      int64
	pos, used int
	lastRead  int

	scale float64
	Funcs []func(*State)
}

// New allocates a new Parser.  SetTable() must be called before the
// parser can be used.
func New(tt *sfnt.Font) *Parser {
	return &Parser{
		tt:    tt,
		scale: 1000 / float64(tt.Head.UnitsPerEm),
	}
}

// SetTable sets up the parser to read from the given table.
// The current reading position is moved to the start of the table.
func (p *Parser) SetTable(tableName string) error {
	info := p.tt.Header.Find(tableName)
	if info == nil {
		return fmt.Errorf("table %q not found", tableName)
	}

	p.tableName = tableName
	p.start = int64(info.Offset)
	p.end = int64(info.Offset) + int64(info.Length)

	return p.seek(0)
}

// Exec runs the given commands, updating the state s.
func (p *Parser) Exec(s *State, cmds ...Command) error {
	var PC int
	var loopStartPC int
	var loopCount int

	for PC < len(cmds) {
		cmd := cmds[PC]
		PC++

		var arg Command
		if cmd < 128 {
			arg = cmds[PC]
			PC++
		}

		switch cmd {
		case CmdRead16:
			buf, err := p.read(2)
			if err != nil {
				return err
			}
			val := uint16(buf[0])<<8 + uint16(buf[1])
			switch arg {
			case TypeUInt:
				s.A = int64(val)
			case TypeInt:
				s.A = int64(int16(val))
			case TypeUFword:
				s.A = int64(math.Round(float64(val) * p.scale))
			case TypeFword:
				s.A = int64(math.Round(float64(int16(val)) * p.scale))
			default:
				panic("unknown type for CmdRead16")
			}
		case CmdRead32:
			buf, err := p.read(4)
			if err != nil {
				return err
			}
			val := uint32(buf[0])<<24 + uint32(buf[1])<<16 + uint32(buf[2])<<8 + uint32(buf[3])
			switch arg {
			case TypeUInt:
				s.A = int64(val)
			case TypeInt:
				s.A = int64(int32(val))
			case TypeUFword:
				s.A = int64(math.Round(float64(val) * p.scale))
			case TypeFword:
				s.A = int64(math.Round(float64(int32(val)) * p.scale))
			default:
				panic("unknown type for CmdRead16")
			}
		case CmdSeek:
			err := p.seek(s.A)
			if err != nil {
				return err
			}

		case CmdStore:
			s.R[arg] = s.A
		case CmdLoad:
			s.A = s.R[arg]
		case CmdStash:
			s.Stash16 = append(s.Stash16, uint16(s.A))

		case CmdDec:
			s.A--
		case CmdIAdd:
			s.A += int64(int8(arg))
		case CmdAdd:
			s.A += s.R[arg]
		case CmdIMul:
			s.A *= int64(int8(arg))

		case CmdCmpEq:
			target := int64(int8(arg))
			s.A = encodeBool[s.A == target]
		case CmdCmpGe:
			target := int64(int8(arg))
			s.A = encodeBool[s.A >= target]
		case CmdCmpLt:
			target := int64(int8(arg))
			s.A = encodeBool[s.A < target]

		case CmdAssertEq:
			target := int64(arg)
			if s.A != target {
				return fmt.Errorf("%s%+d: expected 0x%04x, but got 0x%04x (%d)",
					p.tableName, p.lastRead, target, s.A, PC)
			}
		case CmdAssertGe:
			target := int64(arg)
			if s.A < target {
				return fmt.Errorf("%s%+d: expected >=0x%04x, but got 0x%04x (%d)",
					p.tableName, p.lastRead, target, s.A, PC)
			}
		case CmdAssertGt:
			target := int64(arg)
			if s.A <= target {
				return fmt.Errorf("%s%+d: expected >0x%04x, but got 0x%04x (%d)",
					p.tableName, p.lastRead, target, s.A, PC)
			}
		case CmdAssertLe:
			target := int64(arg)
			if s.A > target {
				return fmt.Errorf("%s%+d: expected <=0x%04x, but got 0x%04x (%d)",
					p.tableName, p.lastRead, target, s.A, PC)
			}

		case CmdJNZ:
			if s.A != 0 {
				PC += int(int8(arg))
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

		case CmdCall:
			p.Funcs[arg](s)

		default:
			panic(fmt.Sprintf("unknown command 0x%02x", cmd))
		}
	}
	return nil
}

func (p *Parser) seek(tablePos int64) error {
	filePos := p.start + tablePos
	if filePos < p.start || filePos > p.end {
		return fmt.Errorf("%s: seek target %d is outside [%d,%d]",
			p.tableName, filePos, p.start, p.end)
	}

	if filePos >= p.from && filePos <= p.from+int64(p.used) {
		p.pos = int(filePos - p.from)
	} else {
		_, err := p.tt.Fd.Seek(filePos, io.SeekStart)
		if err != nil {
			return err
		}
		p.from = filePos
		p.pos = 0
		p.used = 0
	}

	return nil
}

func (p *Parser) read(n int) ([]byte, error) {
	p.lastRead = int(p.from + int64(p.pos) - p.start)

	for p.pos+n > p.used {
		if len(p.buf) == 0 {
			p.buf = make([]byte, bufferSize)
		}
		k := copy(p.buf, p.buf[p.pos:p.used])
		p.from += int64(p.pos)
		p.pos = 0
		p.used = k

		l, err := p.tt.Fd.Read(p.buf[p.used:])
		if err == io.EOF {
			if l > 0 {
				err = nil
			} else {
				err = io.ErrUnexpectedEOF
			}
		}
		if err != nil {
			return nil, err
		}
		p.used += l
	}

	if p.from+int64(p.pos+n) > p.end {
		return nil, io.ErrUnexpectedEOF
	}
	res := p.buf[p.pos : p.pos+n]
	p.pos += n
	return res, nil
}

// Command represents a command (or an argument for a command) for the
// interpreter.
type Command uint8

// Commands which take one argument
const (
	CmdRead16   = iota // arg: type
	CmdRead32          // arg: type
	CmdStore           // arg: register
	CmdLoad            // arg: register
	CmdIAdd            // arg: int8 literal value
	CmdAdd             // arg: register
	CmdIMul            // arg: int8 literal value
	CmdCmpEq           // arg: comparison value
	CmdCmpGe           // arg: comparison value
	CmdCmpLt           // arg: comparison value
	CmdAssertEq        // arg: comparison value
	CmdAssertGe        // arg: comparison value
	CmdAssertGt        // arg: comparison value
	CmdAssertLe        // arg: comparison value
	CmdJNZ             // arg: offset
	CmdCall
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
	TypeFword
	TypeUFword
)

// JumpOffset encodes the jump distance for a relative jump
func JumpOffset(d int8) Command {
	return Command(d)
}

var encodeBool = map[bool]int64{true: 1}
