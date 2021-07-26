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
	"seehuhn.de/go/pdf/font/sfnt/table"
)

const bufferSize = 1024

// State stores the current register values of the interpreter.
type State struct {
	A   int64
	R   [8]int64 // TODO(voss): let the caller allocate this instead?
	Tag string

	Stash []uint16
}

// GetStash return the slice of stashed values and clears the stash.
func (s *State) GetStash() []uint16 {
	res := s.Stash
	s.Stash = nil
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

	// TODO(voss): can I get rid of this?
	Funcs []func(*State)

	classDefCache map[int64]ClassDef
}

// New allocates a new Parser.  SetTable() must be called before the
// parser can be used.
func New(tt *sfnt.Font) *Parser {
	return &Parser{
		tt:    tt,
		scale: 1000 / float64(tt.Head.UnitsPerEm), // TODO(voss): fix this
	}
}

// OpenTable sets up the parser to read from the given table.
// The current reading position is moved to the start of the table.
func (p *Parser) OpenTable(tableName string) error {
	info := p.tt.Header.Find(tableName)
	if info == nil {
		return &table.ErrNoTable{Name: tableName}
	}

	p.tableName = tableName
	p.start = int64(info.Offset)
	p.end = int64(info.Offset) + int64(info.Length)

	p.classDefCache = make(map[int64]ClassDef)

	return p.seek(0)
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
		case CmdReadTag:
			buf, err := p.read(4)
			if err != nil {
				return err
			}
			s.Tag = string(buf)
		case CmdSeek:
			err := p.seek(s.A)
			if err != nil {
				return err
			}

		case CmdStoreInto:
			s.R[arg] = s.A
		case CmdLoadI:
			s.A = int64(int8(arg))
		case CmdLoad:
			s.A = s.R[arg]
		case CmdStash:
			buf, err := p.read(2)
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

		case CmdCmpEq:
			target := int64(int8(arg))
			s.A = encodeBool[s.A == target]
		case CmdCmpGe:
			target := int64(int8(arg))
			s.A = encodeBool[s.A >= target]
		case CmdCmpLt:
			target := int64(int8(arg))
			s.A = encodeBool[s.A < target]

		case CmdExitIfLt:
			target := int64(int8(arg))
			if s.A < target {
				break CommandLoop
			}

		case CmdAssertEq:
			target := int64(arg)
			if s.A != target {
				return p.error("expected 0x%04x, but got 0x%04x (%d)",
					target, s.A, PC)
			}
		case CmdAssertGe:
			target := int64(arg)
			if s.A < target {
				return p.error("expected >=0x%04x, but got 0x%04x (%d)",
					target, s.A, PC)
			}
		case CmdAssertGt:
			target := int64(arg)
			if s.A <= target {
				return p.error("expected >0x%04x, but got 0x%04x (%d)",
					target, s.A, PC)
			}
		case CmdAssertLe:
			target := int64(arg)
			if s.A > target {
				return p.error("expected <=0x%04x, but got 0x%04x (%d)",
					target, s.A, PC)
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

// ReadUInt16 reads a single uint16 value from the current position.
func (p *Parser) ReadUInt16() (uint16, error) {
	buf, err := p.read(2)
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

func (p *Parser) seek(posInTable int64) error {
	filePos := p.start + posInTable
	if filePos < p.start || filePos > p.end {
		return p.error("seek target %d is outside [%d,%d]",
			filePos, p.start, p.end)
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
	if n < 0 {
		n = 0
	} else if n > bufferSize {
		n = bufferSize
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

		l, err := p.tt.Fd.Read(p.buf[p.used:])
		if err == io.EOF {
			if l > 0 {
				err = nil
			} else {
				err = io.ErrUnexpectedEOF
			}
		}
		if err != nil {
			return nil, p.error("read failed: %w", err)
		}
		p.used += l
	}

	res := p.buf[p.pos : p.pos+n]
	p.pos += n
	return res, nil
}

func (p *Parser) error(format string, a ...interface{}) error {
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
	CmdRead16    = iota // arg: type
	CmdRead32           // arg: type
	CmdStoreInto        // arg: register
	CmdLoadI            // arg: new int8 value for A
	CmdLoad             // arg: register
	CmdIAdd             // arg: int8 literal value
	CmdAdd              // arg: register
	CmdIMult            // arg: int8 literal value
	CmdMult             // arg: register
	CmdCmpEq            // arg: comparison value
	CmdCmpGe            // arg: comparison value
	CmdCmpLt            // arg: comparison value
	CmdExitIfLt         // arg: comparison value
	CmdAssertEq         // arg: comparison value
	CmdAssertGe         // arg: comparison value
	CmdAssertGt         // arg: comparison value
	CmdAssertLe         // arg: comparison value
	CmdJNZ              // arg: offset
	CmdCall             // arg: Func index
)

// Commands which take no arguments
const (
	CmdSeek = iota + 128
	CmdReadTag
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
