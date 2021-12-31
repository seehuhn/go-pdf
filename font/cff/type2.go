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
	"math"
)

type stackSlot struct {
	val    float64
	random bool
}

func isInt(x float64) bool {
	return x == float64(int32(x))
}

func (s stackSlot) String() string {
	suffix := ""
	if s.random {
		suffix = "*"
	}
	res := fmt.Sprintf("%.3f", s.val)
	if isInt(s.val) {
		res = fmt.Sprintf("%d", int32(s.val))
	}
	return res + suffix
}

// A Renderer is used to draw a glyph encoded by a CFF charstring.
type Renderer interface {
	SetWidth(w int)
	RMoveTo(x, y float64)
	RLineTo(x, y float64)
	RCurveTo(dxa, dya, dxb, dyb, dxc, dyc float64)
}

// DecodeCharString uses ctx to render the charstring for glyph i.
func (cff *Font) DecodeCharString(ctx Renderer, i int) error {
	if i < 0 || i >= len(cff.charStrings) {
		return errors.New("invalid glyph index")
	}

	_, err := cff.doDecode(ctx, i)
	return err
}

// doDecode returns the commands for the given charstring.
func (cff *Font) doDecode(ctx Renderer, i int) ([][]byte, error) {
	body := cff.charStrings[i]
	var cmds [][]byte
	skipBytes := func(n int) {
		cmds = append(cmds, body[:n])
		body = body[n:]
	}

	var stack []stackSlot
	clearStack := func() {
		stack = stack[:0]
	}

	widthIsSet := false
	setGlyphWidth := func(isPresent bool) {
		if widthIsSet {
			return
		}
		var glyphWidth int32
		if isPresent {
			dw, _ := cff.privateDict.getInt(opNominalWidthX, 0)
			glyphWidth = int32(stack[0].val) + dw
			stack = stack[1:]
		} else {
			glyphWidth, _ = cff.privateDict.getInt(opDefaultWidthX, 0)
		}
		if ctx != nil {
			ctx.SetWidth(int(glyphWidth))
		}
		widthIsSet = true
	}

	storage := make(map[int]stackSlot)
	cmdStack := [][]byte{body}
	var nStems int

	for len(cmdStack) > 0 {
		cmdStack, body = cmdStack[:len(cmdStack)-1], cmdStack[len(cmdStack)-1]

	opLoop:
		for len(body) > 0 {
			if len(stack) > 48 {
				return nil, errStackOverflow
			}

			op := t2op(body[0])

			// {
			// 	show := body[1:]
			// 	tail := ""
			// 	info := ""
			// 	if k := len(stack); (op == t2callgsubr || op == t2callsubr) &&
			// 		k > 0 && isInt(stack[k-1].val) {
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
					val: float64(int32(op) - 139),
				})
				skipBytes(1)
				continue
			} else if op >= 247 && op <= 250 {
				if len(body) < 2 {
					return nil, errIncomplete
				}
				stack = append(stack, stackSlot{
					val: float64(int32(op)*256 + int32(body[1]) + (108 - 247*256)),
				})
				skipBytes(2)
				continue
			} else if op >= 251 && op <= 254 {
				if len(body) < 2 {
					return nil, errIncomplete
				}
				stack = append(stack, stackSlot{
					val: float64(-int32(op)*256 - int32(body[1]) - (108 - 251*256)),
				})
				skipBytes(2)
				continue
			} else if op == 28 {
				if len(body) < 3 {
					return nil, errIncomplete
				}
				stack = append(stack, stackSlot{
					val: float64(int16(uint16(body[1])<<8 + uint16(body[2]))),
				})
				skipBytes(3)
				continue
			} else if op == 255 {
				if len(body) < 5 {
					return nil, errIncomplete
				}
				// 16-bit signed integer with 16 bits of fraction
				x := int32(uint32(body[1])<<24 + uint32(body[2])<<16 +
					uint32(body[3])<<8 + uint32(body[4]))
				stack = append(stack, stackSlot{
					val: float64(x) / 65536,
				})
				skipBytes(5)
				continue
			}

			var cmd []byte
			if op == 0x0c {
				if len(body) < 2 {
					return nil, errIncomplete
				}
				op = op<<8 | t2op(body[1])
				cmd, body = body[:2], body[2:]
			} else {
				cmd, body = body[:1], body[1:]
			}

			switch op {
			case t2rmoveto:
				setGlyphWidth(len(stack) > 2)
				if ctx != nil && len(stack) >= 2 {
					ctx.RMoveTo(stack[0].val, stack[1].val)
				}
				clearStack()

			case t2hmoveto:
				setGlyphWidth(len(stack) > 1)
				if ctx != nil && len(stack) >= 1 {
					ctx.RMoveTo(stack[0].val, 0)
				}
				clearStack()

			case t2vmoveto:
				setGlyphWidth(len(stack) > 1)
				if ctx != nil && len(stack) >= 1 {
					ctx.RMoveTo(0, stack[0].val)
				}
				clearStack()

			case t2rlineto:
				if ctx != nil {
					for len(stack) >= 2 {
						ctx.RLineTo(stack[0].val, stack[1].val)
						stack = stack[2:]
					}
				}
				clearStack()

			case t2hlineto, t2vlineto:
				if ctx != nil {
					horizontal := op == t2hlineto
					for len(stack) > 0 {
						if horizontal {
							ctx.RLineTo(stack[0].val, 0)
						} else {
							ctx.RLineTo(0, stack[0].val)
						}
						stack = stack[1:]
						horizontal = !horizontal
					}
				} else {
					clearStack()
				}

			case t2rrcurveto, t2rcurveline, t2rlinecurve:
				if ctx != nil {
					for op == t2rlinecurve && len(stack) >= 8 {
						ctx.RLineTo(stack[0].val, stack[1].val)
						stack = stack[2:]
					}
					for len(stack) >= 6 {
						ctx.RCurveTo(stack[0].val, stack[1].val,
							stack[2].val, stack[3].val,
							stack[4].val, stack[5].val)
						stack = stack[6:]
					}
					if op == t2rcurveline && len(stack) >= 2 {
						ctx.RLineTo(stack[0].val, stack[1].val)
						stack = stack[2:]
					}
				}
				clearStack()

			case t2hhcurveto:
				if ctx != nil {
					var dy1 float64
					if len(stack)%4 != 0 {
						dy1, stack = stack[0].val, stack[1:]
					}
					for len(stack) >= 4 {
						ctx.RCurveTo(stack[0].val, dy1,
							stack[1].val, stack[2].val,
							stack[3].val, 0)
						stack = stack[4:]
						dy1 = 0
					}
				}
				clearStack()

			case t2hvcurveto, t2vhcurveto:
				if ctx != nil {
					horizontal := op == t2hvcurveto
					for len(stack) >= 4 {
						var extra float64
						if len(stack) == 5 {
							extra = stack[4].val
						}
						if horizontal {
							ctx.RCurveTo(stack[0].val, 0,
								stack[1].val, stack[2].val,
								extra, stack[3].val)
						} else {
							ctx.RCurveTo(0, stack[0].val,
								stack[1].val, stack[2].val,
								stack[3].val, extra)
						}
						stack = stack[4:]
						horizontal = !horizontal
					}
				}
				clearStack()

			case t2vvcurveto:
				if ctx != nil {
					var dx1 float64
					if len(stack)%4 != 0 {
						dx1, stack = stack[0].val, stack[1:]
					}
					for len(stack) >= 4 {
						ctx.RCurveTo(dx1, stack[0].val,
							stack[1].val, stack[2].val,
							0, stack[3].val)
						stack = stack[4:]
						dx1 = 0
					}
				}
				clearStack()

			case t2flex:
				if ctx != nil {
					if len(stack) >= 13 {
						ctx.RCurveTo(stack[0].val, stack[1].val,
							stack[2].val, stack[3].val,
							stack[4].val, stack[5].val)
						ctx.RCurveTo(stack[6].val, stack[7].val,
							stack[8].val, stack[9].val,
							stack[10].val, stack[11].val)
						// fd = stack[12].val / 100
					}
				}
				clearStack()
			case t2flex1:
				if ctx != nil {
					if len(stack) >= 11 {
						ctx.RCurveTo(stack[0].val, stack[1].val,
							stack[2].val, stack[3].val,
							stack[4].val, stack[5].val)
						extra := stack[10].val
						dx := stack[0].val + stack[2].val + stack[4].val + stack[6].val + stack[8].val
						dy := stack[1].val + stack[3].val + stack[5].val + stack[7].val + stack[9].val
						if math.Abs(dx) > math.Abs(dy) {
							ctx.RCurveTo(stack[6].val, stack[7].val,
								stack[8].val, stack[9].val,
								extra, 0)
						} else {
							ctx.RCurveTo(stack[6].val, stack[7].val,
								stack[8].val, stack[9].val,
								0, extra)
						}
						// fd = 0.5
					}
				}
				clearStack()
			case t2hflex:
				if ctx != nil {
					if len(stack) >= 7 {
						ctx.RCurveTo(stack[0].val, 0,
							stack[1].val, stack[2].val,
							stack[3].val, 0)
						ctx.RCurveTo(stack[4].val, 0,
							stack[5].val, -stack[2].val,
							stack[6].val, 0)
						// fd = 0.5
					}
				}
				clearStack()
			case t2hflex1:
				if ctx != nil {
					if len(stack) >= 9 {
						ctx.RCurveTo(stack[0].val, stack[1].val,
							stack[2].val, stack[3].val,
							stack[4].val, 0)
						dy := stack[1].val + stack[3].val + stack[5].val + stack[7].val
						ctx.RCurveTo(stack[5].val, 0,
							stack[6].val, stack[7].val,
							stack[8].val, -dy)
						// fd = 0.5
					}
				}
				clearStack()

			case t2dotsection: // deprecated
				clearStack()

			case t2hstem, t2vstem, t2hstemhm, t2vstemhm:
				setGlyphWidth(len(stack)%2 == 1)
				nStems += len(stack) / 2
				clearStack()

			case t2hintmask, t2cntrmask:
				setGlyphWidth(len(stack)%2 == 1)
				// "If hstem and vstem hints are both declared at the beginning
				// of a charstring, and this sequence is followed directly by
				// the hintmask or cntrmask operators, the vstem hint operator
				// need not be included."
				nStems += len(stack) / 2
				k := (nStems + 7) / 8
				if k >= len(body) {
					return nil, errIncomplete
				}
				cmd = append(cmd, body[:k]...)
				body = body[k:]
				clearStack()

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
				stack[k].random = stack[k].random || stack[k+1].random
				stack = stack[:k+1]
			case t2sub:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val -= stack[k+1].val
				stack[k].random = stack[k].random || stack[k+1].random
				stack = stack[:k+1]
			case t2div:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val /= stack[k+1].val
				stack[k].random = stack[k].random || stack[k+1].random
				stack = stack[:k+1]
			case t2neg:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val = -stack[k].val
			case t2random:
				stack = append(stack, stackSlot{
					val:    0.618,
					random: true,
				})
			case t2mul:
				k := len(stack) - 2
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val *= stack[k+1].val
				stack[k].random = stack[k].random || stack[k+1].random
				stack = stack[:k+1]
			case t2sqrt:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				stack[k].val = math.Sqrt(stack[k].val)
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
				if float64(idx) != stack[k].val || k-idx-1 < 0 {
					return nil, errors.New("invalid index")
				}
				if idx < 0 {
					idx = 0
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
				if float64(m) != stack[k+1].val || m < 0 || m > 32 {
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
				if float64(m) != stack[k+1].val || m < 0 || m > 32 {
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
				stack[k].random = stack[k].random || stack[k+1].random
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
				stack[k].random = stack[k].random || stack[k+1].random
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
				stack[k].random = stack[k].random || stack[k+1].random
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
				m.random = m.random || stack[k+2].random || stack[k+3].random
				stack = append(stack[:k], m)

			case t2callsubr, t2callgsubr:
				k := len(stack) - 1
				if k < 0 {
					return nil, errStackUnderflow
				}
				if stack[k].random {
					return nil, errInvalidSubroutine
				}
				biased := int(stack[k].val)
				stack = stack[:k]

				cmdStack = append(cmdStack, body)
				if len(cmdStack) > 10 {
					return nil, errors.New("maximum call stack size exceeded")
				}

				var err error
				if op == t2callsubr {
					body, err = cff.getSubr(biased)
				} else {
					body, err = cff.getGSubr(biased)
				}
				if err != nil {
					return nil, err
				}

				// remove the subroutine index from the stack
				l := len(cmds) - 1
				if isConstInt(cmds[l]) {
					cmds = cmds[:l]
					continue opLoop
				} else {
					cmd = []byte{12, 18} // t2drop
				}

			case t2return:
				break opLoop

			case t2endchar:
				setGlyphWidth(len(stack) == 1 || len(stack) > 4)
				cmds = append(cmds, []byte{14}) // t2endchar
				return cmds, nil

			default:
				// return nil, fmt.Errorf("unsupported opcode %d", op)
				fmt.Printf("unsupported opcode %d\n", op)
			}
			cmds = append(cmds, cmd)
		} // end of opLoop
	}

	return nil, errIncomplete
}

func isConstInt(cmd []byte) bool {
	if len(cmd) == 0 {
		return false
	}
	op := cmd[0]
	return op == 28 || (32 <= op && op <= 254)
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

var (
	errStackOverflow     = errors.New("operand stack overflow")
	errStackUnderflow    = errors.New("operand stack underflow")
	errIncomplete        = errors.New("incomplete type2 charstring")
	errInvalidSubroutine = errors.New("invalid subroutine index")
)
