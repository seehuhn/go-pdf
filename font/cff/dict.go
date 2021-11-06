package cff

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

var errCorruptDict = errors.New("invalid CFF DICT")

type cffDict map[dictOp][]interface{}

func decodeDict(buf []byte) (cffDict, error) {
	res := cffDict{}
	var stack []interface{}

	flush := func(op dictOp) {
		res[op] = stack
		stack = nil
	}

	for len(buf) > 0 {
		b0 := buf[0]
		switch {
		case b0 == 12:
			if len(buf) < 2 {
				return nil, errCorruptDict
			}
			flush(dictOp(b0)<<8 + dictOp(buf[1]))
			buf = buf[2:]
		case b0 <= 21:
			flush(dictOp(b0))
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
			tmp, x, err := decodeFloat(buf[1:])
			if err != nil {
				return nil, err
			}
			stack = append(stack, x)
			buf = tmp
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

	if len(stack) > 0 {
		return nil, errCorruptDict
	}

	return res, nil
}

// decodes a float (without the leading 0x1e)
func decodeFloat(buf []byte) ([]byte, float64, error) {
	var s []byte

	first := true
	var next byte
	for {
		var nibble byte
		if first {
			if len(buf) == 0 {
				return nil, 0, errors.New("incomplete float")
			}
			next, buf = buf[0], buf[1:]
			nibble = next >> 4
			next = next & 15
			first = false
		} else {
			nibble = next
			first = true
		}

		switch nibble {
		case 0x0a:
			s = append(s, '.')
		case 0xb:
			s = append(s, 'e')
		case 0xc:
			s = append(s, 'e', '-')
		case 0xd: // reserved
			return nil, 0, errors.New("unsupported float format")
		case 0xe:
			s = append(s, '-')
		case 0xf:
			x, err := strconv.ParseFloat(string(s), 64)
			return buf, x, err
		default:
			s = append(s, '0'+nibble)
		}
	}
}

func (d cffDict) getInt(op dictOp, defVal int32) (int32, bool) {
	if len(d[op]) != 1 {
		return defVal, false
	}
	x, ok := d[op][0].(int32)
	if !ok {
		return defVal, false
	}
	return x, true
}

func (d cffDict) getPair(op dictOp) (int32, int32, bool) {
	xy := d[op]
	if len(xy) != 2 {
		return 0, 0, false
	}
	x, ok := xy[0].(int32)
	if !ok {
		return 0, 0, false
	}
	y, ok := xy[1].(int32)
	if !ok {
		return 0, 0, false
	}
	return x, y, true
}

func (d cffDict) keys() []dictOp {
	keys := make([]dictOp, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	conv := func(op dictOp) int {
		if op == opROS {
			return -1
		} else if op == opSyntheticBase {
			return -2
		}
		return int(op)
	}
	sort.Slice(keys, func(i, j int) bool {
		return conv(keys[i]) < conv(keys[j])
	})
	return keys
}

func (d cffDict) encode() []byte {
	keys := d.keys()

	res := &bytes.Buffer{}
	for _, op := range keys {
		args := d[op]
		for _, arg := range args {
			switch a := arg.(type) {
			case int32:
				switch {
				case a >= -107 && a <= 107:
					res.WriteByte(byte(a + 139))
				case a >= 108 && a <= 1131:
					// a = (b0–247)*256+b1+108
					a -= 108
					b1 := byte(a)
					a >>= 8
					b0 := byte(a + 247)
					res.Write([]byte{b0, b1})
				case a >= -1131 && a <= -108:
					// a = -(b0–251)*256-b1-108
					a = -108 - a
					b1 := byte(a)
					a >>= 8
					b0 := byte(a + 251)
					res.Write([]byte{b0, b1})
				case a >= -32768 && a <= 32767:
					a16 := uint16(a)
					res.Write([]byte{28, byte(a16 >> 8), byte(a16)})
				default:
					a32 := uint32(a)
					res.Write([]byte{29, byte(a32 >> 24), byte(a32 >> 16), byte(a32 >> 8), byte(a32)})
				}
			case float64:
				res.WriteByte(0x1e)
				s := fmt.Sprintf("%g::", a)
				first := true
				var tmp byte
				for i := 0; i < len(s); i++ {
					var nibble byte
					c := s[i]
					switch {
					case c >= '0' && c <= '9':
						nibble = c - '0'
					case c == '.':
						nibble = 0x0a
					case c == 'e':
						if s[i+1] != '-' {
							nibble = 0x0b
						} else {
							i++
							nibble = 0x0c
						}
					case i == 0 && c == '-':
						nibble = 0x0e
					case c == ':':
						nibble = 0x0f
					}
					if first {
						tmp = nibble << 4
					} else {
						res.WriteByte(tmp | nibble)
					}
					first = !first
				}
			}
		}
		if op > 255 {
			if op>>8 != 12 {
				panic("invalid DICT operator")
			}
			res.WriteByte(12)
		}
		res.WriteByte(byte(op))
	}
	return res.Bytes()
}

type dictOp uint16

func (d dictOp) String() string {
	switch d {
	case opCharset:
		return "Charset"
	case opCharStrings:
		return "CharStrings"
	case opCharstringType:
		return "CharstringType"
	case opSyntheticBase:
		return "SyntheticBase"
	case opROS:
		return "ROS"
	default:
		if d < 256 {
			return fmt.Sprintf("%d", d)
		}
		return fmt.Sprintf("%d %d", d>>8, d&0xff)
	}
}

const (
	opCharset        dictOp = 0x000F
	opCharStrings    dictOp = 0x0011
	opCharstringType dictOp = 0x0C06 // number (default=2)
	opSyntheticBase  dictOp = 0x0C14
	opROS            dictOp = 0x0C1E
	opPrivate        dictOp = 0x0012

	// opNotice         = 0x0001 // SID
	// opFullName       = 0x0002 // SID
	// opFamilyName     = 0x0003 // SID
	// opFontBBox       = 0x0005
	// opPrivate        = 0x0012
	// opCopyright      = 0x0C00 // SID
	// opUnderlinePos   = 0x0C03
)
