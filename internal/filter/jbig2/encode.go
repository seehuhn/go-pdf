// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package jbig2

import (
	"encoding/binary"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// WriteSegmentHeader writes a JBIG2 segment header.
func WriteSegmentHeader(buf []byte, segNum uint32, segType int, pageAssoc int, refs []uint32, dataLen uint32) []byte {
	// segment number (4 bytes)
	buf = appendUint32(buf, segNum)

	// header flags (1 byte): type in bits 0-5, bit 6 = page assoc size
	flags := byte(segType & 0x3F)
	if pageAssoc > 255 {
		flags |= 0x40
	}
	buf = append(buf, flags)

	// referred-to count and retention flags
	refCount := len(refs)
	if refCount <= 4 {
		// short form: count in bits 7-5, retention flags in bits 4-0
		buf = append(buf, byte(refCount)<<5)
	} else {
		// long form: bits 7-5 = 7, bits 4-0 = high bits of 29-bit count
		buf = append(buf, 0xE0|byte(refCount>>24)&0x1F)
		buf = append(buf, byte(refCount>>16), byte(refCount>>8), byte(refCount))
		// retention flag bytes: ceil((refCount + 1) / 8)
		retBytes := (refCount + 8) / 8
		buf = append(buf, make([]byte, retBytes)...)
	}

	// referred-to segment numbers
	for _, ref := range refs {
		if segNum <= 256 {
			buf = append(buf, byte(ref))
		} else if segNum <= 65536 {
			buf = appendUint16(buf, uint16(ref))
		} else {
			buf = appendUint32(buf, ref)
		}
	}

	// page association
	if pageAssoc > 255 {
		buf = appendUint32(buf, uint32(pageAssoc))
	} else {
		buf = append(buf, byte(pageAssoc))
	}

	// data length (4 bytes)
	buf = appendUint32(buf, dataLen)
	return buf
}

// WriteRegionSegmentInfo writes the 17-byte region segment information field.
func WriteRegionSegmentInfo(buf []byte, width, height, x, y int, combOp bitmap.CombOp) []byte {
	buf = appendUint32(buf, uint32(width))
	buf = appendUint32(buf, uint32(height))
	buf = appendUint32(buf, uint32(x))
	buf = appendUint32(buf, uint32(y))
	buf = append(buf, byte(combOp))
	return buf
}

// appendDefaultAT appends the default adaptive template pixel positions
// for a generic region to buf and returns the updated buf and the
// AT coordinate arrays.
func appendDefaultAT(buf []byte, template int) ([]byte, [4]int8, [4]int8) {
	var atx, aty [4]int8
	switch template {
	case 0:
		atx = [4]int8{3, -3, 2, -2}
		aty = [4]int8{-1, -1, -2, -2}
		for i := range 4 {
			buf = append(buf, byte(atx[i]), byte(aty[i]))
		}
	default:
		atx[0] = 3
		aty[0] = -1
		if template >= 2 {
			atx[0] = 2
		}
		buf = append(buf, byte(atx[0]), byte(aty[0]))
	}
	return buf, atx, aty
}

// WritePageInfo writes a page information segment's data.
func WritePageInfo(buf []byte, width, height int) []byte {
	buf = appendUint32(buf, uint32(width))
	buf = appendUint32(buf, uint32(height))
	buf = appendUint32(buf, 0) // x resolution
	buf = appendUint32(buf, 0) // y resolution
	buf = append(buf, 0x01)    // flags: lossless
	buf = appendUint16(buf, 0) // striping
	return buf
}

// WritePageInfoStripe writes page info for a page with unknown height.
// The page height is determined by end-of-stripe segments.
// maxStripeHeight is the maximum number of rows in any stripe.
func WritePageInfoStripe(buf []byte, width, maxStripeHeight int) []byte {
	buf = appendUint32(buf, uint32(width))
	buf = appendUint32(buf, 0xFFFFFFFF)
	buf = appendUint32(buf, 0) // x resolution
	buf = appendUint32(buf, 0) // y resolution
	buf = append(buf, 0x01)    // flags: lossless
	buf = appendUint16(buf, 0x8000|uint16(maxStripeHeight&0x7FFF))
	return buf
}

// WriteEndOfStripe writes an end-of-stripe segment's data.
// y is the Y coordinate of the last row in the stripe.
func WriteEndOfStripe(buf []byte, y int) []byte {
	return appendUint32(buf, uint32(y))
}

// EncodeSymbolDictSegment encodes a symbol dictionary segment's data.
// Symbols are grouped into height classes automatically.
// Symbols must be sorted by height, then by width within each height class.
func EncodeSymbolDictSegment(symbols []*bitmap.Bitmap, template int) []byte {
	if len(symbols) == 0 {
		return nil
	}

	// SD flags: arithmetic, no refinement, template
	flags := uint16(template&3) << 10
	var buf []byte
	buf = appendUint16(buf, flags)

	// AT positions
	buf, atx, aty := appendDefaultAT(buf, template)

	// SDNUMEXSYMS and SDNUMNEWSYMS
	buf = appendUint32(buf, uint32(len(symbols)))
	buf = appendUint32(buf, uint32(len(symbols)))

	// encode MQ data
	enc := newMQEncoder()
	iadh := &intCtx{}
	iadw := &intCtx{}
	iaex := &intCtx{}
	gbCx := make([]byte, genericContextSize(template))

	hcHeight := 0
	i := 0
	for i < len(symbols) {
		// start new height class
		dh := symbols[i].Height() - hcHeight
		iadh.encode(enc, int64(dh))
		hcHeight = symbols[i].Height()

		// encode symbols in this height class
		prevWidth := 0
		for i < len(symbols) && symbols[i].Height() == hcHeight {
			dw := symbols[i].Width() - prevWidth
			iadw.encode(enc, int64(dw))
			prevWidth = symbols[i].Width()

			p := &genericRegionParams{
				Width:    symbols[i].Width(),
				Height:   hcHeight,
				Template: template,
			}
			copy(p.ATX[:], atx[:])
			copy(p.ATY[:], aty[:])
			encodeGenericRegion(enc, symbols[i], p, gbCx)
			i++
		}
		iadw.encodeOOB(enc)
	}

	// export flags: skip 0, export all
	iaex.encode(enc, 0)
	iaex.encode(enc, int64(len(symbols)))

	enc.flush()
	buf = append(buf, enc.bytes()...)
	return buf
}

// EncodeSymbolDictSegmentHuffRef encodes a Huffman-coded symbol dictionary
// with single-instance refinement aggregation. Each symbols[i] is encoded as
// a refinement of refSymbols[i]. The two slices must have the same length.
func EncodeSymbolDictSegmentHuffRef(
	symbols []*bitmap.Bitmap,
	refSymbols []*bitmap.Bitmap,
	sdrTemplate int,
) ([]byte, error) {
	if len(symbols) != len(refSymbols) {
		return nil, fmt.Errorf("symbols (%d) and refSymbols (%d) length mismatch",
			len(symbols), len(refSymbols))
	}
	if len(symbols) == 0 {
		return nil, nil
	}

	// SD flags: sdhuff=1, sdrefagg=1, dhSel=0 (B.4), dwSel=0 (B.2),
	// aggInstSel=0 (B.1), sdrTemplate
	flags := uint16(0x03) // bits 0-1: sdhuff=1, sdrefagg=1
	flags |= uint16(sdrTemplate&1) << 12

	var buf []byte
	buf = appendUint16(buf, flags)

	// refinement AT flags (only for sdrTemplate == 0)
	var sdrATX [2]int8
	var sdrATY [2]int8
	if sdrTemplate == 0 {
		sdrATX = [2]int8{-1, -1}
		sdrATY = [2]int8{-1, -1}
		buf = append(buf, byte(sdrATX[0]), byte(sdrATY[0]),
			byte(sdrATX[1]), byte(sdrATY[1]))
	}

	// SDNUMEXSYMS and SDNUMNEWSYMS
	n := len(symbols)
	buf = appendUint32(buf, uint32(n))
	buf = appendUint32(buf, uint32(n))

	// symbol ID code length
	numRefSyms := len(refSymbols)
	codeLen := symCodeLen(numRefSyms + n)

	// Huffman bitstream
	w := newBitWriter()

	// group symbols by height class
	i := 0
	hcHeight := 0
	for i < n {
		// height class delta
		dh := symbols[i].Height() - hcHeight
		if err := huffTableB4.encode(w, int64(dh)); err != nil {
			return nil, err
		}
		hcHeight = symbols[i].Height()

		// collect symbols in this height class
		hcFirst := i
		prevWidth := 0
		for i < n && symbols[i].Height() == hcHeight {
			dw := symbols[i].Width() - prevWidth
			if err := huffTableB2.encode(w, int64(dw)); err != nil {
				return nil, err
			}
			prevWidth = symbols[i].Width()
			i++
		}
		// end of height class
		if err := huffTableB2.encodeOOB(w); err != nil {
			return nil, err
		}

		// refinement data for each symbol in this height class
		for j := hcFirst; j < i; j++ {
			// REFAGGNINST = 1
			if err := huffTableB1.encode(w, 1); err != nil {
				return nil, err
			}

			// symbol ID: index into refSymbols
			symID := j
			w.writeBits(uint32(symID), codeLen)

			// RDX = 0, RDY = 0
			if err := huffTableB15.encode(w, 0); err != nil {
				return nil, err
			}
			if err := huffTableB15.encode(w, 0); err != nil {
				return nil, err
			}

			// MQ-encode refinement region
			rp := &refinementParams{
				Width:     symbols[j].Width(),
				Height:    hcHeight,
				Template:  sdrTemplate,
				Reference: refSymbols[j],
			}
			copy(rp.ATX[:], sdrATX[:])
			copy(rp.ATY[:], sdrATY[:])
			mqData := encodeRefinementRegion(symbols[j], rp)

			// refinement data size
			if err := huffTableB1.encode(w, int64(len(mqData))); err != nil {
				return nil, err
			}
			w.align()
			w.writeBytes(mqData)
		}
	}

	// export flags: skip 0 input symbols, export all new symbols
	if err := huffTableB1.encode(w, int64(numRefSyms)); err != nil {
		return nil, err
	}
	if err := huffTableB1.encode(w, int64(n)); err != nil {
		return nil, err
	}

	buf = append(buf, w.bytes()...)
	return buf, nil
}

// EncodeTextRegionSegment encodes a text region segment's data.
// refCorner is one of cornerBottomLeft, cornerTopLeft, cornerBottomRight,
// cornerTopRight. When transposed is true, T and S axes are swapped.
// Instances with a non-nil Bitmap field are encoded with refinement.
func EncodeTextRegionSegment(
	width, height, x, y int,
	instances []SymbolInstance,
	symbols []*bitmap.Bitmap,
	refCorner int,
	transposed bool,
	combOp bitmap.CombOp,
) []byte {
	numSymbols := len(symbols)

	// detect whether any instance needs refinement
	needsRefine := false
	for i := range instances {
		if instances[i].Bitmap != nil {
			needsRefine = true
			break
		}
	}

	// text region flags: arithmetic, strips=1
	flags := uint16(0) // SBHUFF=0, LOGSBSTRIPS=0
	if needsRefine {
		flags |= 0x02            // SBREFINE=1
		flags |= uint16(1) << 15 // SBRTEMPLATE=1
	}
	flags |= uint16(refCorner&3) << 4
	if transposed {
		flags |= 0x40
	}
	flags |= uint16(combOp&3) << 7

	var buf []byte
	buf = WriteRegionSegmentInfo(buf, width, height, x, y, combOp)
	buf = appendUint16(buf, flags)

	// number of instances
	buf = appendUint32(buf, uint32(len(instances)))

	// encode MQ data
	enc := newMQEncoder()
	iadt := &intCtx{}
	iafs := &intCtx{}
	iads := &intCtx{}
	iaid, _ := newIAIDCtx(textRegionSymCodeLen(numSymbols))
	codeLen := textRegionSymCodeLen(numSymbols)

	// refinement contexts (allocated only when needed)
	var iari, iardw, iardh, iardx, iardy *intCtx
	if needsRefine {
		iari = &intCtx{}
		iardw = &intCtx{}
		iardh = &intCtx{}
		iardx = &intCtx{}
		iardy = &intCtx{}
	}

	if len(instances) == 0 {
		enc.flush()
		buf = append(buf, enc.bytes()...)
		return buf
	}

	// initial STRIPT (negated, strips=1 so no multiplication)
	firstT := instances[0].T
	iadt.encode(enc, int64(-firstT))

	stripT := firstT
	firstS := 0
	nInst := 0

	for nInst < len(instances) {
		curT := instances[nInst].T
		dt := curT - stripT
		iadt.encode(enc, int64(dt))
		stripT = curT

		// encode instances in this strip
		first := true
		curS := 0
		for nInst < len(instances) && instances[nInst].T == curT {
			inst := instances[nInst]
			if first {
				dfs := inst.S - firstS
				iafs.encode(enc, int64(dfs))
				firstS = inst.S
				curS = inst.S
				first = false
			} else {
				ids := inst.S - curS
				iads.encode(enc, int64(ids))
				curS = inst.S
			}

			// symbol ID
			iaid.encode(enc, codeLen, inst.SymID)

			// refinement (§6.4.11)
			if needsRefine {
				if inst.Bitmap == nil {
					iari.encode(enc, 0)
				} else {
					iari.encode(enc, 1)
					origSym := symbols[inst.SymID]
					rdw := int64(inst.Bitmap.Width() - origSym.Width())
					rdh := int64(inst.Bitmap.Height() - origSym.Height())
					iardw.encode(enc, rdw)
					iardh.encode(enc, rdh)
					iardx.encode(enc, 0)
					iardy.encode(enc, 0)
					rp := &refinementParams{
						Width:     inst.Bitmap.Width(),
						Height:    inst.Bitmap.Height(),
						Template:  1, // matches SBRTEMPLATE=1
						Reference: origSym,
						RefDX:     int(rdw >> 1),
						RefDY:     int(rdh >> 1),
					}
					encodeRefinementRegionInline(enc, inst.Bitmap, rp, nil)
				}
			}

			// CURS pre-placement update (§6.4.5 step 3c-vi)
			if !transposed {
				if refCorner == cornerTopRight || refCorner == cornerBottomRight {
					curS += inst.Wi - 1
				}
			} else {
				if refCorner == cornerBottomLeft || refCorner == cornerBottomRight {
					curS += inst.Hi - 1
				}
			}

			// CURS post-placement update (§6.4.5 step 3c-x)
			if !transposed {
				if refCorner == cornerTopLeft || refCorner == cornerBottomLeft {
					curS += inst.Wi - 1
				}
			} else {
				if refCorner == cornerTopLeft || refCorner == cornerTopRight {
					curS += inst.Hi - 1
				}
			}

			nInst++
		}
		// OOB to end strip (unless last strip)
		if nInst < len(instances) {
			iads.encodeOOB(enc)
		}
	}
	iads.encodeOOB(enc) // final OOB

	enc.flush()
	buf = append(buf, enc.bytes()...)
	return buf
}

// SymbolInstance describes a symbol placement in a text region.
// T is the strip coordinate (Y when not transposed, X when transposed).
// S is the inline coordinate (X when not transposed, Y when transposed).
// When Bitmap is non-nil, the instance is encoded with refinement (ri=1)
// against the dictionary symbol identified by SymID. Wi and Hi must then
// match the Bitmap dimensions. When Bitmap is nil, the dictionary symbol
// is placed as-is (ri=0).
type SymbolInstance struct {
	SymID  int
	T, S   int
	Wi, Hi int            // bitmap dimensions for CURS update
	Bitmap *bitmap.Bitmap // refined bitmap; nil = use dictionary symbol
}

// encodeSymIDHuffTable writes the custom symbol-ID Huffman table to w
// using run-length coding (§7.4.3.7) and returns the resulting table.
// All symbols receive the same code length (uniform coding).
func encodeSymIDHuffTable(w *bitWriter, numSyms int) *huffTable {
	codeLen := textRegionSymCodeLen(numSyms)

	// step 1: write 35 four-bit RUNCODE code lengths.
	// Only the entry for code length `codeLen` is non-zero (prefix length 1).
	// Entry 32 (repeat previous) also gets prefix length 2 so we can use it.
	for i := range 35 {
		switch i {
		case codeLen:
			w.writeBits(1, 4) // prefix length 1
		case 32:
			w.writeBits(2, 4) // prefix length 2
		default:
			w.writeBits(0, 4) // unused
		}
	}

	// step 2: build the RUNCODE table and assign codes
	runCodeLines := make([]huffLine, 35)
	for i := range 35 {
		switch i {
		case codeLen:
			runCodeLines[i] = huffLine{RangeLow: int32(i), PrefLen: 1}
		case 32:
			runCodeLines[i] = huffLine{RangeLow: 32, PrefLen: 2}
		default:
			runCodeLines[i] = huffLine{RangeLow: int32(i), PrefLen: 0}
		}
	}
	runCodeTable := &huffTable{Lines: runCodeLines}
	runCodeTable.assignCodes()

	// steps 3-5: encode the symbol code lengths.
	// First symbol as literal, then repeat via RUNCODE 32.
	remaining := numSyms
	runCodeTable.encode(w, int64(codeLen)) // literal
	remaining--

	for remaining >= 3 {
		repeat := min(remaining, 6)
		runCodeTable.encode(w, 32)       // RUNCODE 32: repeat previous
		w.writeBits(uint32(repeat-3), 2) // 2 extra bits: repeat count - 3
		remaining -= repeat
	}
	for range remaining {
		runCodeTable.encode(w, int64(codeLen)) // literal for remainder
	}

	// step 6: byte-align
	w.align()

	// step 7: build the symbol ID Huffman table
	lines := make([]huffLine, numSyms)
	for i := range numSyms {
		lines[i] = huffLine{RangeLow: int32(i), PrefLen: codeLen}
	}
	t := &huffTable{Lines: lines}
	t.assignCodes()
	return t
}

// EncodeTextRegionSegmentHuffman encodes a Huffman-coded text region segment.
// Uses standard tables B.6 (FS), B.8 (DS), B.11 (DT).
// refCorner is one of cornerBottomLeft, cornerTopLeft, cornerBottomRight,
// cornerTopRight. When transposed is true, T and S axes are swapped.
// Instances with a non-nil Bitmap field are encoded with refinement.
func EncodeTextRegionSegmentHuffman(
	width, height, x, y int,
	instances []SymbolInstance,
	symbols []*bitmap.Bitmap,
	refCorner int,
	transposed bool,
	combOp bitmap.CombOp,
) ([]byte, error) {
	return encodeTextRegionHuffman(
		width, height, x, y, instances, symbols,
		refCorner, transposed, combOp,
		huffTableB6, uint16(0))
}

// encodeTextRegionHuffman is the shared implementation for Huffman-coded
// text region encoding. fsTable is the Huffman table used for the first-S
// (SBHUFFFS) values. htags is the Huffman table selection flags written
// into the segment header.
func encodeTextRegionHuffman(
	width, height, x, y int,
	instances []SymbolInstance,
	symbols []*bitmap.Bitmap,
	refCorner int,
	transposed bool,
	combOp bitmap.CombOp,
	fsTable *huffTable,
	htags uint16,
) ([]byte, error) {
	numSymbols := len(symbols)

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances to encode")
	}

	// sort instances by T (ascending) since B.11 only encodes values >= 1
	sorted := make([]SymbolInstance, len(instances))
	copy(sorted, instances)
	slices.SortStableFunc(sorted, func(a, b SymbolInstance) int {
		return a.T - b.T
	})
	instances = sorted

	// detect whether any instance needs refinement
	needsRefine := false
	for i := range instances {
		if instances[i].Bitmap != nil {
			needsRefine = true
			break
		}
	}

	// text region flags: SBHUFF=1, LOGSBSTRIPS=0
	flags := uint16(1) // SBHUFF=1
	if needsRefine {
		flags |= 0x02            // SBREFINE=1
		flags |= uint16(1) << 15 // SBRTEMPLATE=1
	}
	flags |= uint16(refCorner&3) << 4
	if transposed {
		flags |= 0x40
	}
	flags |= uint16(combOp&3) << 7

	var buf []byte
	buf = WriteRegionSegmentInfo(buf, width, height, x, y, combOp)
	buf = appendUint16(buf, flags)
	buf = appendUint16(buf, htags)
	buf = appendUint32(buf, uint32(len(instances)))

	// Huffman bitstream
	w := &bitWriter{}

	// symbol ID table
	symIDTable := encodeSymIDHuffTable(w, numSymbols)

	// initial STRIPT: encode 1 so stripT = -1
	// (B.11 minimum value is 1)
	huffTableB11.encode(w, 1)
	stripT := -1
	firstS := 0
	nInst := 0

	for nInst < len(instances) {
		curT := instances[nInst].T
		dt := curT - stripT
		if dt < 1 {
			return nil, fmt.Errorf("non-positive strip delta %d (curT=%d, stripT=%d)", dt, curT, stripT)
		}
		huffTableB11.encode(w, int64(dt))
		stripT = curT

		// encode instances in this strip
		first := true
		curS := 0
		for nInst < len(instances) && instances[nInst].T == curT {
			inst := instances[nInst]
			if first {
				dfs := inst.S - firstS
				fsTable.encode(w, int64(dfs))
				firstS = inst.S
				curS = inst.S
				first = false
			} else {
				ids := inst.S - curS
				huffTableB8.encode(w, int64(ids))
				curS = inst.S
			}

			// symbol ID
			symIDTable.encode(w, int64(inst.SymID))

			// refinement (§6.4.11)
			if needsRefine {
				if inst.Bitmap == nil {
					w.writeBit(0)
				} else {
					w.writeBit(1)
					origSym := symbols[inst.SymID]
					rdw := int64(inst.Bitmap.Width() - origSym.Width())
					rdh := int64(inst.Bitmap.Height() - origSym.Height())
					huffTableB14.encode(w, rdw)
					huffTableB14.encode(w, rdh)
					huffTableB14.encode(w, 0) // rdx
					huffTableB14.encode(w, 0) // rdy
					rp := &refinementParams{
						Width:     inst.Bitmap.Width(),
						Height:    inst.Bitmap.Height(),
						Template:  1,
						Reference: origSym,
						RefDX:     int(rdw >> 1),
						RefDY:     int(rdh >> 1),
					}
					mqData := encodeRefinementRegion(inst.Bitmap, rp)
					huffTableB1.encode(w, int64(len(mqData)))
					w.align()
					w.writeBytes(mqData)
				}
			}

			// CURS pre-placement update (§6.4.5 step 3c-vi)
			if !transposed {
				if refCorner == cornerTopRight || refCorner == cornerBottomRight {
					curS += inst.Wi - 1
				}
			} else {
				if refCorner == cornerBottomLeft || refCorner == cornerBottomRight {
					curS += inst.Hi - 1
				}
			}

			// CURS post-placement update (§6.4.5 step 3c-x)
			if !transposed {
				if refCorner == cornerTopLeft || refCorner == cornerBottomLeft {
					curS += inst.Wi - 1
				}
			} else {
				if refCorner == cornerTopLeft || refCorner == cornerTopRight {
					curS += inst.Hi - 1
				}
			}

			nInst++
		}
		// OOB to end strip
		huffTableB8.encodeOOB(w)
	}

	w.align()
	buf = append(buf, w.bytes()...)
	return buf, nil
}

// encodeGenericRegionSegment encodes a generic region segment's data.
func EncodeGenericRegionSegment(bm *bitmap.Bitmap, x, y int, template int, combOp bitmap.CombOp) []byte {
	var buf []byte
	buf = WriteRegionSegmentInfo(buf, bm.Width(), bm.Height(), x, y, combOp)

	// generic region flags (1 byte)
	flags := byte((template & 3) << 1) // MMR=0, TPGDON=0
	buf = append(buf, flags)

	// AT positions
	buf, atx, aty := appendDefaultAT(buf, template)

	// encode the bitmap
	p := &genericRegionParams{
		Width:    bm.Width(),
		Height:   bm.Height(),
		Template: template,
	}
	copy(p.ATX[:], atx[:])
	copy(p.ATY[:], aty[:])

	enc := newMQEncoder()
	encodeGenericRegion(enc, bm, p, nil)
	enc.flush()
	buf = append(buf, enc.bytes()...)
	return buf
}

// EncodeRefinementRegionSegment encodes a generic refinement region segment
// (type 42). The bitmap bm is encoded as a refinement of ref.
func EncodeRefinementRegionSegment(bm, ref *bitmap.Bitmap, x, y, template int, combOp bitmap.CombOp) []byte {
	var buf []byte
	buf = WriteRegionSegmentInfo(buf, bm.Width(), bm.Height(), x, y, combOp)

	// refinement region flags (1 byte): bit 0 = GRTEMPLATE, bit 1 = TPGRON
	buf = append(buf, byte(template&1))

	// AT positions (only for template 0)
	p := &refinementParams{
		Width:     bm.Width(),
		Height:    bm.Height(),
		Template:  template,
		Reference: ref,
	}
	if template == 0 {
		p.ATX = [2]int8{-1, -1}
		p.ATY = [2]int8{-1, -1}
		buf = append(buf, byte(p.ATX[0]), byte(p.ATY[0]), byte(p.ATX[1]), byte(p.ATY[1]))
	}

	mqData := encodeRefinementRegion(bm, p)
	buf = append(buf, mqData...)
	return buf
}

// EncodeGenericRegionSegmentMMR encodes a generic region segment using MMR.
func EncodeGenericRegionSegmentMMR(bm *bitmap.Bitmap, x, y int, combOp bitmap.CombOp) ([]byte, error) {
	var buf []byte
	buf = WriteRegionSegmentInfo(buf, bm.Width(), bm.Height(), x, y, combOp)

	// generic region flags: MMR=1, template=0, TPGDON=0
	buf = append(buf, 0x01)

	// no AT positions for MMR

	mmrData, err := encodeMMR(bm)
	if err != nil {
		return nil, err
	}
	buf = append(buf, mmrData...)
	return buf, nil
}

// encodePatternDictSegment encodes a pattern dictionary segment's data.
// All patterns must have the same dimensions.
func encodePatternDictSegment(patterns []*bitmap.Bitmap, template int) []byte {
	if len(patterns) == 0 {
		return nil
	}

	hdpw := patterns[0].Width()
	hdph := patterns[0].Height()
	grayMax := len(patterns) - 1

	// flags (1 byte): MMR=0, template in bits 1-2
	flags := byte((template & 3) << 1)
	var buf []byte
	buf = append(buf, flags)
	buf = append(buf, byte(hdpw))
	buf = append(buf, byte(hdph))
	buf = appendUint32(buf, uint32(grayMax))

	// build collective bitmap: all patterns side by side
	collectiveWidth := len(patterns) * hdpw
	collective := bitmap.New(collectiveWidth, hdph)
	for i, pat := range patterns {
		xOff := i * hdpw
		for y := range hdph {
			for x := range hdpw {
				collective.SetPixel(xOff+x, y, pat.GetPixel(x, y))
			}
		}
	}

	// AT positions per Table 27
	p := &genericRegionParams{
		Width:    collectiveWidth,
		Height:   hdph,
		Template: template,
	}
	switch template {
	case 0:
		// AT positions per Table 27 (written to header per §7.4.4.2)
		p.ATX[0] = int8(-hdpw)
		p.ATY[0] = 0
		p.ATX[1] = -3
		p.ATY[1] = -1
		p.ATX[2] = 2
		p.ATY[2] = -2
		p.ATX[3] = -2
		p.ATY[3] = -2
		buf = append(buf,
			byte(p.ATX[0]), byte(p.ATY[0]),
			byte(p.ATX[1]), byte(p.ATY[1]),
			byte(p.ATX[2]), byte(p.ATY[2]),
			byte(p.ATX[3]), byte(p.ATY[3]))
	case 1:
		// AT positions are fixed per Table 27 and not written to the
		// segment header (unlike symbol dict / generic region segments).
		p.ATX[0] = 3
		p.ATY[0] = -1
	default:
		p.ATX[0] = 2
		p.ATY[0] = -1
	}

	enc := newMQEncoder()
	encodeGenericRegion(enc, collective, p, nil)
	enc.flush()
	buf = append(buf, enc.bytes()...)
	return buf
}

// encodePatternDictSegmentMMR encodes a pattern dictionary segment using MMR.
func encodePatternDictSegmentMMR(patterns []*bitmap.Bitmap) ([]byte, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	hdpw := patterns[0].Width()
	hdph := patterns[0].Height()
	grayMax := len(patterns) - 1

	// flags: MMR=1, template=0
	var buf []byte
	buf = append(buf, 0x01)
	buf = append(buf, byte(hdpw))
	buf = append(buf, byte(hdph))
	buf = appendUint32(buf, uint32(grayMax))

	// no AT positions for MMR

	// collective bitmap: all patterns side by side
	collectiveWidth := len(patterns) * hdpw
	collective := bitmap.New(collectiveWidth, hdph)
	for i, pat := range patterns {
		xOff := i * hdpw
		for y := range hdph {
			for x := range hdpw {
				collective.SetPixel(xOff+x, y, pat.GetPixel(x, y))
			}
		}
	}

	mmrData, err := encodeMMR(collective)
	if err != nil {
		return nil, err
	}
	buf = append(buf, mmrData...)
	return buf, nil
}

// halftoneATPositions returns the Table C.4 AT positions for gray-scale
// bitplane encoding/decoding.
func halftoneATPositions(template int) (atx [4]int8, aty [4]int8) {
	switch template {
	case 0:
		atx = [4]int8{-1, -3, -1, 2}
		aty = [4]int8{-2, -2, -2, -2}
	case 1:
		atx[0] = -1
		aty[0] = -2
	case 2, 3:
		atx[0] = 2
		aty[0] = -2
	}
	return
}

// encodeGrayScaleImage encodes a gray-scale image to bitplanes (Annex C).
// grayValues is row-major [gsh][gsw]. Returns the concatenated MQ data.
func encodeGrayScaleImage(grayValues []int, gsw, gsh, template int) []byte {
	// determine bits per gray value
	maxVal := 0
	for _, v := range grayValues {
		if v > maxVal {
			maxVal = v
		}
	}
	gsbpp := 1
	for (1 << gsbpp) <= maxVal {
		gsbpp++
	}

	// extract bitplanes from gray values
	planes := make([]*bitmap.Bitmap, gsbpp)
	for j := range gsbpp {
		planes[j] = bitmap.New(gsw, gsh)
		for y := range gsh {
			for x := range gsw {
				if grayValues[y*gsw+x]&(1<<j) != 0 {
					planes[j].SetPixel(x, y, true)
				}
			}
		}
	}

	// binary to Gray code: G[j] = B[j] XOR B[j+1]
	// iterate upward so planes[j+1] is still in binary form
	for j := range gsbpp - 1 {
		for y := range gsh {
			for x := range gsw {
				above := planes[j+1].GetPixel(x, y)
				cur := planes[j].GetPixel(x, y)
				planes[j].SetPixel(x, y, above != cur)
			}
		}
	}

	// encode each bitplane from MSB to LSB
	atx, aty := halftoneATPositions(template)
	var result []byte
	for j := gsbpp - 1; j >= 0; j-- {
		p := &genericRegionParams{
			Width:    gsw,
			Height:   gsh,
			Template: template,
		}
		copy(p.ATX[:], atx[:])
		copy(p.ATY[:], aty[:])

		enc := newMQEncoder()
		encodeGenericRegion(enc, planes[j], p, nil)
		enc.flush()
		// include the trailing byte (buf[bp]) that the decoder's
		// MQ look-ahead will read when bitplanes are concatenated
		result = append(result, enc.buf[1:]...)
	}
	return result
}

// encodeGrayScaleImageMMR encodes a gray-scale image to MMR-coded bitplanes (Annex C).
func encodeGrayScaleImageMMR(grayValues []int, gsw, gsh int) ([]byte, error) {
	// determine bits per gray value
	maxVal := 0
	for _, v := range grayValues {
		if v > maxVal {
			maxVal = v
		}
	}
	gsbpp := 1
	for (1 << gsbpp) <= maxVal {
		gsbpp++
	}

	// extract bitplanes from gray values
	planes := make([]*bitmap.Bitmap, gsbpp)
	for j := range gsbpp {
		planes[j] = bitmap.New(gsw, gsh)
		for y := range gsh {
			for x := range gsw {
				if grayValues[y*gsw+x]&(1<<j) != 0 {
					planes[j].SetPixel(x, y, true)
				}
			}
		}
	}

	// binary to Gray code
	for j := range gsbpp - 1 {
		for y := range gsh {
			for x := range gsw {
				above := planes[j+1].GetPixel(x, y)
				cur := planes[j].GetPixel(x, y)
				planes[j].SetPixel(x, y, above != cur)
			}
		}
	}

	// encode each bitplane from MSB to LSB
	var result []byte
	for j := gsbpp - 1; j >= 0; j-- {
		data, err := encodeMMR(planes[j])
		if err != nil {
			return nil, err
		}
		result = append(result, data...)
	}
	return result, nil
}

// encodeHalftoneRegionSegmentMMR encodes a halftone region segment using MMR.
func encodeHalftoneRegionSegmentMMR(
	width, height int,
	grayValues []int, gsw, gsh int,
	hgx, hgy, hrx, hry int,
	combOp bitmap.CombOp,
) ([]byte, error) {
	var buf []byte
	buf = WriteRegionSegmentInfo(buf, width, height, 0, 0, combOp)

	// halftone flags: MMR=1, template=0, enableSkip=0, combOp, defPixel=0
	flags := byte(0x01) | byte((combOp&7)<<4)
	buf = append(buf, flags)

	// grid parameters
	buf = appendUint32(buf, uint32(gsw))
	buf = appendUint32(buf, uint32(gsh))
	buf = appendUint32(buf, uint32(int32(hgx)))
	buf = appendUint32(buf, uint32(int32(hgy)))
	buf = appendUint16(buf, uint16(hrx))
	buf = appendUint16(buf, uint16(hry))

	// encode gray-scale image
	gsData, err := encodeGrayScaleImageMMR(grayValues, gsw, gsh)
	if err != nil {
		return nil, err
	}
	buf = append(buf, gsData...)
	return buf, nil
}

// encodeHalftoneRegionSegment encodes a halftone region segment's data.
func encodeHalftoneRegionSegment(
	width, height int,
	grayValues []int, gsw, gsh int,
	hgx, hgy, hrx, hry int,
	template int, combOp bitmap.CombOp,
) []byte {
	var buf []byte
	buf = WriteRegionSegmentInfo(buf, width, height, 0, 0, combOp)

	// halftone flags (1 byte): MMR=0, template, enableSkip=0, combOp, defPixel=0
	flags := byte((template&3)<<1) | byte((combOp&7)<<4)
	buf = append(buf, flags)

	// grid parameters
	buf = appendUint32(buf, uint32(gsw))
	buf = appendUint32(buf, uint32(gsh))
	buf = appendUint32(buf, uint32(int32(hgx)))
	buf = appendUint32(buf, uint32(int32(hgy)))
	buf = appendUint16(buf, uint16(hrx))
	buf = appendUint16(buf, uint16(hry))

	// encode gray-scale image
	buf = append(buf, encodeGrayScaleImage(grayValues, gsw, gsh, template)...)
	return buf
}

func appendUint16(buf []byte, v uint16) []byte {
	return append(buf, byte(v>>8), byte(v))
}

func appendUint32(buf []byte, v uint32) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return append(buf, b...)
}
