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
	"errors"
	"fmt"
	"math/bits"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// decodeSymbolDictionary decodes a symbol dictionary segment.
// Returns the exported symbols.
func (d *decoder) decodeSymbolDictionary(hdr *segmentHeader, data []byte) ([]*bitmap.Bitmap, error) {
	if len(data) < 10 {
		return nil, fmt.Errorf("symbol dictionary data too short")
	}

	// parse flags (2 bytes, big-endian per JBIG2 convention)
	flags := binary.BigEndian.Uint16(data[0:2])
	sdhuff := flags&1 != 0
	sdrefagg := flags&2 != 0
	sdTemplate := int((flags >> 10) & 3)
	sdrTemplate := int((flags >> 12) & 1)

	offset := 2

	// AT flags (if not Huffman)
	var sdATX [4]int8
	var sdATY [4]int8
	if !sdhuff {
		var atBytes int
		if sdTemplate == 0 {
			atBytes = 8
		} else {
			atBytes = 2
		}
		if offset+atBytes > len(data) {
			return nil, fmt.Errorf("SD AT flags truncated")
		}
		for i := 0; i < atBytes/2; i++ {
			sdATX[i] = int8(data[offset+i*2])
			sdATY[i] = int8(data[offset+i*2+1])
		}
		offset += atBytes
	}

	// refinement AT flags
	var sdrATX [2]int8
	var sdrATY [2]int8
	if sdrefagg && sdrTemplate == 0 {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("SD refinement AT flags truncated")
		}
		sdrATX[0] = int8(data[offset])
		sdrATY[0] = int8(data[offset+1])
		sdrATX[1] = int8(data[offset+2])
		sdrATY[1] = int8(data[offset+3])
		offset += 4
	}

	// SDNUMEXSYMS and SDNUMNEWSYMS (4 bytes each)
	if offset+8 > len(data) {
		return nil, fmt.Errorf("SD counts truncated")
	}
	sdNumExSyms := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	sdNumNewSyms := int(binary.BigEndian.Uint32(data[offset+4 : offset+8]))
	offset += 8

	// each symbol needs at least a few bytes of encoded data
	if sdNumNewSyms > len(data) {
		return nil, fmt.Errorf("symbol count %d too large for %d bytes of data",
			sdNumNewSyms, len(data))
	}

	// collect input symbols from referred segments
	var inputSymbols []*bitmap.Bitmap
	for _, refNum := range hdr.RefSegments {
		if ref, ok := d.segments[refNum]; ok && ref.symbols != nil {
			inputSymbols = append(inputSymbols, ref.symbols...)
		}
	}
	sdNumInSyms := len(inputSymbols)

	if sdhuff {
		return d.decodeSymbolDictHuffman(hdr, data[offset:], sdNumInSyms, sdNumNewSyms, sdNumExSyms,
			inputSymbols, sdrefagg, sdrTemplate, sdrATX, sdrATY, flags)
	}

	// arithmetic symbol dictionary decoding
	dec := newMQDecoder(data[offset:])
	newSymbols := make([]*bitmap.Bitmap, sdNumNewSyms)

	iadh := &intCtx{}
	iadw := &intCtx{}
	iaai := &intCtx{}
	iaex := &intCtx{}
	iaid, err := newIAIDCtx(symCodeLen(sdNumInSyms + sdNumNewSyms))
	if err != nil {
		return nil, err
	}
	iardx := &intCtx{}
	iardy := &intCtx{}

	// generic region and refinement contexts persist across all symbols
	// within a dictionary (§7.4.2.2 steps 3-4, 7)
	gbCx := make([]byte, genericContextSize(sdTemplate))
	grCx := make([]byte, 1<<13)

	var hcHeight, symWidth int64
	nDecoded := 0
	emptyClasses := 0

	for nDecoded < sdNumNewSyms {
		// decode height class delta
		dh := iadh.decode(dec)
		if dh == oobResult {
			return nil, fmt.Errorf("unexpected OOB for height class delta")
		}
		hcHeight += dh
		symWidth = 0

		// decode symbols in this height class (§6.5.5.1 step 4c)
		// The inner loop runs until OOB is received. When SDREFAGG=1,
		// the T.88 reference encoder omits the trailing OOB, so we
		// check the count before attempting to read another DW.
		prevDecoded := nDecoded
		for {
			if sdrefagg && nDecoded >= sdNumNewSyms {
				break
			}
			dw := iadw.decode(dec)
			if dw == oobResult {
				break // end of height class
			}
			if nDecoded >= sdNumNewSyms {
				break // safety: don't overflow
			}
			symWidth += dw

			if symWidth <= 0 || hcHeight <= 0 {
				return nil, fmt.Errorf("invalid symbol dimensions: %dx%d", symWidth, hcHeight)
			}
			if symWidth > maxSymbolPixels || hcHeight > maxSymbolPixels ||
				symWidth*hcHeight > maxSymbolPixels {
				return nil, fmt.Errorf("symbol %dx%d exceeds maximum size", symWidth, hcHeight)
			}

			iSymWidth := int(symWidth)
			iHcHeight := int(hcHeight)

			if !sdrefagg {
				// direct bitmap coding
				p := &genericRegionParams{
					Width:    iSymWidth,
					Height:   iHcHeight,
					Template: sdTemplate,
				}
				copy(p.ATX[:], sdATX[:])
				copy(p.ATY[:], sdATY[:])
				var err error
				newSymbols[nDecoded], err = decodeGenericRegion(dec, p, gbCx)
				if err != nil {
					return nil, err
				}
			} else {
				// refinement/aggregate coding
				refAggNInst := iaai.decode(dec)
				if refAggNInst == 1 {
					// single-instance refinement
					codeLen := symCodeLen(sdNumInSyms + sdNumNewSyms)
					symID := iaid.decode(dec, codeLen)
					rdx := iardx.decode(dec)
					rdy := iardy.decode(dec)

					// look up reference symbol
					allSyms := make([]*bitmap.Bitmap, len(inputSymbols)+nDecoded)
					copy(allSyms, inputSymbols)
					copy(allSyms[len(inputSymbols):], newSymbols[:nDecoded])
					if symID >= len(allSyms) {
						return nil, fmt.Errorf("symbol ID %d out of range", symID)
					}
					refSym := allSyms[symID]

					rp := &refinementParams{
						Width:     iSymWidth,
						Height:    iHcHeight,
						Template:  sdrTemplate,
						Reference: refSym,
						RefDX:     int(rdx),
						RefDY:     int(rdy),
					}
					copy(rp.ATX[:], sdrATX[:])
					copy(rp.ATY[:], sdrATY[:])
					sym, err := decodeRefinementRegion(dec, rp, grCx)
					if err != nil {
						return nil, err
					}
					newSymbols[nDecoded] = sym
				} else {
					// multi-instance aggregation via text region
					// TODO: implement full text region for aggregation
					newSymbols[nDecoded] = bitmap.New(iSymWidth, iHcHeight)
				}
			}
			nDecoded++
		}

		// guard against malformed data producing endless empty
		// height classes without any symbols
		if nDecoded == prevDecoded {
			emptyClasses++
			if emptyClasses > 4 {
				return nil, errors.New("too many empty height classes")
			}
		} else {
			emptyClasses = 0
		}
	}

	// decode export flags
	if sdNumExSyms > sdNumInSyms+sdNumNewSyms {
		sdNumExSyms = sdNumInSyms + sdNumNewSyms
	}
	allSyms := make([]*bitmap.Bitmap, sdNumInSyms+sdNumNewSyms)
	copy(allSyms, inputSymbols)
	copy(allSyms[sdNumInSyms:], newSymbols)

	exported := decodeExportFlags(allSyms, sdNumExSyms, func() int {
		return int(iaex.decode(dec))
	})
	return exported, nil
}

// decodeExportFlags selects exported symbols from allSyms using
// run-length encoded export flags. decodeRun returns the next run
// length; it should return a negative value when the data is exhausted.
func decodeExportFlags(allSyms []*bitmap.Bitmap, cap int, decodeRun func() int) []*bitmap.Bitmap {
	exported := make([]*bitmap.Bitmap, 0, cap)
	exIdx := 0
	curFlag := 0
	zeroRuns := 0
	for exIdx < len(allSyms) {
		runLen := decodeRun()
		if runLen < 0 || exIdx+runLen > len(allSyms) {
			break
		}
		if runLen == 0 {
			zeroRuns++
			if zeroRuns > 4 {
				break // malformed data producing endless zero-length runs
			}
		} else {
			zeroRuns = 0
		}
		for i := 0; i < runLen && exIdx < len(allSyms); i++ {
			if curFlag != 0 {
				exported = append(exported, allSyms[exIdx])
			}
			exIdx++
		}
		curFlag ^= 1
	}
	return exported
}

func symCodeLen(n int) int {
	if n <= 1 {
		return 1
	}
	return bits.Len(uint(n - 1))
}

// decodeSymbolDictHuffman decodes a Huffman-coded symbol dictionary.
func (d *decoder) decodeSymbolDictHuffman(
	hdr *segmentHeader,
	data []byte,
	sdNumInSyms, sdNumNewSyms, sdNumExSyms int,
	inputSymbols []*bitmap.Bitmap,
	sdrefagg bool, sdrTemplate int,
	sdrATX [2]int8, sdrATY [2]int8,
	flags uint16,
) ([]*bitmap.Bitmap, error) {
	// select Huffman tables from flags
	dhSel := (flags >> 2) & 3
	dwSel := (flags >> 4) & 3
	bmSizeSel := (flags >> 6) & 1
	aggInstSel := (flags >> 7) & 1

	tableIdx := 0

	var dhTable, dwTable *huffTable
	switch dhSel {
	case 0:
		dhTable = huffTableB4
	case 1:
		dhTable = huffTableB5
	case 3:
		var err error
		dhTable, err = d.customTable(hdr.RefSegments, &tableIdx)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported SDHUFFDH selection %d", dhSel)
	}
	switch dwSel {
	case 0:
		dwTable = huffTableB2
	case 1:
		dwTable = huffTableB3
	case 3:
		var err error
		dwTable, err = d.customTable(hdr.RefSegments, &tableIdx)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported SDHUFFDW selection %d", dwSel)
	}

	var bmSizeTable *huffTable
	if bmSizeSel == 0 {
		bmSizeTable = huffTableB1
	} else {
		var err error
		bmSizeTable, err = d.customTable(hdr.RefSegments, &tableIdx)
		if err != nil {
			return nil, err
		}
	}

	// SDHUFFAGGINST: only used when sdrefagg is true
	var aggInstTable *huffTable
	if sdrefagg {
		if aggInstSel == 0 {
			aggInstTable = huffTableB1
		} else {
			var err error
			aggInstTable, err = d.customTable(hdr.RefSegments, &tableIdx)
			if err != nil {
				return nil, err
			}
		}
	}

	hr := newHuffReader(data)
	newSymbols := make([]*bitmap.Bitmap, sdNumNewSyms)
	newSymWidths := make([]int, sdNumNewSyms)

	var hcHeight int64
	nDecoded := 0

	for nDecoded < sdNumNewSyms {
		if hr.err != nil || hr.eof {
			break
		}

		// decode height class delta
		dh := dhTable.decode(hr)
		hcHeight += dh
		var symWidth, totWidth int64
		hcFirstSym := nDecoded

		// decode symbols in this height class
		gotOOB := false
		for nDecoded < sdNumNewSyms {
			dw := dwTable.decode(hr)
			if dw == oobResult {
				gotOOB = true
				break
			}
			symWidth += dw

			if symWidth <= 0 || hcHeight <= 0 {
				return nil, fmt.Errorf("invalid symbol dimensions: %dx%d", symWidth, hcHeight)
			}
			if symWidth > maxSymbolPixels || hcHeight > maxSymbolPixels ||
				symWidth*hcHeight > maxSymbolPixels {
				return nil, fmt.Errorf("symbol %dx%d exceeds maximum size", symWidth, hcHeight)
			}

			totWidth += symWidth
			newSymWidths[nDecoded] = int(symWidth)
			nDecoded++
		}
		// consume the trailing OOB if the loop exited by count
		if !gotOOB && !hr.eof {
			dwTable.decode(hr)
		}

		// guard against infinite loop: if a height class produced no
		// symbols, the outer loop would never terminate
		if nDecoded == hcFirstSym {
			return nil, errors.New("empty height class in Huffman symbol dictionary")
		}

		iHcHeight := int(hcHeight)
		iTotWidth := int(totWidth)

		if !sdrefagg {
			// decode collective bitmap (MMR or uncompressed)
			bmSize := int(bmSizeTable.decode(hr))
			if bmSize < 0 {
				return nil, fmt.Errorf("invalid bitmap size %d", bmSize)
			}
			hr.align()

			nSymsInClass := nDecoded - hcFirstSym
			if nSymsInClass == 0 {
				continue
			}

			// collective bitmap: totWidth * hcHeight
			if err := checkBitmapSize(iTotWidth, iHcHeight); err != nil {
				return nil, err
			}

			var collectiveBM *bitmap.Bitmap
			if bmSize == 0 {
				// uncompressed: read raw bytes
				rawStride := (iTotWidth + 7) / 8
				rawSize := iHcHeight * rawStride
				if hr.offset()+rawSize > len(data) {
					return nil, fmt.Errorf("uncompressed bitmap overflows: offset=%d size=%d totWidth=%d hcHeight=%d len=%d",
						hr.offset(), rawSize, iTotWidth, iHcHeight, len(data))
				}
				rawData := data[hr.offset() : hr.offset()+rawSize]
				collectiveBM = bitmap.New(iTotWidth, iHcHeight)
				for y := range iHcHeight {
					copy(collectiveBM.Pix[y*collectiveBM.Stride:], rawData[y*rawStride:(y+1)*rawStride])
				}
				// advance past raw data
				hr.bytePos = hr.offset() + rawSize
				hr.bitPos = 7
			} else {
				// MMR-coded collective bitmap
				if hr.offset()+bmSize > len(data) {
					return nil, fmt.Errorf("MMR bitmap overflows: offset=%d bmSize=%d len=%d",
						hr.offset(), bmSize, len(data))
				}
				mmrData := data[hr.offset() : hr.offset()+bmSize]
				var err error
				collectiveBM, _, err = decodeMMR(mmrData, iTotWidth, iHcHeight)
				if err != nil {
					return nil, err
				}
				hr.bytePos = hr.offset() + bmSize
				hr.bitPos = 7
			}

			// split collective bitmap into individual symbols
			parts := splitBitmapH(collectiveBM, newSymWidths[hcFirstSym:nDecoded])
			copy(newSymbols[hcFirstSym:], parts)
		} else {
			// refinement/aggregate coding (§6.5.8)
			codeLen := symCodeLen(sdNumInSyms + sdNumNewSyms)
			var grCx []byte
			if sdrTemplate == 0 {
				grCx = make([]byte, 1<<13)
			} else {
				grCx = make([]byte, 1<<10)
			}

			for i := hcFirstSym; i < nDecoded; i++ {
				iSymWidth := newSymWidths[i]

				// §6.5.8.2.1: decode number of symbol instances
				refAggNInst := int(aggInstTable.decode(hr))
				if refAggNInst < 1 {
					return nil, fmt.Errorf("invalid REFAGGNINST %d", refAggNInst)
				}

				if refAggNInst == 1 {
					// §6.5.8.2.2: single-instance refinement
					symID := int(hr.readBits(codeLen))
					rdx := int(huffTableB15.decode(hr))
					rdy := int(huffTableB15.decode(hr))

					// refinement bitmap data size (§6.5.8.2.2 step 5)
					bmSize := int(bmSizeTable.decode(hr))
					hr.align()

					// look up reference symbol
					allSyms := make([]*bitmap.Bitmap, len(inputSymbols)+i)
					copy(allSyms, inputSymbols)
					copy(allSyms[len(inputSymbols):], newSymbols[:i])
					if symID >= len(allSyms) {
						return nil, fmt.Errorf("symbol ID %d out of range", symID)
					}
					refSym := allSyms[symID]

					// decode refinement region from bounded data
					startOff := hr.offset()
					if bmSize < 0 || startOff+bmSize > len(data) {
						return nil, fmt.Errorf("refinement bitmap size %d overflows data", bmSize)
					}
					dec := newMQDecoder(data[startOff : startOff+bmSize])

					rp := &refinementParams{
						Width:     iSymWidth,
						Height:    iHcHeight,
						Template:  sdrTemplate,
						Reference: refSym,
						RefDX:     rdx,
						RefDY:     rdy,
					}
					copy(rp.ATX[:], sdrATX[:])
					copy(rp.ATY[:], sdrATY[:])
					sym, err := decodeRefinementRegion(dec, rp, grCx)
					if err != nil {
						return nil, err
					}
					newSymbols[i] = sym

					// §6.5.8.2.2 step 7: advance past the refinement data
					hr.bytePos = startOff + bmSize
					hr.bitPos = 7
				} else {
					// multi-instance aggregation via text region
					// TODO: implement full text region for aggregation
					newSymbols[i] = bitmap.New(iSymWidth, iHcHeight)
				}
			}
		}
	}

	if hr.err != nil {
		return nil, hr.err
	}

	// decode export flags using Table B.1
	if sdNumExSyms > sdNumInSyms+sdNumNewSyms {
		sdNumExSyms = sdNumInSyms + sdNumNewSyms
	}
	allSyms := make([]*bitmap.Bitmap, sdNumInSyms+sdNumNewSyms)
	copy(allSyms, inputSymbols)
	copy(allSyms[sdNumInSyms:], newSymbols)

	exported := decodeExportFlags(allSyms, sdNumExSyms, func() int {
		if hr.eof {
			return -1
		}
		return int(huffTableB1.decode(hr))
	})
	if hr.err != nil {
		return nil, hr.err
	}

	return exported, nil
}
