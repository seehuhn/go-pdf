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
	"io"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// Font is a CFF font.
type Font struct {
	FontName string

	topDict     cffDict
	gsubrs      cffIndex
	charStrings cffIndex
	glyphNames  []string
	privateDict cffDict
	subrs       cffIndex

	gid2cid []font.GlyphID

	// TODO(voss): store CIDFont information
}

// Read reads a CFF font from r.
func Read(r io.ReadSeeker) (*Font, error) {
	cff := &Font{}

	length, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	p := parser.New(r)
	err = p.SetRegion("CFF", 0, length)
	if err != nil {
		return nil, err
	}
	x, err := p.ReadUInt32()
	if err != nil {
		return nil, err
	}
	major := x >> 24
	minor := (x >> 16) & 0xFF
	nameIndexOffs := int64((x >> 8) & 0xFF)
	offSize := x & 0xFF // unused
	if major == 2 {
		return nil, fmt.Errorf("unsupported CFF version %d.%d", major, minor)
	} else if major != 1 || nameIndexOffs < 4 || offSize > 4 {
		return nil, errors.New("not a CFF font")
	}

	// read the Name INDEX
	err = p.SeekPos(nameIndexOffs)
	if err != nil {
		return nil, err
	}
	fontNames, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(fontNames) != 1 {
		return nil, errors.New("CFF with multiple fonts not supported")
	}
	cff.FontName = string(fontNames[0])

	// read the Top DICT
	topDictIndex, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(topDictIndex) != 1 {
		return nil, errors.New("invalid CFF")
	}

	// read the String INDEX
	stringIndex, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	strings := &cffStrings{
		data: make([]string, len(stringIndex)),
	}
	for i, s := range stringIndex {
		strings.data[i] = string(s)
	}

	topDict, err := decodeDict(topDictIndex[0], strings)
	if err != nil {
		return nil, err
	}
	cff.topDict = topDict

	// read the Global Subr INDEX
	gsubrs, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	cff.gsubrs = gsubrs

	_, isCIDFont := topDict[opROS]
	if isCIDFont {
		return nil, errors.New("CIDfont support not implemented")
	}

	// read the CharStrings INDEX
	cct, _ := topDict.getInt(opCharstringType, 2)
	if cct != 2 {
		return nil, errors.New("unsupported charstring type")
	}
	charStringsOffs, ok := topDict.getInt(opCharStrings, 0)
	if !ok {
		return nil, errors.New("missing CharStrings offset")
	}
	delete(topDict, opCharStrings)
	err = p.SeekPos(int64(charStringsOffs))
	if err != nil {
		return nil, err
	}
	charStrings, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	cff.charStrings = charStrings

	// read the list of glyph names
	charsetOffs, _ := topDict.getInt(opCharset, 0)
	delete(topDict, opCharset)
	var charset []int32
	switch charsetOffs {
	case 0: // ISOAdobe
		panic("not implemented")
	case 1: // Expert
		panic("not implemented")
	case 2: // ExpertSubset
		panic("not implemented")
	default:
		err = p.SeekPos(int64(charsetOffs))
		if err != nil {
			return nil, err
		}
		charset, err = readCharset(p, len(charStrings))
		if err != nil {
			return nil, err
		}
	}
	cff.glyphNames = make([]string, len(charset))
	for i, sid := range charset {
		cff.glyphNames[i] = strings.get(sid)
	}

	// read the Private DICT
	pdSize, pdOffs, ok := topDict.getPair(opPrivate)
	if !ok {
		return nil, errors.New("missing Private DICT")
	}
	delete(topDict, opPrivate)
	err = p.SeekPos(int64(pdOffs))
	if err != nil {
		return nil, err
	}
	privateDictBlob := make([]byte, pdSize)
	_, err = p.Read(privateDictBlob)
	if err != nil {
		return nil, err
	}
	cff.privateDict, err = decodeDict(privateDictBlob, strings)
	if err != nil {
		return nil, err
	}

	subrsIndexOffs, _ := cff.privateDict.getInt(opSubrs, 0)
	delete(cff.privateDict, opSubrs)
	if subrsIndexOffs > 0 {
		err = p.SeekPos(int64(pdOffs) + int64(subrsIndexOffs))
		if err != nil {
			return nil, err
		}
		subrs, err := readIndex(p)
		if err != nil {
			return nil, err
		}
		cff.subrs = subrs
	}

	return cff, nil
}

func readCharset(p *parser.Parser, nGlyphs int) ([]int32, error) {
	format, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	charset := make([]int32, 0, nGlyphs)
	charset = append(charset, 0)
	switch format {
	case 0:
		s := &parser.State{
			A: int64(nGlyphs - 1),
		}
		err = p.Exec(s,
			parser.CmdLoop,
			parser.CmdStash,
			parser.CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}

		data := s.GetStash()
		for _, xi := range data {
			charset = append(charset, int32(xi))
		}
	case 1:
		for len(charset) < nGlyphs {
			first, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			nLeft, err := p.ReadUInt8()
			if err != nil {
				return nil, err
			}
			for i := 0; i < int(nLeft)+1; i++ {
				charset = append(charset, int32(int(first)+i))
			}
		}
	case 2:
		for len(charset) < nGlyphs {
			first, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			nLeft, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			for i := 0; i < int(nLeft)+1; i++ {
				charset = append(charset, int32(int(first)+i))
			}
		}
	default:
		return nil, fmt.Errorf("unsupported charset format %d", format)
	}

	return charset, nil
}

func encodeCharset(names []int32) ([]byte, error) {
	if names[0] != 0 {
		return nil, errors.New("invalid charset")
	}
	names = names[1:]

	// find runs of consecutive glyph names
	var runs []int
	for i := 0; i < len(names); i++ {
		if i == 0 || names[i] != names[i-1]+1 {
			runs = append(runs, i)
		}
	}
	runs = append(runs, len(names))

	length0 := 1 + 2*len(names) // length with format 0 encoding

	length1 := 1 + 3*(len(runs)-1) // length with format 1 encoding
	for i := 0; i < len(runs)-1; i++ {
		d := runs[i+1] - runs[i]
		for d > 256 {
			length1 += 3
			d -= 256
		}
	}

	length2 := 1 + 4*(len(runs)-1) // length with format 2 encoding

	var buf []byte
	if length0 <= length1 && length0 <= length2 {
		buf = make([]byte, length0)
		buf[0] = 0
		for i, name := range names {
			buf[2*i+1] = byte(name >> 8)
			buf[2*i+2] = byte(name)
		}
	} else if length1 < length2 {
		buf = make([]byte, length1)
		buf[0] = 1
		for i := 0; i < len(runs)-1; i++ {
			name := names[runs[i]]
			dd := runs[i+1] - runs[i]
			for dd > 0 {
				d := dd - 1
				if d > 255 {
					d = 255
				}
				buf[3*i+1] = byte(name >> 8)
				buf[3*i+2] = byte(name)
				buf[3*i+3] = byte(d)
				name += int32(d + 1)
				dd -= d + 1
			}
		}
	} else {
		buf = make([]byte, length2)
		buf[0] = 2
		for i := 0; i < len(runs)-1; i++ {
			name := names[runs[i]]
			d := runs[i+1] - runs[i] - 1
			buf[4*i+1] = byte(name >> 8)
			buf[4*i+2] = byte(name)
			buf[4*i+3] = byte(d >> 8)
			buf[4*i+4] = byte(d)
		}
	}
	return buf, nil
}

// Encode returns the binary encoding of a CFF font.
func (cff *Font) Encode() ([]byte, error) {
	numGlyphs := uint16(len(cff.charStrings))

	blobs := make([][]byte, numSections)
	newStrings := &cffStrings{}

	// section 0: Header
	blobs[secHeader] = []byte{
		1, // major
		0, // minor
		4, // hdrSize
		4, // offSize
	}

	// section 1: Name INDEX
	var err error
	blobs[secNameIndex], err = cffIndex{[]byte(cff.FontName)}.encode()
	if err != nil {
		return nil, err
	}

	// section 2: top dict INDEX
	tdCopy := cff.topDict.Copy()
	// opCharset is updated below
	delete(tdCopy, opEncoding)
	// opCharStrings is updated below
	// opPrivate is updated below

	// section 3: secStringIndex
	// The new string index is stored in `newStrings`.
	// We encode the blob below, once all strings are known.

	// section 4: global subr INDEX
	blobs[secGsubrsIndex], err = cffIndex(cff.gsubrs).encode()
	if err != nil {
		return nil, err
	}

	// section 5: charsets INDEX
	subset := make([]int32, numGlyphs)
	for i := uint16(0); i < numGlyphs; i++ {
		s := cff.glyphNames[i]
		if s == "" {
			s = ".notdef"
		}
		subset[i] = newStrings.lookup(s)
	}
	blobs[secCharsets], err = encodeCharset(subset)
	if err != nil {
		return nil, err
	}

	// section 6: charstrings INDEX
	blobs[secCharStringsIndex], err = cffIndex(cff.charStrings).encode()
	if err != nil {
		return nil, err
	}

	// section 7: private DICT
	pdCopy := cff.privateDict.Copy()
	// opSubrs is set below

	// section 8: subrs INDEX
	blobs[secSubrsIndex], err = cff.subrs.encode()
	if err != nil {
		return nil, err
	}

	cumsum := func() []int32 {
		res := make([]int32, numSections+1)
		for i := 0; i < numSections; i++ {
			res[i+1] = res[i] + int32(len(blobs[i]))
		}
		return res
	}

	offs := cumsum()
	for { // TODO(voss): does this loop always terminate?
		blobs[secHeader][3] = offsSize(offs[numSections])

		pdCopy[opSubrs] = []interface{}{offs[secSubrsIndex] - offs[secPrivateDict]}
		blobs[secPrivateDict] = pdCopy.encode(newStrings)
		pdSize := len(blobs[secPrivateDict])
		pdDesc := []interface{}{int32(pdSize), offs[secPrivateDict]}

		tdCopy[opCharset] = []interface{}{offs[secCharsets]}
		tdCopy[opCharStrings] = []interface{}{offs[secCharStringsIndex]}
		tdCopy[opPrivate] = pdDesc
		topDictData := tdCopy.encode(newStrings)
		blobs[secTopDictIndex], err = cffIndex{topDictData}.encode()
		if err != nil {
			return nil, err
		}

		blobs[secStringIndex], err = newStrings.encode()
		if err != nil {
			return nil, err
		}

		newOffs := cumsum()
		fmt.Println(newOffs)
		done := true
		for i := 0; i < numSections; i++ {
			if newOffs[i] != offs[i] {
				done = false
				break
			}
		}
		if done {
			break
		}

		offs = newOffs
	}

	res := &bytes.Buffer{}
	for i := 0; i < numSections; i++ {
		res.Write(blobs[i])
	}

	return res.Bytes(), nil
}

// EncodeCID returns the binary encoding of a CFF font as a CIDFont.
// TODO(voss): this only works if the original font is not CID-keyed
func (cff *Font) EncodeCID(registry, ordering string, supplement int) ([]byte, error) {
	numGlyphs := int32(len(cff.charStrings))

	fontMatrix := getFontMatrix(cff.topDict)

	blobs := make([][]byte, cidNumSections)
	newStrings := &cffStrings{}

	// section 0: Header
	blobs[cidHeader] = []byte{
		1, // major
		0, // minor
		4, // hdrSize
		4, // offSize
	}

	// section 1: Name INDEX
	var err error
	blobs[cidNameIndex], err = cffIndex{[]byte(cff.FontName)}.encode()
	if err != nil {
		return nil, err
	}

	// section 2: top dict INDEX
	// afdko/c/shared/source/cffwrite/cffwrite_dict.c:cfwDictFillTop
	tdCopy := cff.topDict.Copy()
	delete(tdCopy, opPaintType)  // per font
	delete(tdCopy, opFontMatrix) // per font
	// opCharset is updated below
	delete(tdCopy, opEncoding)
	// opCharStrings is updated below
	delete(tdCopy, opPrivate) // per font
	registrySID := newStrings.lookup(registry)
	orderingSID := newStrings.lookup(ordering)
	tdCopy[opROS] = []interface{}{
		int32(registrySID), int32(orderingSID), int32(supplement),
	}
	tdCopy[opCIDCount] = []interface{}{int32(numGlyphs)}
	// opFDArray is updated below
	// opFDSelect is updated below
	delete(tdCopy, opFontName) // per font

	// section 3: secStringIndex
	// The new string index is stored in `newStrings`.
	// We encode the blob below, once all strings are known.

	// section 4: global subr INDEX
	blobs[cidGsubrsIndex], err = cffIndex(cff.gsubrs).encode()
	if err != nil {
		return nil, err
	}

	// section 5: charsets INDEX
	subset := make([]int32, numGlyphs)
	for i := int32(0); i < numGlyphs; i++ {
		subset[i] = i
	}
	blobs[cidCharsets], err = encodeCharset(subset)
	if err != nil {
		return nil, err
	}

	// section 6: FDSelect
	fdSelect := &bytes.Buffer{}
	fdSelect.Write([]byte{
		3,    // format
		0, 1, // nRanges

		0, 0, // first = first glyph index in range
		0, // font DICT 0

		byte(numGlyphs >> 8), byte(numGlyphs), // sentinel
	})
	blobs[cidFdSelect] = fdSelect.Bytes()

	// section 7: charstrings INDEX
	blobs[cidCharStringsIndex], err = cffIndex(cff.charStrings).encode()
	if err != nil {
		return nil, err
	}

	// section 8: font DICT INDEX
	// (see afdko/c/shared/source/cffwrite/cffwrite_dict.c:cfwDictFillFont)
	fontDict := cffDict{}
	setFontMatrix(fontDict, fontMatrix)
	fontDict[opFontName] = []interface{}{int32(newStrings.lookup(cff.FontName))}
	// maybe also needs the following field:
	//   - PaintType
	// opPrivate is set below

	// section 9: private DICT
	pdCopy := cff.privateDict.Copy()
	// opSubrs is set below

	// section 10: subrs INDEX
	blobs[cidSubrsIndex], err = cff.subrs.encode()
	if err != nil {
		return nil, err
	}

	cumsum := func() []int32 {
		res := make([]int32, cidNumSections+1)
		for i := 0; i < cidNumSections; i++ {
			res[i+1] = res[i] + int32(len(blobs[i]))
		}
		return res
	}

	offs := cumsum()
	for { // TODO(voss): does this loop always terminate?
		blobs[secHeader][3] = offsSize(offs[numSections])

		pdCopy[opSubrs] = []interface{}{offs[cidSubrsIndex] - offs[cidPrivateDict]}
		blobs[cidPrivateDict] = pdCopy.encode(newStrings)
		pdSize := len(blobs[cidPrivateDict])
		pdDesc := []interface{}{int32(pdSize), offs[cidPrivateDict]}

		fontDict[opPrivate] = pdDesc
		fontDictData := fontDict.encode(newStrings)
		blobs[cidFDArray], err = cffIndex{fontDictData}.encode()
		if err != nil {
			return nil, err
		}

		tdCopy[opCharset] = []interface{}{offs[cidCharsets]}
		tdCopy[opCharStrings] = []interface{}{offs[cidCharStringsIndex]}
		tdCopy[opFDArray] = []interface{}{offs[cidFDArray]}
		tdCopy[opFDSelect] = []interface{}{offs[cidFdSelect]}
		topDictData := tdCopy.encode(newStrings)
		blobs[cidTopDictIndex], err = cffIndex{topDictData}.encode()
		if err != nil {
			return nil, err
		}

		blobs[cidStringIndex], err = newStrings.encode()
		if err != nil {
			return nil, err
		}

		newOffs := cumsum()
		done := true
		for i := 0; i < cidNumSections; i++ {
			if newOffs[i] != offs[i] {
				done = false
				break
			}
		}
		if done {
			break
		}
		offs = newOffs
	}

	res := &bytes.Buffer{}
	for i := 0; i < cidNumSections; i++ {
		res.Write(blobs[i])
	}

	return res.Bytes(), nil
}

// these are the sections of a CIDKeyed CFF file, in the order they appear in
// in the file.
const (
	secHeader int = iota
	secNameIndex
	secTopDictIndex
	secStringIndex
	secGsubrsIndex
	secCharsets
	secCharStringsIndex
	secPrivateDict
	secSubrsIndex

	numSections
)

// these are the sections of a CIDKeyed CFF file, in the order they appear in
// in the file.
const (
	cidHeader int = iota
	cidNameIndex
	cidTopDictIndex
	cidStringIndex
	cidGsubrsIndex
	cidCharsets
	cidFdSelect
	cidCharStringsIndex
	cidFDArray
	cidPrivateDict
	cidSubrsIndex

	cidNumSections
)

// TODO(voss): inline this?
func offsSize(i int32) byte {
	switch {
	case i < 1<<8:
		return 1
	case i < 1<<16:
		return 2
	case i < 1<<24:
		return 3
	default:
		return 4
	}
}
