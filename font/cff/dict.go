package cff

import (
	"errors"
	"strconv"
)

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
			tmp, x, err := decodeFloat(buf)
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

const (
	// keyNotice         = 0x0001 // SID
	// keyFullName       = 0x0002 // SID
	// keyFamilyName     = 0x0003 // SID
	// keyFontBBox       = 0x0005
	// keyCharset        = 0x000F
	// keyCharStrings    = 0x0011
	// keyPrivate        = 0x0012
	// keyCopyright      = 0x0C00 // SID
	// keyUnderlinePos   = 0x0C03
	keyCharstringType = 0x0C06 // number (default=2)
	keyROS            = 0x0C1E
)
