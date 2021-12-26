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
	"errors"
	"fmt"
)

type stackSlot struct {
	val   int32
	isInt bool
}

func (s stackSlot) String() string {
	if s.isInt {
		return fmt.Sprintf("%d", s.val)
	}
	return fmt.Sprintf("%d?", s.val)
}

func (cff *Font) decodeCharString(cc []byte) ([][]byte, error) {
	var cmds [][]byte
	var stack []stackSlot
	var nStems int

	storage := make(map[int]stackSlot)
	cmdStack := [][]byte{cc}

glyphLoop:
	for len(cmdStack) > 0 {
		cc = cmdStack[len(cmdStack)-1]
		cmdStack = cmdStack[:len(cmdStack)-1]

	subrLoop:
		for len(cc) > 0 {
			op := t2op(cc[0])

			// {
			// 	show := cc[1:]
			// 	tail := ""
			// 	info := ""
			// 	if k := len(stack); (op == t2callgsubr || op == t2callsubr) && k > 0 && stack[k-1].isInt {
			// 		idx := int(stack[k-1].val) + bias(len(cff.subrs))
			// 		info = fmt.Sprintf("@%d", idx)
			// 	}
			// 	if len(show) > 10 {
			// 		show = show[:10]
			// 		tail = "..."
			// 	}
			// 	fmt.Println(stack, " ", op.String()+info, " ", show, tail)
			// }

			if op >= 32 && op <= 246 {
				stack = append(stack, stackSlot{
					val:   int32(op) - 139,
					isInt: true,
				})
				cmds = append(cmds, cc[:1])
				cc = cc[1:]
				continue
			} else if op >= 247 && op <= 250 {
				if len(cc) < 2 {
					return nil, errors.New("incomplete command")
				}
				stack = append(stack, stackSlot{
					val:   int32(op)*256 + int32(cc[1]) + (108 - 247*256),
					isInt: true,
				})
				cmds = append(cmds, cc[:2])
				cc = cc[2:]
				continue
			} else if op >= 251 && op <= 254 {
				if len(cc) < 2 {
					return nil, errors.New("incomplete command")
				}
				stack = append(stack, stackSlot{
					val:   -int32(op)*256 - int32(cc[0]) - (108 - 251*256),
					isInt: true,
				})
				cmds = append(cmds, cc[:2])
				cc = cc[2:]
				continue
			} else if op == 28 {
				if len(cc) < 3 {
					return nil, errors.New("incomplete command")
				}
				stack = append(stack, stackSlot{
					val:   int32(int16(uint16(cc[1])<<8 + uint16(cc[2]))),
					isInt: true,
				})
				cmds = append(cmds, cc[:3])
				cc = cc[3:]
				continue
			} else if op == 255 {
				if len(cc) < 5 {
					return nil, errors.New("incomplete command")
				}
				x := int32(uint32(cc[1])<<24 + uint32(cc[2])<<16 + uint32(cc[3])<<8 + uint32(cc[4]))
				// 16-bit signed integer with 16 bits of fraction
				stack = append(stack, stackSlot{
					val:   x >> 16,
					isInt: false,
				})
				cmds = append(cmds, cc[:5])
				cc = cc[5:]
				continue
			}

			var cmd []byte
			if op == 0x0c {
				if len(cc) < 2 {
					return nil, errors.New("incomplete command")
				}
				op = op<<8 | t2op(cc[1])
				cmd, cc = cc[:2], cc[2:]
			} else {
				cmd, cc = cc[:1], cc[1:]
			}

			switch op {
			case t2rmoveto, t2hmoveto, t2vmoveto, t2rlineto, t2hlineto, t2vlineto,
				t2rrcurveto, t2hhcurveto, t2hvcurveto, t2rcurveline, t2rlinecurve,
				t2vhcurveto, t2vvcurveto, t2flex, t2hflex, t2hflex1, t2flex1:
				// all path construction operators clear the stack
				stack = stack[:0]

			case t2dotsection: // deprecated
				stack = stack[:0]

			case t2hstem, t2vstem, t2hstemhm, t2vstemhm:
				nStems += len(stack) / 2
				stack = stack[:0]

			case t2hintmask, t2cntrmask:
				nStems += len(stack) / 2 // TODO(voss): check this!  (maybe via freetype implementation)
				k := (nStems + 7) / 8
				cmd = append(cmd, cc[:k]...)
				cc = cc[k:]
				stack = stack[:0]

			case t2abs:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				if stack[k].val < 0 {
					stack[k].val = -stack[k].val
				}
			case t2add:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val += stack[k+1].val
				stack[k].isInt = stack[k].isInt && stack[k+1].isInt
				stack = stack[:k+1]
			case t2sub:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val -= stack[k+1].val
				stack[k].isInt = stack[k].isInt && stack[k+1].isInt
				stack = stack[:k+1]
			case t2div:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				a := stack[k].val
				b := stack[k+1].val
				if b != 0 && a%b == 0 && stack[k].isInt && stack[k+1].isInt {
					stack[k].val = a / b
				} else {
					stack[k].isInt = false
				}
				stack = stack[:k+1]
			case t2neg:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val = -stack[k].val
			case t2random:
				stack = append(stack, stackSlot{
					val:   0,
					isInt: false,
				})
			case t2mul:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val *= stack[k+1].val
				stack[k].isInt = stack[k].isInt && stack[k+1].isInt
				stack = stack[:k+1]
			case t2sqrt:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].isInt = false
			case t2drop:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack = stack[:k]
			case t2exch:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k], stack[k+1] = stack[k+1], stack[k]
			case t2index:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				idx := int(stack[k].val)
				if idx < 0 || k-idx-1 < 0 || !stack[k].isInt {
					return nil, errors.New("invalid index")
				}
				stack[k] = stack[k-idx-1]
			case t2roll:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				n := int(stack[k].val)
				j := int(stack[k+1].val)
				if n <= 0 || n > k {
					return nil, errors.New("invalid roll count")
				}
				roll(stack[k-n:k], j)
				stack = stack[:k]
			case t2dup:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack = append(stack, stack[k])

			case t2put:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				m := int(stack[k+1].val)
				if m < 0 || m > 32 || !stack[k+1].isInt {
					return nil, errors.New("invalid store index")
				}
				storage[m] = stack[k]
				stack = stack[:k]
			case t2get:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				m := int(stack[k].val)
				if m < 0 || m > 32 || !stack[k].isInt {
					return nil, errors.New("invalid store index")
				}
				stack[k] = storage[m]

			case t2and:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				var m stackSlot
				if stack[k].val != 0 && stack[k+1].val != 0 {
					m.val = 1
				}
				m.isInt = stack[k].isInt && stack[k+1].isInt
				stack = append(stack[:k], m)
			case t2or:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				var m stackSlot
				if stack[k].val != 0 || stack[k+1].val != 0 {
					m.val = 1
				}
				m.isInt = stack[k].isInt && stack[k+1].isInt
				stack = append(stack[:k], m)
			case t2not:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				if stack[k].val == 0 {
					stack[k].val = 1
				} else {
					stack[k].val = 0
				}
			case t2eq:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				if stack[k].val == stack[k+1].val {
					stack[k].val = 1
				} else {
					stack[k].val = 0
				}
				stack[k].isInt = stack[k].isInt && stack[k+1].isInt
				stack = stack[:k+1]
			case t2ifelse:
				k := len(stack) - 4
				if k < 0 {
					return nil, errStackUnderflow
				}
				var m stackSlot
				if stack[k+2].val <= stack[k+3].val {
					m = stack[k]
				} else {
					m = stack[k+1]
				}
				m.isInt = m.isInt && stack[k+2].isInt && stack[k+3].isInt
				stack = append(stack[:k], m)

			case t2callsubr, t2callgsubr:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				if !stack[k].isInt {
					return nil, errors.New("invalid subroutine index")
				}
				biased := int(stack[k].val)
				stack = stack[:k]

				if len(cc) > 0 {
					cmdStack = append(cmdStack, cc)
				}

				var err error
				if op == t2callsubr {
					cc, err = cff.getSubr(biased)
				} else {
					cc, err = cff.getGSubr(biased)
				}
				if err != nil {
					return nil, err
				}

				// remove the subroutine index from the stack
				l := len(cmds) - 1
				if isConstInt(cmds[l]) {
					cmds = cmds[:l]
					continue subrLoop
				} else {
					cmd = []byte{12, 18} // t2drop
				}

			case t2return:
				break subrLoop

			case t2endchar:
				break glyphLoop

			default:
				// return nil, fmt.Errorf("unsupported opcode %d", op)
				fmt.Printf("unsupported opcode %d\n", op)
			}
			cmds = append(cmds, cmd)
		} // end of subrLoop
	} // end of glyphLoop
	cmds = append(cmds, []byte{14}) // t2endchar
	return cmds, nil
}

func roll(data []stackSlot, j int) {
	n := len(data)

	j = j % n
	if j < 0 {
		j += n
	}

	tmp := make([]stackSlot, j)
	copy(tmp, data[n-j:])
	copy(data[j:], data[:n-j])
	copy(data[:j], tmp)
}

func isConstInt(cmd []byte) bool {
	if len(cmd) == 0 {
		return false
	}
	op := cmd[0]
	return op == 28 || (32 <= op && op <= 254)
}

type t2op uint16

func (op t2op) String() string {
	switch op {
	case t2hstem:
		return "t2hstem"
	case t2vstem:
		return "t2vstem"
	case t2vmoveto:
		return "t2vmoveto"
	case t2rlineto:
		return "t2rlineto"
	case t2hlineto:
		return "t2hlineto"
	case t2vlineto:
		return "t2vlineto"
	case t2rrcurveto:
		return "t2rrcurveto"
	case t2callsubr:
		return "t2callsubr"
	case t2return:
		return "t2return"
	case t2endchar:
		return "t2endchar"
	case t2hstemhm:
		return "t2hstemhm"
	case t2hintmask:
		return "t2hintmask"
	case t2cntrmask:
		return "t2cntrmask"
	case t2rmoveto:
		return "t2rmoveto"
	case t2hmoveto:
		return "t2hmoveto"
	case t2vstemhm:
		return "t2vstemhm"
	case t2rcurveline:
		return "t2rcurveline"
	case t2rlinecurve:
		return "t2rlinecurve"
	case t2vvcurveto:
		return "t2vvcurveto"
	case t2hhcurveto:
		return "t2hhcurveto"
	case t2shortint:
		return "t2int3"
	case t2callgsubr:
		return "t2callgsubr"
	case t2vhcurveto:
		return "t2vhcurveto"
	case t2hvcurveto:
		return "t2hvcurveto"
	case t2dotsection:
		return "t2dotsection"
	case t2and:
		return "t2and"
	case t2or:
		return "t2or"
	case t2not:
		return "t2not"
	case t2abs:
		return "t2abs"
	case t2add:
		return "t2add"
	case t2sub:
		return "t2sub"
	case t2div:
		return "t2div"
	case t2neg:
		return "t2neg"
	case t2eq:
		return "t2eq"
	case t2drop:
		return "t2drop"
	case t2put:
		return "t2put"
	case t2get:
		return "t2get"
	case t2ifelse:
		return "t2ifelse"
	case t2random:
		return "t2random"
	case t2mul:
		return "t2mul"
	case t2sqrt:
		return "t2sqrt"
	case t2dup:
		return "t2dup"
	case t2exch:
		return "t2exch"
	case t2index:
		return "t2index"
	case t2roll:
		return "t2roll"
	case t2hflex:
		return "t2hflex"
	case t2flex:
		return "t2flex"
	case t2hflex1:
		return "t2hflex1"
	case t2flex1:
		return "t2flex1"
	case 255:
		return "t2float4"
	}
	if 32 <= op && op <= 246 {
		return fmt.Sprintf("t2int1(%d)", op)
	}
	if 247 <= op && op <= 254 {
		return fmt.Sprintf("t2int2(%d)", op)
	}
	return fmt.Sprintf("t2op(%d)", op)
}

const (
	t2hstem      t2op = 0x0001
	t2vstem      t2op = 0x0003
	t2vmoveto    t2op = 0x0004
	t2rlineto    t2op = 0x0005
	t2hlineto    t2op = 0x0006
	t2vlineto    t2op = 0x0007
	t2rrcurveto  t2op = 0x0008
	t2callsubr   t2op = 0x000a
	t2return     t2op = 0x000b
	t2endchar    t2op = 0x000e
	t2hstemhm    t2op = 0x0012
	t2hintmask   t2op = 0x0013
	t2cntrmask   t2op = 0x0014
	t2rmoveto    t2op = 0x0015
	t2hmoveto    t2op = 0x0016
	t2vstemhm    t2op = 0x0017
	t2rcurveline t2op = 0x0018
	t2rlinecurve t2op = 0x0019
	t2vvcurveto  t2op = 0x001a
	t2hhcurveto  t2op = 0x001b
	t2shortint   t2op = 0x001c
	t2callgsubr  t2op = 0x001d
	t2vhcurveto  t2op = 0x001e
	t2hvcurveto  t2op = 0x001f

	t2dotsection t2op = 0x0c00
	t2and        t2op = 0x0c03
	t2or         t2op = 0x0c04
	t2not        t2op = 0x0c05
	t2abs        t2op = 0x0c09
	t2add        t2op = 0x0c0a
	t2sub        t2op = 0x0c0b
	t2div        t2op = 0x0c0c
	t2neg        t2op = 0x0c0e
	t2eq         t2op = 0x0c0f
	t2drop       t2op = 0x0c12
	t2put        t2op = 0x0c14
	t2get        t2op = 0x0c15
	t2ifelse     t2op = 0x0c16
	t2random     t2op = 0x0c17
	t2mul        t2op = 0x0c18
	t2sqrt       t2op = 0x0c1a
	t2dup        t2op = 0x0c1b
	t2exch       t2op = 0x0c1c
	t2index      t2op = 0x0c1d
	t2roll       t2op = 0x0c1e
	t2hflex      t2op = 0x0c22
	t2flex       t2op = 0x0c23
	t2hflex1     t2op = 0x0c24
	t2flex1      t2op = 0x0c25
)
