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
	"fmt"
	"math"
	"sort"
	"strconv"

	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/type1"
)

type cffDict map[dictOp][]interface{}

func decodeDict(buf []byte, ss *cffStrings) (cffDict, error) {
	res := cffDict{}
	var stack []interface{}

	flush := func(op dictOp) {
		if op.isString() {
			l := len(stack)
			if l > 2 { // special case for opROS
				l = 2
			}
			for i := 0; i < l; i++ {
				s, ok := stack[i].(int32)
				if ok {
					stack[i] = ss.get(s)
				} else {
					stack[i] = ""
				}
			}
		}
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

func (d cffDict) Copy() cffDict {
	c := make(cffDict, len(d))
	for k, v := range d {
		c[k] = append([]interface{}{}, v...)
	}
	return c
}

func (d cffDict) getInt(op dictOp, defVal int32) int32 {
	if len(d[op]) != 1 {
		return defVal
	}
	x, ok := d[op][0].(int32)
	if !ok {
		return defVal
	}
	return x
}

func (d cffDict) getFloat(op dictOp, defVal float64) float64 {
	if len(d[op]) != 1 {
		return defVal
	}
	switch x := d[op][0].(type) {
	case int32:
		return float64(x)
	case float64:
		return x
	default:
		return defVal
	}
}

func (d cffDict) getString(op dictOp) string {
	if len(d[op]) != 1 {
		return ""
	}
	x, _ := d[op][0].(string)
	return x
}

func (d cffDict) getDelta32(op dictOp) []int32 {
	values := d[op]
	if len(values) == 0 {
		return nil
	}
	res := make([]int32, len(values))
	var prev int32
	for i, v := range values {
		x, ok := v.(int32)
		if !ok {
			return nil
		}
		res[i] = x + prev
		prev = res[i]
	}
	return res
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

func (d cffDict) getFontMatrix(op dictOp) []float64 {
	xx, ok := d[op]
	if !ok || len(xx) != 6 {
		return defaultFontMatrix
	}

	res := make([]float64, 6)
	for i, x := range xx {
		xi, ok := x.(float64)
		if !ok {
			return defaultFontMatrix
		}
		res[i] = xi
	}

	return res
}

type privateInfo struct {
	private      *type1.PrivateDict
	fontMatrix   []float64 // TODO(voss): use or remove this
	subrs        cffIndex
	defaultWidth int32
	nominalWidth int32
}

func (d cffDict) readPrivate(p *parser.Parser, strings *cffStrings) (*privateInfo, error) {
	pdSize, pdOffs, ok := d.getPair(opPrivate)
	if !ok || pdOffs < 4 || pdSize < 0 {
		return nil, errors.New("cff: missing Private DICT")
	}

	err := p.SeekPos(int64(pdOffs))
	if err != nil {
		return nil, err
	}

	privateDictBlob := make([]byte, pdSize)
	_, err = p.Read(privateDictBlob)
	if err != nil {
		return nil, err
	}

	privateDict, err := decodeDict(privateDictBlob, strings)
	if err != nil {
		return nil, err
	}

	private := &type1.PrivateDict{
		BlueValues: privateDict.getDelta32(opBlueValues),
		OtherBlues: privateDict.getDelta32(opOtherBlues),
		BlueScale:  privateDict.getFloat(opBlueScale, defaultBlueScale),
		BlueShift:  privateDict.getInt(opBlueShift, 7),
		BlueFuzz:   privateDict.getInt(opBlueFuzz, 1),
		StdHW:      privateDict.getFloat(opStdHW, 0),
		StdVW:      privateDict.getFloat(opStdVW, 0),
		ForceBold:  privateDict.getInt(opForceBold, 0) != 0,
	}

	var subrs cffIndex
	subrsIndexOffs := privateDict.getInt(opSubrs, 0)
	if subrsIndexOffs > 0 {
		subrs, err = readIndexAt(p, pdOffs+subrsIndexOffs, "Subrs")
		if err != nil {
			return nil, err
		}
	}

	info := &privateInfo{
		private:      private,
		defaultWidth: privateDict.getInt(opDefaultWidthX, 0),
		nominalWidth: privateDict.getInt(opNominalWidthX, 0),
		fontMatrix:   d.getFontMatrix(opFontMatrix), // TODO(voss): default for CIDFonts?
		subrs:        subrs,
	}

	return info, nil
}

func (d cffDict) setDelta32(op dictOp, val []int32) {
	if len(val) == 0 {
		delete(d, op)
		return
	}
	res := make([]interface{}, len(val))
	var prev int32
	for i, x := range val {
		res[i] = x - prev
		prev = x
	}
	d[op] = res
}

func (d cffDict) setFontMatrix(op dictOp, fm []float64) {
	if len(fm) == 0 {
		return
	} else if len(fm) != 6 {
		panic("bad font matrix")
	}

	needed := false
	for i, xi := range fm {
		if math.Abs(xi-defaultFontMatrix[i]) > 1e-5 {
			needed = true
			break
		}
	}
	if !needed {
		return
	}

	val := make([]interface{}, 6)
	for i, xi := range fm {
		val[i] = xi
	}
	d[op] = val
}

func (d cffDict) sortedKeys() []dictOp {
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

func (d cffDict) encode(ss *cffStrings) []byte {
	keys := d.sortedKeys()

	res := &bytes.Buffer{}
	for _, op := range keys {
		args := d[op]
		for i, arg := range args {
			if s, ok := arg.(string); ok {
				args[i] = int32(ss.lookup(s))
			}
		}

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
			res.WriteByte(12)
		}
		res.WriteByte(byte(op))
	}
	return res.Bytes()
}

func makeTopDict(info *type1.FontInfo) cffDict {
	topDict := cffDict{}
	if info.Version != "" {
		topDict[opVersion] = []interface{}{info.Version}
	}
	if info.Notice != "" {
		topDict[opNotice] = []interface{}{info.Notice}
	}
	if info.Copyright != "" {
		topDict[opCopyright] = []interface{}{info.Copyright}
	}
	if info.FullName != "" {
		topDict[opFullName] = []interface{}{info.FullName}
	}
	if info.FamilyName != "" {
		topDict[opFamilyName] = []interface{}{info.FamilyName}
	}
	if info.Weight != "" {
		topDict[opWeight] = []interface{}{info.Weight}
	}
	if info.IsFixedPitch {
		topDict[opIsFixedPitch] = []interface{}{int32(1)}
	}
	if info.ItalicAngle != 0 {
		topDict[opItalicAngle] = []interface{}{info.ItalicAngle}
	}
	if info.UnderlinePosition != defaultUnderlinePosition {
		topDict[opUnderlinePosition] = []interface{}{info.UnderlinePosition}
	}
	if info.UnderlineThickness != defaultUnderlineThickness {
		topDict[opUnderlineThickness] = []interface{}{info.UnderlineThickness}
	}
	if info.PaintType != 0 {
		topDict[opPaintType] = []interface{}{info.PaintType}
	}
	topDict.setFontMatrix(opFontMatrix, info.FontMatrix)

	return topDict
}

func (cff *Font) makePrivateDict(defaultWidth, nominalWidth int32) cffDict {
	privateDict := cffDict{}

	if len(cff.Info.Private) != 1 {
		panic("not implemented")
	}
	private := cff.Info.Private[0]

	privateDict.setDelta32(opBlueValues, private.BlueValues)
	privateDict.setDelta32(opOtherBlues, private.OtherBlues)
	if math.Abs(private.BlueScale-defaultBlueScale) > 1e-6 {
		privateDict[opBlueScale] = []interface{}{private.BlueScale}
	}
	if private.BlueShift != defaultBlueShift {
		privateDict[opBlueShift] = []interface{}{private.BlueShift}
	}
	if private.BlueFuzz != defaultBlueFuzz {
		privateDict[opBlueFuzz] = []interface{}{private.BlueFuzz}
	}
	if private.StdHW != 0 {
		privateDict[opStdHW] = []interface{}{private.StdHW}
	}
	if private.StdVW != 0 {
		privateDict[opStdVW] = []interface{}{private.StdVW}
	}
	if private.ForceBold {
		privateDict[opForceBold] = []interface{}{int32(1)}
	}

	if defaultWidth != 0 {
		privateDict[opDefaultWidthX] = []interface{}{defaultWidth}
	}
	if nominalWidth != 0 {
		privateDict[opNominalWidthX] = []interface{}{nominalWidth}
	}

	return privateDict
}

var defaultFontMatrix = []float64{0.001, 0, 0, 0.001, 0, 0}

type dictOp uint16

func (d dictOp) String() string {
	switch d {
	case opVersion:
		return "Version"
	case opNotice:
		return "Notice"
	case opFullName:
		return "FullName"
	case opFamilyName:
		return "FamilyName"
	case opWeight:
		return "Weight"
	case opFontBBox:
		return "FontBBox"
	case opCharset:
		return "Charset"
	case opEncoding:
		return "Encoding"
	case opCharStrings:
		return "CharStrings"
	case opPrivate:
		return "Private"
	case opCopyright:
		return "Copyright"
	case opUnderlinePosition:
		return "UnderlinePosition"
	case opCharstringType:
		return "CharstringType"
	case opSyntheticBase:
		return "SyntheticBase"
	case opROS:
		return "ROS"
	case opCIDFontVersion:
		return "CIDFontVersion"
	case opCIDFontRevision:
		return "CIDFontRevision"
	case opCIDFontType:
		return "CIDFontType"
	case opUIDBase:
		return "UIDBase"
	case opFontName:
		return "FontName"
	case opCIDCount:
		return "CIDCount"
	case opFDArray:
		return "FDArray"
	case opFDSelect:
		return "FDSelect"

	case opBlueValues:
		return "BlueValues"
	case opOtherBlues:
		return "OtherBlues"
	case opFamilyBlues:
		return "FamilyBlues"
	case opFamilyOtherBlues:
		return "FamilyOtherBlues"
	case opStdHW:
		return "StdHW"
	case opStdVW:
		return "StdVW"
	case opSubrs:
		return "Subrs"
	case opDefaultWidthX:
		return "DefaultWidthX"
	case opNominalWidthX:
		return "NominalWidthX"
	case opBlueScale:
		return "BlueScale"
	case opBlueShift:
		return "BlueShift"
	case opBlueFuzz:
		return "BlueFuzz"
	case opForceBold:
		return "ForceBold"

	default:
		if d < 256 {
			return fmt.Sprintf("%d", d)
		}
		return fmt.Sprintf("%d %d", d>>8, d&0xff)
	}
}

const (
	// top DICT operators
	opVersion            dictOp = 0x0000
	opNotice             dictOp = 0x0001
	opFullName           dictOp = 0x0002
	opFamilyName         dictOp = 0x0003
	opWeight             dictOp = 0x0004
	opFontBBox           dictOp = 0x0005
	opCharset            dictOp = 0x000F
	opEncoding           dictOp = 0x0010
	opCharStrings        dictOp = 0x0011
	opPrivate            dictOp = 0x0012
	opCopyright          dictOp = 0x0C00
	opIsFixedPitch       dictOp = 0x0C01
	opItalicAngle        dictOp = 0x0C02
	opUnderlinePosition  dictOp = 0x0C03
	opUnderlineThickness dictOp = 0x0C04
	opPaintType          dictOp = 0x0C05
	opCharstringType     dictOp = 0x0C06
	opFontMatrix         dictOp = 0x0C07
	opSyntheticBase      dictOp = 0x0C14
	opPostScript         dictOp = 0x0C15
	opBaseFontName       dictOp = 0x0C16
	opROS                dictOp = 0x0C1E
	opCIDFontVersion     dictOp = 0x0C1F
	opCIDFontRevision    dictOp = 0x0C20
	opCIDFontType        dictOp = 0x0C21
	opCIDCount           dictOp = 0x0C22
	opUIDBase            dictOp = 0x0C23
	opFDArray            dictOp = 0x0C24
	opFDSelect           dictOp = 0x0C25
	opFontName           dictOp = 0x0C26

	// private DICT operators
	opBlueValues       dictOp = 0x0006
	opOtherBlues       dictOp = 0x0007
	opFamilyBlues      dictOp = 0x0008
	opFamilyOtherBlues dictOp = 0x0009
	opStdHW            dictOp = 0x000A
	opStdVW            dictOp = 0x000B
	opSubrs            dictOp = 0x0013 // Offset (self) to local subrs
	opDefaultWidthX    dictOp = 0x0014
	opNominalWidthX    dictOp = 0x0015
	opBlueScale        dictOp = 0x0C09
	opBlueShift        dictOp = 0x0C0A
	opBlueFuzz         dictOp = 0x0C0B
	opForceBold        dictOp = 0x0C0E

	// used in local unit tests only
	opDebug dictOp = 0x0CFF
)

func (d dictOp) isString() bool {
	switch d {
	case opVersion, opNotice, opCopyright, opFullName, opFamilyName, opWeight,
		opPostScript, opBaseFontName, opROS, opFontName:
		return true
	default:
		return false
	}
}

var errCorruptDict = errors.New("cff: invalid DICT")
