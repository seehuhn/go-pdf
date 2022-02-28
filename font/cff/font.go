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
	"fmt"
	"io"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/type1"
)

// TODO(voss): implement support for font matrices

// Font stores the data of a CFF font.
// Use the Read() function to decode a CFF font from a io.ReadSeeker.
type Font struct {
	Info     *type1.FontInfo
	Glyphs   []*Glyph
	FdSelect FdSelectFn

	IsCIDFont bool

	Encoding []font.GlyphID

	Gid2cid    []int32 // TODO(voss): what is a good data type for this?
	Registry   string
	Ordering   string
	Supplement int32
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

	// section 0: header
	x, err := p.ReadUInt32()
	if err != nil {
		return nil, err
	}
	major := x >> 24
	minor := (x >> 16) & 0xFF
	nameIndexOffs := int64((x >> 8) & 0xFF)
	offSize := x & 0xFF // only used to exclude non-CFF files
	if major == 2 {
		return nil, notSupported(fmt.Sprintf("version %d.%d", major, minor))
	} else if major != 1 || nameIndexOffs < 4 || offSize > 4 {
		return nil, invalidSince("invalid header")
	}

	// section 1: Name INDEX
	err = p.SeekPos(nameIndexOffs)
	if err != nil {
		return nil, err
	}
	fontNames, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(fontNames) == 0 {
		return nil, invalidSince("no font data")
	} else if len(fontNames) > 1 {
		return nil, notSupported("fontsets with more than one font")
	}
	cff.Info = &type1.FontInfo{
		FontName: pdf.Name(fontNames[0]),
	}

	// section 2: top DICT INDEX
	topDictIndex, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(topDictIndex) != len(fontNames) {
		return nil, invalidSince("wrong number of top dicts")
	}

	// section 3: String INDEX
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

	// interlude: decode the top DICT
	topDict, err := decodeDict(topDictIndex[0], strings)
	if err != nil {
		return nil, err
	}
	if topDict.getInt(opCharstringType, 2) != 2 {
		return nil, notSupported("charstring type != 2")
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
	cff.Info.UnderlinePosition = int16(topDict.getInt(opUnderlinePosition,
		defaultUnderlinePosition))
	cff.Info.UnderlineThickness = int16(topDict.getInt(opUnderlineThickness,
		defaultUnderlineThickness))

	// TODO(voss): different default for CIDFonts?
	cff.Info.FontMatrix = topDict.getFontMatrix(opFontMatrix)

	// section 4: global subr INDEX
	gsubrs, err := readIndex(p)
	if err != nil {
		return nil, err
	}

	// section 5: encodings
	// read below, once we know the charset

	// read the CharStrings INDEX
	charStringsOffs := topDict.getInt(opCharStrings, 0)
	charStrings, err := readIndexAt(p, charStringsOffs, "CharStrings")
	if err != nil {
		return nil, err
	} else if len(charStrings) == 0 {
		return nil, invalidSince("no charstrings")
	}

	ROS, isCIDFont := topDict[opROS]
	cff.IsCIDFont = isCIDFont
	var decoders []*decodeInfo
	if isCIDFont {
		if len(ROS) != 3 {
			return nil, invalidSince("wrong number of ROS values")
		}
		if reg, ok := ROS[0].(string); ok {
			cff.Registry = reg
		} else {
			return nil, invalidSince("wrong type for Registry")
		}
		if ord, ok := ROS[1].(string); ok {
			cff.Ordering = ord
		} else {
			return nil, invalidSince("wrong type for Ordering")
		}
		if sup, ok := ROS[2].(int32); ok {
			cff.Supplement = sup
		} else {
			return nil, invalidSince("wrong type for Supplement")
		}

		fdArrayOffs := topDict.getInt(opFDArray, 0)
		fdArrayIndex, err := readIndexAt(p, fdArrayOffs, "Font DICT")
		if err != nil {
			return nil, err
		} else if len(fdArrayIndex) > 256 {
			return nil, invalidSince("too many Font DICTs")
		} else if len(fdArrayIndex) == 0 {
			return nil, invalidSince("no Font DICTs")
		}
		for _, fdBlob := range fdArrayIndex {
			fontDict, err := decodeDict(fdBlob, strings)
			if err != nil {
				return nil, err
			}
			pInfo, err := fontDict.readPrivate(p, strings)
			if err != nil {
				return nil, err
			}
			cff.Info.Private = append(cff.Info.Private, pInfo.private)
			decoders = append(decoders, &decodeInfo{
				subr:         pInfo.subrs,
				gsubr:        gsubrs,
				defaultWidth: pInfo.defaultWidth,
				nominalWidth: pInfo.nominalWidth,
			})
		}

		fdSelectOffs := topDict.getInt(opFDSelect, 0)
		if fdSelectOffs < 4 {
			return nil, invalidSince("missing FDSelect")
		}
		err = p.SeekPos(int64(fdSelectOffs))
		if err != nil {
			return nil, err
		}
		cff.FdSelect, err = readFDSelect(p, len(charStrings), len(cff.Info.Private))
		if err != nil {
			return nil, err
		}
	}

	// read the list of glyph names
	charsetOffs := topDict.getInt(opCharset, 0)
	var charset []int32
	if cff.IsCIDFont {
		err = p.SeekPos(int64(charsetOffs))
		if err != nil {
			return nil, err
		}
		charset, err = readCharset(p, len(charStrings))
		if err != nil {
			return nil, err
		}
		cff.Gid2cid = make([]int32, len(charStrings)) // filled in below
	} else {
		switch charsetOffs {
		case 0: // ISOAdobe charset
			charset = make([]int32, len(charStrings))
			for i := range charset {
				charset[i] = strings.lookup(isoAdobeCharset[i])
			}
		case 1: // Expert charset
			charset = make([]int32, len(charStrings))
			for i := range charset {
				charset[i] = strings.lookup(expertCharset[i])
			}
		case 2: // ExpertSubset charset
			charset = make([]int32, len(charStrings))
			for i := range charset {
				charset[i] = strings.lookup(expertSubsetCharset[i])
			}
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
	}

	// read the Private DICT
	if !cff.IsCIDFont {
		pInfo, err := topDict.readPrivate(p, strings)
		if err != nil {
			return nil, err
		}
		cff.Info.Private = []*type1.PrivateDict{pInfo.private}
		decoders = append(decoders, &decodeInfo{
			subr:         pInfo.subrs,
			gsubr:        gsubrs,
			defaultWidth: pInfo.defaultWidth,
			nominalWidth: pInfo.nominalWidth,
		})
	}

	cff.Glyphs = make([]*Glyph, len(charStrings))
	fdSelect := cff.FdSelect
	if fdSelect == nil {
		fdSelect = func(gid font.GlyphID) int { return 0 }
	}
	for gid, code := range charStrings {
		fdIdx := fdSelect(font.GlyphID(gid))
		info := decoders[fdIdx]

		glyph, err := decodeCharString(info, code)
		if err != nil {
			return nil, err
		}
		if cff.IsCIDFont {
			if charset != nil {
				cff.Gid2cid[gid] = charset[gid]
			}
		} else {
			name, err := strings.get(charset[gid])
			if err != nil {
				return nil, err
			}
			glyph.Name = pdf.Name(name)
		}
		cff.Glyphs[gid] = glyph
	}

	// read the encoding
	if !cff.IsCIDFont {
		encodingOffs := topDict.getInt(opEncoding, 0)
		var enc []font.GlyphID
		switch {
		case encodingOffs == 0:
			enc = standardEncoding(cff.Glyphs)
		case encodingOffs == 1:
			enc = expertEncoding(cff.Glyphs)
		case encodingOffs >= 4:
			err = p.SeekPos(int64(encodingOffs))
			if err != nil {
				return nil, err
			}
			enc, err = readEncoding(p, charset)
			if err != nil {
				return nil, err
			}
		}
		cff.Encoding = enc
	}

	return cff, nil
}

// GlyphExtents returns the boundig boxes of all glyphs.
func (cff *Font) GlyphExtents() []font.Rect {
	numGlyphs := len(cff.Glyphs)
	extents := make([]font.Rect, numGlyphs)
	for i := 0; i < numGlyphs; i++ {
		extents[i] = cff.Glyphs[i].Extent()
	}
	return extents
}

// Encode writes the binary form of a CFF font.
func (cff *Font) Encode(w io.Writer) error {
	numGlyphs := uint16(len(cff.Glyphs))

	// TODO(voss): this should be done per private dict.
	charStrings, defWidth, nomWidth, err := cff.encodeCharStrings()
	if err != nil {
		return err
	}

	var blobs [][]byte
	strings := &cffStrings{}

	// section 0: Header
	secHeader := len(blobs)
	blobs = append(blobs, []byte{
		1, // major
		0, // minor
		4, // hdrSize
		4, // offSize (updated below)
	})

	// section 1: Name INDEX
	blobs = append(blobs, cffIndex{[]byte(cff.Info.FontName)}.encode())

	// section 2: top dict INDEX
	topDict := makeTopDict(cff.Info)
	// opCharset is updated below
	// opCharStrings is updated below
	if cff.IsCIDFont {
		// afdko/c/shared/source/cffwrite/cffwrite_dict.c:cfwDictFillTop
		registrySID := strings.lookup(cff.Registry)
		orderingSID := strings.lookup(cff.Ordering)
		topDict[opROS] = []interface{}{
			registrySID, orderingSID, cff.Supplement,
		}
		topDict[opCIDCount] = []interface{}{int32(numGlyphs)}
		// opFDArray is updated below
		// opFDSelect is updated below
	} else {
		topDict.setFontMatrix(opFontMatrix, cff.Info.FontMatrix)
		// opEncoding is updated below
		// opPrivate is updated below
	}
	secTopDictIndex := len(blobs)
	blobs = append(blobs, nil)

	// section 3: string INDEX
	// The new string index is stored in `strings`.
	// We encode the blob below, once all strings are known.
	secStringIndex := len(blobs)
	blobs = append(blobs, nil)

	// section 4: global subr INDEX
	blobs = append(blobs, cffIndex{}.encode())

	// section 5: encodings
	secEncodings := -1
	var glyphNames []int32
	if !cff.IsCIDFont {
		glyphNames = make([]int32, numGlyphs)
		for i := uint16(0); i < numGlyphs; i++ {
			glyphNames[i] = strings.lookup(string(cff.Glyphs[i].Name))
		}

		if isStandardEncoding(cff.Encoding, cff.Glyphs) {
			// topDict[opEncoding] = []interface{}{int32(0)}
		} else if isExpertEncoding(cff.Encoding, cff.Glyphs) {
			topDict[opEncoding] = []interface{}{int32(1)}
		} else {
			encoding, err := encodeEncoding(cff.Encoding, glyphNames)
			if err != nil {
				return err
			}
			secEncodings = len(blobs)
			blobs = append(blobs, encoding)
		}
	}

	// section 6: charsets
	var charsets []byte
	if !cff.IsCIDFont {
		charsets, err = encodeCharset(glyphNames)
	} else {
		charsets, err = encodeCharset(cff.Gid2cid)
	}
	if err != nil {
		return err
	}
	secCharsets := len(blobs)
	blobs = append(blobs, charsets)

	// section 7: FDSelect
	secFdSelect := -1
	if cff.IsCIDFont {
		secFdSelect = len(blobs)
		blobs = append(blobs, cff.FdSelect.encode(int(numGlyphs)))
	}

	// section 8: charstrings INDEX
	secCharStringsIndex := len(blobs)
	blobs = append(blobs, cffIndex(charStrings).encode())

	// section 9: font DICT INDEX
	numFonts := len(cff.Info.Private)
	fontDicts := make([]cffDict, numFonts)
	if cff.IsCIDFont {
		for i := range fontDicts {
			// see afdko/c/shared/source/cffwrite/cffwrite_dict.c:cfwDictFillFont
			fontDict := cffDict{}
			fontDict.setFontMatrix(opFontMatrix, cff.Info.FontMatrix)
			// opPrivate is set below
			fontDicts[i] = fontDict
		}
	}
	secFontDictIndex := len(blobs)
	blobs = append(blobs, nil)

	// section 10: private DICT
	privateDicts := make([]cffDict, numFonts)
	secPrivateDicts := make([]int, numFonts)
	for i := range privateDicts {
		privateDicts[i] = cff.makePrivateDict(i, defWidth, nomWidth)
		// opSubrs is set below
		secPrivateDicts[i] = len(blobs)
		blobs = append(blobs, nil)
	}

	// section 11: subrs INDEX
	secSubrsIndex := len(blobs)
	blobs = append(blobs, cffIndex{}.encode())

	numSections := len(blobs)

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

		var fontDictIndex cffIndex
		for i := 0; i < numFonts; i++ {
			secPrivateDict := secPrivateDicts[i]
			privateDicts[i][opSubrs] = []interface{}{offs[secSubrsIndex] - offs[secPrivateDict]}
			blobs[secPrivateDict] = privateDicts[i].encode(strings)
			pdSize := len(blobs[secPrivateDict])
			pdDesc := []interface{}{int32(pdSize), offs[secPrivateDict]}
			if cff.IsCIDFont {
				fontDicts[i][opPrivate] = pdDesc
				fontDictData := fontDicts[i].encode(strings)
				fontDictIndex = append(fontDictIndex, fontDictData)
			} else {
				topDict[opPrivate] = pdDesc
			}
		}
		if cff.IsCIDFont {
			blobs[secFontDictIndex] = fontDictIndex.encode()
		}

		topDict[opCharset] = []interface{}{offs[secCharsets]}
		if secEncodings >= 4 {
			topDict[opEncoding] = []interface{}{offs[secEncodings]}
		}
		topDict[opCharStrings] = []interface{}{offs[secCharStringsIndex]}
		if secFdSelect >= 0 {
			topDict[opFDSelect] = []interface{}{offs[secFdSelect]}
			topDict[opFDArray] = []interface{}{offs[secFontDictIndex]}
		}
		topDictData := topDict.encode(strings)
		blobs[secTopDictIndex] = cffIndex{topDictData}.encode()

		blobs[secStringIndex] = strings.encode()

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

func (cff *Font) encodeCharStrings() (cffIndex, int32, int32, error) {
	numGlyphs := len(cff.Glyphs)
	if numGlyphs < 1 || (!cff.IsCIDFont && cff.Glyphs[0].Name != ".notdef") {
		return nil, 0, 0, invalidSince("missing .notdef glyph")
	}

	// TODO(voss): re-introduce subroutines.
	//
	// Size used for a subroutine:
	//   - an entry in the subrs and gsubrs INDEX takes
	//     up to 4 bytes, plus the size of the subroutine
	//   - the subrouting must finish with t2return
	//     or t2endchar (1 byte)
	//   - calling the subroutine uses k+1 bytes, where
	//     k=1 for the first 215 subroutines of each type, and
	//     k=2 for the next 2048 subroutines of each type.
	// An approximation could be the following:
	//   - if n bytes occur k times, this uses n*k bytes
	//   - if the n bytes are turned into a subroutine, this uses
	//     approximately k*2 + n + 3 or k*3 + n + 4 bytes.
	//   - the savings are n*k - k*2 - n - 3 = (n-2)*(k-1)-5
	//     or n*k - k*3 - n - 4 = (n-3)*(k-1)-7 bytes.
	//
	// http://www.allisons.org/ll/AlgDS/Tree/Suffix/
	// https://stackoverflow.com/questions/9452701/ukkonens-suffix-tree-algorithm-in-plain-english

	cc := make(cffIndex, numGlyphs)
	defaultWidth, nominalWidth := cff.selectWidths()
	for i, glyph := range cff.Glyphs {
		code, err := glyph.encodeCharString(defaultWidth, nominalWidth)
		if err != nil {
			return nil, 0, 0, err
		}
		cc[i] = code
	}

	return cc, defaultWidth, nominalWidth, nil
}

// Widths returns the widths of all glyphs.
func (cff *Font) Widths() []uint16 {
	res := make([]uint16, len(cff.Glyphs))
	for i, glyph := range cff.Glyphs {
		res[i] = uint16(glyph.Width)
	}
	return res
}

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
