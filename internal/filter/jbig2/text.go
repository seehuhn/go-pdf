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
	"fmt"
	"math/bits"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// reference corner values (Table 10 / §7.4.3)
const (
	cornerBottomLeft  = 0
	cornerTopLeft     = 1
	cornerBottomRight = 2
	cornerTopRight    = 3
)

// textRegionParams holds parameters for text region decoding.
type textRegionParams struct {
	SBHuff       bool
	SBRefine     bool
	Width        int
	Height       int
	NumInstances int
	Strips       int // 1, 2, 4, or 8
	Symbols      []*bitmap.Bitmap
	SymCodeLen   int
	DefPixel     int
	CombOp       bitmap.CombOp
	Transposed   bool
	RefCorner    int
	DSOffset     int

	// refinement parameters
	RTemplate int
	RATX      [2]int8
	RATY      [2]int8
}

// decodeTextRegion decodes a text region bitmap using arithmetic coding.
func decodeTextRegion(dec *mqDecoder, p *textRegionParams) (*bitmap.Bitmap, error) {
	if err := checkBitmapSize(p.Width, p.Height); err != nil {
		return nil, err
	}
	bm := bitmap.New(p.Width, p.Height)
	if p.DefPixel != 0 {
		for i := range bm.Pix {
			bm.Pix[i] = 0xFF
		}
	}

	iadt := &intCtx{}
	iafs := &intCtx{}
	iads := &intCtx{}
	iait := &intCtx{}
	iaid, err := newIAIDCtx(p.SymCodeLen)
	if err != nil {
		return nil, err
	}
	iari := &intCtx{}
	iardw := &intCtx{}
	iardh := &intCtx{}
	iardx := &intCtx{}
	iardy := &intCtx{}

	var stripT, firstS int64
	nInstances := 0

	// decode initial STRIPT (§6.4.6: multiply by SBSTRIPS, then negate)
	dt := iadt.decode(dec)
	stripT = -dt * int64(p.Strips)

	for nInstances < p.NumInstances {
		// decode strip delta T (§6.4.6)
		dt = iadt.decode(dec)
		stripT += dt * int64(p.Strips)

		// decode symbol instances in strip (§6.4.5 step 3c)
		first := true
		var curS int64
		for nInstances < p.NumInstances {
			if first {
				dfs := iafs.decode(dec)
				firstS += dfs
				curS = firstS
				first = false
			} else {
				ids := iads.decode(dec)
				if ids == oobResult {
					break // end of strip
				}
				curS += ids + int64(p.DSOffset)
			}

			// decode T within strip (§6.4.9)
			var curT int64
			if p.Strips > 1 {
				curT = iait.decode(dec)
			}
			ti := stripT + curT

			// decode symbol ID (§6.4.10)
			symID := iaid.decode(dec, p.SymCodeLen)
			if symID >= len(p.Symbols) {
				nInstances++
				continue
			}

			// determine symbol bitmap (§6.4.11)
			var ib *bitmap.Bitmap
			var ri int64
			if p.SBRefine {
				ri = iari.decode(dec)
			}

			if ri == 0 {
				ib = p.Symbols[symID]
			} else {
				// refinement coding
				rdw := iardw.decode(dec)
				rdh := iardh.decode(dec)
				rdx := iardx.decode(dec)
				rdy := iardy.decode(dec)

				origSym := p.Symbols[symID]
				rp := &refinementParams{
					Width:     int(int64(origSym.Width()) + rdw),
					Height:    int(int64(origSym.Height()) + rdh),
					Template:  p.RTemplate,
					Reference: origSym,
					RefDX:     int((rdw >> 1) + rdx),
					RefDY:     int((rdh >> 1) + rdy),
				}
				copy(rp.ATX[:], p.RATX[:])
				copy(rp.ATY[:], p.RATY[:])
				var err error
				ib, err = decodeRefinementRegion(dec, rp, nil)
				if err != nil {
					return nil, err
				}
			}

			if ib == nil {
				nInstances++
				continue
			}

			wi := int64(ib.Width())
			hi := int64(ib.Height())

			// update CURS before placement (§6.4.5 step 3c-vi)
			if !p.Transposed {
				if p.RefCorner == cornerTopRight || p.RefCorner == cornerBottomRight {
					curS += wi - 1
				}
			} else {
				if p.RefCorner == cornerBottomLeft || p.RefCorner == cornerBottomRight {
					curS += hi - 1
				}
			}

			si := curS

			// placement position (§6.4.5 step 3c-viii)
			var px, py int64
			if !p.Transposed {
				px = si
				py = ti
			} else {
				px = ti
				py = si
			}

			// adjust for reference corner
			switch p.RefCorner {
			case cornerTopLeft:
				// top-left at (px, py)
			case cornerTopRight:
				px -= wi - 1
			case cornerBottomLeft:
				py -= hi - 1
			case cornerBottomRight:
				px -= wi - 1
				py -= hi - 1
			}

			// composite symbol (§6.4.5 step 3c-ix)
			bm.Combine(ib, int(px), int(py), p.CombOp)

			// update CURS after placement (§6.4.5 step 3c-x)
			if !p.Transposed {
				if p.RefCorner == cornerTopLeft || p.RefCorner == cornerBottomLeft {
					curS += wi - 1
				}
			} else {
				if p.RefCorner == cornerTopLeft || p.RefCorner == cornerTopRight {
					curS += hi - 1
				}
			}

			nInstances++
		}
	}

	return bm, nil
}

// textRegionSymCodeLen computes the symbol code length for a text region.
func textRegionSymCodeLen(numSyms int) int {
	if numSyms <= 1 {
		return 1
	}
	return bits.Len(uint(numSyms - 1))
}

// textRegionHuffParams holds parameters for Huffman text region decoding.
type textRegionHuffParams struct {
	Width, Height int
	NumInstances  int
	Strips        int
	Symbols       []*bitmap.Bitmap
	DefPixel      int
	CombOp        bitmap.CombOp
	Transposed    bool
	RefCorner     int
	DSOffset      int
	SBRefine      bool
	RTemplate     int
	RATX, RATY    [2]int8
	// Huffman tables
	FSTable    *huffTable
	DSTable    *huffTable
	DTTable    *huffTable
	RDWTable   *huffTable
	RDHTable   *huffTable
	RDXTable   *huffTable
	RDYTable   *huffTable
	RSIZETable *huffTable
	SymIDTable *huffTable
}

// decodeTextRegionHuffman decodes a Huffman-coded text region.
func decodeTextRegionHuffman(hr *huffReader, p *textRegionHuffParams) (*bitmap.Bitmap, error) {
	if err := checkBitmapSize(p.Width, p.Height); err != nil {
		return nil, err
	}
	bm := bitmap.New(p.Width, p.Height)
	if p.DefPixel != 0 {
		for i := range bm.Pix {
			bm.Pix[i] = 0xFF
		}
	}

	var stripT, firstS int64
	nInstances := 0

	// initial STRIPT (§6.4.6)
	dt := p.DTTable.decode(hr)
	stripT = -dt * int64(p.Strips)

	for nInstances < p.NumInstances {
		// strip delta T
		dt = p.DTTable.decode(hr)
		stripT += dt * int64(p.Strips)

		// symbol instances in strip
		first := true
		var curS int64
		for nInstances < p.NumInstances {
			if first {
				dfs := p.FSTable.decode(hr)
				firstS += dfs
				curS = firstS
				first = false
			} else {
				ids := p.DSTable.decode(hr)
				if ids == oobResult {
					break // end of strip
				}
				curS += ids + int64(p.DSOffset)
			}

			// T within strip (§6.4.9)
			var curT int64
			if p.Strips > 1 {
				curT = int64(hr.readBits(bits.Len(uint(p.Strips - 1))))
			}
			ti := stripT + curT

			// symbol ID (§6.4.10)
			symID := p.SymIDTable.decode(hr)
			if symID >= int64(len(p.Symbols)) || symID < 0 {
				nInstances++
				continue
			}

			// symbol bitmap (§6.4.11)
			var ib *bitmap.Bitmap
			ri := 0
			if p.SBRefine {
				ri = hr.readBit()
			}

			if ri == 0 {
				ib = p.Symbols[symID]
			} else {
				// Huffman refinement
				rdw := p.RDWTable.decode(hr)
				rdh := p.RDHTable.decode(hr)
				rdx := p.RDXTable.decode(hr)
				rdy := p.RDYTable.decode(hr)
				rsize := int(p.RSIZETable.decode(hr))

				hr.align()
				offset := hr.offset()
				if offset+rsize > len(hr.data) {
					nInstances++
					continue
				}

				origSym := p.Symbols[symID]
				rp := &refinementParams{
					Width:     int(int64(origSym.Width()) + rdw),
					Height:    int(int64(origSym.Height()) + rdh),
					Template:  p.RTemplate,
					Reference: origSym,
					RefDX:     int((rdw >> 1) + rdx),
					RefDY:     int((rdh >> 1) + rdy),
				}
				copy(rp.ATX[:], p.RATX[:])
				copy(rp.ATY[:], p.RATY[:])

				dec := newMQDecoder(hr.data[offset : offset+rsize])
				var err error
				ib, err = decodeRefinementRegion(dec, rp, nil)
				if err != nil {
					return nil, err
				}

				hr.bytePos = offset + rsize
				hr.bitPos = 7
			}

			if ib == nil {
				nInstances++
				continue
			}

			wi := int64(ib.Width())
			hi := int64(ib.Height())

			// CURS pre-update (§6.4.5 step 3c-vi)
			if !p.Transposed {
				if p.RefCorner == cornerTopRight || p.RefCorner == cornerBottomRight {
					curS += wi - 1
				}
			} else {
				if p.RefCorner == cornerBottomLeft || p.RefCorner == cornerBottomRight {
					curS += hi - 1
				}
			}

			si := curS

			// placement (§6.4.5 step 3c-viii)
			var px, py int64
			if !p.Transposed {
				px = si
				py = ti
			} else {
				px = ti
				py = si
			}

			switch p.RefCorner {
			case cornerTopLeft:
				// no adjustment
			case cornerTopRight:
				px -= wi - 1
			case cornerBottomLeft:
				py -= hi - 1
			case cornerBottomRight:
				px -= wi - 1
				py -= hi - 1
			}

			bm.Combine(ib, int(px), int(py), p.CombOp)

			// CURS post-update (§6.4.5 step 3c-x)
			if !p.Transposed {
				if p.RefCorner == cornerTopLeft || p.RefCorner == cornerBottomLeft {
					curS += wi - 1
				}
			} else {
				if p.RefCorner == cornerTopLeft || p.RefCorner == cornerTopRight {
					curS += hi - 1
				}
			}

			nInstances++
		}
	}

	if hr.err != nil {
		return nil, hr.err
	}
	return bm, nil
}

// decodeSymIDHuffTable decodes the symbol ID Huffman table from the
// text region segment data (§7.4.3.7).
func decodeSymIDHuffTable(hr *huffReader, numSyms int) (*huffTable, error) {
	// step 1: read 35 four-bit RUNCODE code lengths
	runCodeLens := make([]int, 35)
	for i := range 35 {
		runCodeLens[i] = int(hr.readBits(4))
	}

	// step 2: assign Huffman codes to RUNCODEs
	runCodeLines := make([]huffLine, 35)
	for i, l := range runCodeLens {
		runCodeLines[i] = huffLine{
			RangeLow: int32(i),
			PrefLen:  l,
			RangeLen: 0,
		}
	}
	runCodeTable := &huffTable{Lines: runCodeLines}
	runCodeTable.assignCodes()

	// steps 3-5: decode symbol ID code lengths
	symCodeLens := make([]int, 0, numSyms)
	prevLen := 0
	for len(symCodeLens) < numSyms {
		rc := int(runCodeTable.decode(hr))
		switch {
		case rc >= 0 && rc <= 31:
			// literal code length
			symCodeLens = append(symCodeLens, rc)
			prevLen = rc
		case rc == 32:
			// repeat previous 3-6 times
			repeat := int(hr.readBits(2)) + 3
			for range repeat {
				if len(symCodeLens) >= numSyms {
					break
				}
				symCodeLens = append(symCodeLens, prevLen)
			}
		case rc == 33:
			// repeat 0 for 3-10 times
			repeat := int(hr.readBits(3)) + 3
			for range repeat {
				if len(symCodeLens) >= numSyms {
					break
				}
				symCodeLens = append(symCodeLens, 0)
			}
			prevLen = 0
		case rc == 34:
			// repeat 0 for 11-138 times
			repeat := int(hr.readBits(7)) + 11
			for range repeat {
				if len(symCodeLens) >= numSyms {
					break
				}
				symCodeLens = append(symCodeLens, 0)
			}
			prevLen = 0
		default:
			return nil, fmt.Errorf("invalid RUNCODE %d", rc)
		}
	}
	if hr.err != nil {
		return nil, hr.err
	}

	// step 6: byte-align
	hr.align()

	// step 7: assign Huffman codes to symbols
	lines := make([]huffLine, numSyms)
	for i, l := range symCodeLens {
		lines[i] = huffLine{
			RangeLow: int32(i),
			PrefLen:  l,
			RangeLen: 0,
		}
	}
	t := &huffTable{Lines: lines}
	t.assignCodes()
	return t, nil
}
