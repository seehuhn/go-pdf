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
	"io"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/type1"
)

// Font stores the data of a CFF font.
// Use the Read() function to decode a CFF font from a io.ReadSeeker.
type Font struct {
	Info   *type1.FontInfo
	Glyphs []*Glyph

	gid2cid []font.GlyphID
}

// Read reads a CFF font from r.
//
// TODO(voss): implement reading of CIDFonts.
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
	offSize := x & 0xFF // only used to exclude non-CFF files
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
	cff.Info = &type1.FontInfo{
		FontName: pdf.Name(fontNames[0]),
	}

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
	if topDict.getInt(opCharstringType, 2) != 2 {
		return nil, errors.New("cff: unsupported charstring type")
	}
	cff.Info.Version = topDict.getString(opVersion)
	cff.Info.Notice = topDict.getString(opNotice)
	cff.Info.Copyright = topDict.getString(opCopyright)
	cff.Info.FullName = topDict.getString(opFullName)
	cff.Info.FamilyName = topDict.getString(opFamilyName)
	cff.Info.Weight = topDict.getString(opWeight)
	isFixedPitch := topDict.getInt(opIsFixedPitch, 0)
	cff.Info.IsFixedPitch = isFixedPitch != 0
	cff.Info.ItalicAngle = topDict.getFloat(opItalicAngle, 0)
	cff.Info.UnderlinePosition = topDict.getFloat(opUnderlinePosition,
		defaultUnderlinePosition)
	cff.Info.UnderlineThickness = topDict.getFloat(opUnderlineThickness,
		defaultUnderlineThickness)
	cff.Info.PaintType = topDict.getInt(opPaintType, 0)

	// TODO(voss): different default for CIDFonts?
	cff.Info.FontMatrix = topDict.getFontMatrix(opFontMatrix)

	// read the Global Subr INDEX
	gsubrs, err := readIndex(p)
	if err != nil {
		return nil, err
	}

	// read the CharStrings INDEX
	charStringsOffs := topDict.getInt(opCharStrings, 0)
	charStrings, err := readIndexAt(p, charStringsOffs, "CharStrings")
	if err != nil {
		return nil, err
	}

	_, isCIDFont := topDict[opROS]
	if isCIDFont {
		fdArrayOffs := topDict.getInt(opFDArray, 0)
		fdArrayIndex, err := readIndexAt(p, fdArrayOffs, "Font DICT")
		if err != nil {
			return nil, err
		}
		for _, fdBlob := range fdArrayIndex {
			fontDict, err := decodeDict(fdBlob, strings)
			if err != nil {
				return nil, err
			}
			privateInfo, err := fontDict.readPrivate(p, strings)
			if err != nil {
				return nil, err
			}
			_ = privateInfo
		}

		return nil, errors.New("reading CIDfonts not implemented")
	}

	// read the list of glyph names
	charsetOffs := topDict.getInt(opCharset, 0)
	var charset []int32
	switch charsetOffs {
	case 0: // ISOAdobe
		// TODO(voss): implement
		return nil, errors.New("ISOAdobe charset not implemented")
	case 1: // Expert
		// TODO(voss): implement
		return nil, errors.New("Expert charset not implemented")
	case 2: // ExpertSubset
		// TODO(voss): implement
		return nil, errors.New("ExpertSubset charset not implemented")
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

	// encodingOffs, _ := topDict.getInt(opEncoding, 0)
	// if encodingOffs != 0 {
	// 	err = p.SeekPos(int64(encodingOffs))
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	_, err = cff.readEncoding(p)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// read the Private DICT
	private, err := topDict.readPrivate(p, strings)
	if err != nil {
		return nil, err
	}

	cff.Info.Private = []*type1.PrivateDict{private.private}

	cff.Glyphs = make([]*Glyph, len(charStrings))
	info := &decodeInfo{
		subr:         private.subrs,
		gsubr:        gsubrs,
		defaultWidth: private.defaultWidth,
		nominalWidth: private.nominalWidth,
	}
	for i, code := range charStrings {
		glyph, err := cff.doDecode(info, code)
		if err != nil {
			return nil, err
		}
		glyph.Name = strings.get(charset[i])
		cff.Glyphs[i] = glyph
	}

	return cff, nil
}

func (cff *Font) GlyphExtent() []font.Rect {
	numGlyphs := len(cff.Glyphs)
	extents := make([]font.Rect, numGlyphs)
	for i := 0; i < numGlyphs; i++ {
		extents[i] = cff.Glyphs[i].Extent()
	}
	return extents
}

func (cff *Font) selectWidths() (int32, int32) {
	numGlyphs := int32(len(cff.Glyphs))
	if numGlyphs == 0 {
		return 0, 0
	} else if numGlyphs == 1 {
		return cff.Glyphs[0].Width, cff.Glyphs[0].Width
	}

	widthHist := make(map[int32]int32)
	var mostFrequentCount int32
	var defaultWidth int32
	for _, glyph := range cff.Glyphs {
		w := glyph.Width
		widthHist[w]++
		if widthHist[w] > mostFrequentCount {
			defaultWidth = w
			mostFrequentCount = widthHist[w]
		}
	}

	// TODO(voss): the choice of nominalWidth can be improved
	var sum int32
	var minWidth int32 = math.MaxInt32
	var maxWidth int32
	for _, glyph := range cff.Glyphs {
		w := glyph.Width
		if w == defaultWidth {
			continue
		}
		sum += w
		if w < minWidth {
			minWidth = w
		}
		if w > maxWidth {
			maxWidth = w
		}
	}
	nominalWidth := (sum + numGlyphs/2) / (numGlyphs - 1)
	if nominalWidth < minWidth+107 {
		nominalWidth = minWidth + 107
	} else if nominalWidth > maxWidth-107 {
		nominalWidth = maxWidth - 107
	}
	return defaultWidth, nominalWidth
}

func (cff *Font) encodeCharStrings() (cffIndex, int32, int32) {
	numGlyphs := len(cff.Glyphs)

	// TODO(voss): introduce subroutines

	cc := make(cffIndex, numGlyphs)
	defaultWidth, nominalWidth := cff.selectWidths()
	for i, glyph := range cff.Glyphs {
		w := glyph.Width
		var wEnc []byte
		if w != defaultWidth {
			wEnc = encodeInt(int16(w - nominalWidth))
		}

		cmds := encodeArgs(glyph.Cmds)
		data := encodeCommands(cmds)

		k := len(wEnc)
		for _, b := range data {
			k += len(b)
		}
		code := make([]byte, 0, k)
		code = append(code, wEnc...)
		for _, b := range data {
			code = append(code, b...)
		}
		cc[i] = code
	}

	return cc, defaultWidth, nominalWidth
}

// Encode writes the binary form of a CFF font as a simple font.
func (cff *Font) Encode(w io.Writer) error {
	numGlyphs := uint16(len(cff.Glyphs))

	charStrings, defWidth, nomWidth := cff.encodeCharStrings()

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
	blobs[secNameIndex], err = cffIndex{[]byte(cff.Info.FontName)}.encode()
	if err != nil {
		return err
	}

	// section 2: top dict INDEX
	topDict := makeTopDict(cff.Info)
	// opCharset is updated below
	// opEncoding???
	// opCharStrings is updated below
	// opPrivate is updated below

	// section 3: secStringIndex
	// The new string index is stored in `newStrings`.
	// We encode the blob below, once all strings are known.

	// section 4: global subr INDEX
	gsubrs := cffIndex{}
	blobs[secGsubrsIndex], err = gsubrs.encode()
	if err != nil {
		return err
	}

	// section 5: encodings
	numEncoded := numGlyphs - 1 // leave out .notdef
	if numEncoded >= 256 {
		numEncoded = 256
	}
	blobs[secEncodings] = []byte{1, 1, 0, byte(numEncoded - 1)}

	// section 6: charsets INDEX
	subset := make([]int32, numGlyphs)
	for i := uint16(0); i < numGlyphs; i++ {
		s := cff.Glyphs[i].Name
		if s == "" {
			s = ".notdef"
		}
		subset[i] = newStrings.lookup(s)
	}
	blobs[secCharsets], err = encodeCharset(subset)
	if err != nil {
		return err
	}

	// section 7: charstrings INDEX
	blobs[secCharStringsIndex], err = cffIndex(charStrings).encode()
	if err != nil {
		return err
	}

	// section 8: private DICT
	privateDict := cff.makePrivateDict(defWidth, nomWidth)
	// opSubrs is set below

	// section 9: subrs INDEX
	subrs := cffIndex{}
	blobs[secSubrsIndex], err = subrs.encode()
	if err != nil {
		return err
	}

	cumsum := func() []int32 {
		res := make([]int32, numSections+1)
		for i := 0; i < numSections; i++ {
			res[i+1] = res[i] + int32(len(blobs[i]))
		}
		return res
	}

	offs := cumsum()
	for {
		// This loop terminates because the elements of offs are monotonically
		// increasing.

		blobs[secHeader][3] = offsSize(offs[numSections])

		privateDict[opSubrs] = []interface{}{offs[secSubrsIndex] - offs[secPrivateDict]}
		blobs[secPrivateDict] = privateDict.encode(newStrings)
		pdSize := len(blobs[secPrivateDict])
		pdDesc := []interface{}{int32(pdSize), offs[secPrivateDict]}

		topDict[opCharset] = []interface{}{offs[secCharsets]}
		topDict[opEncoding] = []interface{}{offs[secEncodings]}
		topDict[opCharStrings] = []interface{}{offs[secCharStringsIndex]}
		topDict[opPrivate] = pdDesc
		topDictData := topDict.encode(newStrings)
		blobs[secTopDictIndex], err = cffIndex{topDictData}.encode()
		if err != nil {
			return err
		}

		blobs[secStringIndex], err = newStrings.encode()
		if err != nil {
			return err
		}

		newOffs := cumsum()
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

	for i := 0; i < numSections; i++ {
		_, err = w.Write(blobs[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// EncodeCID returns the binary encoding of a CFF font as a CIDFont.
func (cff *Font) EncodeCID(w io.Writer, registry, ordering string, supplement int) error {
	numGlyphs := int32(len(cff.Glyphs))

	charStrings, defWidth, nomWidth := cff.encodeCharStrings()

	fontMatrix := cff.Info.FontMatrix

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
	blobs[cidNameIndex], err = cffIndex{[]byte(cff.Info.FontName)}.encode()
	if err != nil {
		return err
	}

	// section 2: top dict INDEX
	// afdko/c/shared/source/cffwrite/cffwrite_dict.c:cfwDictFillTop
	topDict := makeTopDict(cff.Info)
	delete(topDict, opPaintType)  // per font
	delete(topDict, opFontMatrix) // per font
	// opCharset is updated below
	delete(topDict, opEncoding)
	// opCharStrings is updated below
	registrySID := newStrings.lookup(registry)
	orderingSID := newStrings.lookup(ordering)
	topDict[opROS] = []interface{}{
		int32(registrySID), int32(orderingSID), int32(supplement),
	}
	topDict[opCIDCount] = []interface{}{int32(numGlyphs)}
	// opFDArray is updated below
	// opFDSelect is updated below

	// section 3: secStringIndex
	// The new string index is stored in `newStrings`.
	// We encode the blob below, once all strings are known.

	// section 4: global subr INDEX
	gsubrs := cffIndex{}
	blobs[cidGsubrsIndex], err = gsubrs.encode()
	if err != nil {
		return err
	}

	// section 5: charsets INDEX (represents CIDs instead of glyph names)
	subset := make([]int32, numGlyphs)
	if cff.gid2cid != nil {
		for gid, cid := range cff.gid2cid {
			subset[gid] = int32(cid)
		}
	} else {
		for i := int32(0); i < numGlyphs; i++ {
			subset[i] = i
		}
	}
	blobs[cidCharsets], err = encodeCharset(subset)
	if err != nil {
		return err
	}

	// section 6: FDSelect
	blobs[cidFdSelect] = []byte{
		3,    // format
		0, 1, // nRanges

		0, 0, // first = first glyph index in range
		0, // font DICT 0

		byte(numGlyphs >> 8), byte(numGlyphs), // sentinel
	}

	// section 7: charstrings INDEX
	blobs[cidCharStringsIndex], err = cffIndex(charStrings).encode()
	if err != nil {
		return err
	}

	// section 8: font DICT INDEX
	// (see afdko/c/shared/source/cffwrite/cffwrite_dict.c:cfwDictFillFont)
	fontDict := cffDict{}
	fontDict.setFontMatrix(opFontMatrix, fontMatrix)
	// maybe also needs the following field:
	//   - PaintType
	// opPrivate is set below

	// section 9: private DICT
	privateDict := cff.makePrivateDict(defWidth, nomWidth)
	// opSubrs is set below

	// section 10: subrs INDEX
	subrs := cffIndex{}
	blobs[cidSubrsIndex], err = subrs.encode()
	if err != nil {
		return err
	}

	cumsum := func() []int32 {
		res := make([]int32, cidNumSections+1)
		for i := 0; i < cidNumSections; i++ {
			res[i+1] = res[i] + int32(len(blobs[i]))
		}
		return res
	}

	offs := cumsum()
	for {
		// This loop terminates because the elements of offs are monotonically
		// increasing.

		blobs[secHeader][3] = offsSize(offs[numSections])

		privateDict[opSubrs] = []interface{}{offs[cidSubrsIndex] - offs[cidPrivateDict]}
		blobs[cidPrivateDict] = privateDict.encode(newStrings)
		pdSize := len(blobs[cidPrivateDict])
		pdDesc := []interface{}{int32(pdSize), offs[cidPrivateDict]}

		fontDict[opPrivate] = pdDesc
		fontDictData := fontDict.encode(newStrings)
		blobs[cidFDArray], err = cffIndex{fontDictData}.encode()
		if err != nil {
			return err
		}

		topDict[opCharset] = []interface{}{offs[cidCharsets]}
		topDict[opCharStrings] = []interface{}{offs[cidCharStringsIndex]}
		topDict[opFDArray] = []interface{}{offs[cidFDArray]}
		topDict[opFDSelect] = []interface{}{offs[cidFdSelect]}
		topDictData := topDict.encode(newStrings)
		blobs[cidTopDictIndex], err = cffIndex{topDictData}.encode()
		if err != nil {
			return err
		}

		blobs[cidStringIndex], err = newStrings.encode()
		if err != nil {
			return err
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

	for i := 0; i < cidNumSections; i++ {
		_, err = w.Write(blobs[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// these are the sections of a simple CFF file, in the order they appear in
// in the file.
const (
	secHeader int = iota
	secNameIndex
	secTopDictIndex
	secStringIndex
	secGsubrsIndex
	secEncodings
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

const (
	defaultUnderlinePosition  = -100
	defaultUnderlineThickness = 50
	defaultBlueScale          = 0.039625
	defaultBlueShift          = 7
	defaultBlueFuzz           = 1
)

var (
	errNoNotdef = errors.New("cff: missing .notdef glyph")
)
