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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

const (
	// maximum total pixels (width * height) for a page or region bitmap
	// (300 dpi A4 ≈ 8.7M pixels; 16M gives comfortable headroom)
	maxPixels = 1 << 24

	// maximum total pixels for a single symbol bitmap
	// (largest practical symbol: ~100x100 at 300dpi)
	maxSymbolPixels = 1 << 14

	// maximum ratio of output pixel bytes to input stream bytes
	maxExpansion = 8192

	// maximum IAID code length (limits context array to 1 MB)
	maxIAIDCodeLen = 20

	// maximum number of referred-to segments in a segment header
	maxRefCount = 1 << 16
)

// checkBitmapSize returns an error if dimensions are negative or if
// width*height exceeds maxPixels.
func checkBitmapSize(width, height int) error {
	if width < 0 || height < 0 {
		return fmt.Errorf("negative bitmap dimensions: %d x %d", width, height)
	}
	if width == 0 || height == 0 {
		return nil
	}
	if int64(width)*int64(height) > maxPixels {
		return fmt.Errorf("bitmap too large: %d x %d", width, height)
	}
	return nil
}

// Decode decodes a JBIG2 image from PDF globals and page streams.
// The globals stream may be nil if there are no global segments.
func Decode(globals, page []byte) (*bitmap.Bitmap, error) {
	d := &decoder{
		segments:         make(map[uint32]segmentResult),
		inputSize:        len(globals) + len(page),
		prescannedHeight: prescanPageHeight(page),
	}

	// parse global segments (page association 0)
	if len(globals) > 0 {
		if err := d.processStream(globals); err != nil {
			return nil, fmt.Errorf("globals: %w", err)
		}
	}

	// parse page segments
	if err := d.processStream(page); err != nil {
		return nil, fmt.Errorf("page: %w", err)
	}

	if d.pageBitmap == nil {
		return nil, errors.New("no page bitmap produced")
	}
	return d.pageBitmap, nil
}

// prescanPageHeight scans segment headers to find the last
// end-of-stripe segment and returns its Y coordinate + 1.
// Returns -1 if no end-of-stripe segment is found.
func prescanPageHeight(data []byte) int {
	lastY := -1
	r := bytes.NewReader(data)
	for {
		hdr, err := parseSegmentHeader(r)
		if err != nil {
			break
		}
		dataLen := hdr.DataLength
		if dataLen == 0xFFFFFFFF {
			break
		}
		if int(dataLen) > r.Len() {
			break
		}
		if hdr.Type == segEndOfStripe && dataLen >= 4 {
			var yBuf [4]byte
			if _, err := r.Read(yBuf[:]); err != nil {
				break
			}
			y := int(binary.BigEndian.Uint32(yBuf[:]))
			lastY = y + 1
			// skip remaining data
			if dataLen > 4 {
				r.Seek(int64(dataLen-4), io.SeekCurrent)
			}
		} else {
			r.Seek(int64(dataLen), io.SeekCurrent)
		}
	}
	return lastY
}

type segmentResult struct {
	header   *segmentHeader
	symbols  []*bitmap.Bitmap // for symbol dictionary segments
	patterns []*bitmap.Bitmap // for pattern dictionary segments
	bm       *bitmap.Bitmap   // for region segments
	table    *huffTable       // for custom Huffman table segments (type 53)
}

type decoder struct {
	segments         map[uint32]segmentResult
	pageBitmap       *bitmap.Bitmap
	pageWidth        int
	pageHeight       int
	inputSize        int // total input bytes (globals + page)
	prescannedHeight int // from prescanPageHeight; -1 if not available
}

func (d *decoder) processStream(data []byte) error {
	r := bytes.NewReader(data)
	for {
		hdr, err := parseSegmentHeader(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// read segment data
		dataLen := hdr.DataLength
		if dataLen == 0xFFFFFFFF {
			dataLen = uint32(r.Len())
		}
		if int(dataLen) > r.Len() {
			return fmt.Errorf("segment %d: data length %d exceeds remaining %d bytes",
				hdr.Number, dataLen, r.Len())
		}
		segData := make([]byte, dataLen)
		if _, err := io.ReadFull(r, segData); err != nil {
			return fmt.Errorf("segment %d data: %w", hdr.Number, err)
		}

		if err := d.processSegment(hdr, segData); err != nil {
			return fmt.Errorf("segment %d (type %d): %w", hdr.Number, hdr.Type, err)
		}
	}
	return nil
}

func (d *decoder) processSegment(hdr *segmentHeader, data []byte) error {
	switch hdr.Type {
	case segPageInfo:
		return d.processPageInfo(data)
	case segEndOfPage:
		return nil
	case segIntermediateGeneric, segImmediateGeneric, segImmediateLosslessGeneric:
		return d.processGenericRegion(hdr, data)
	case segIntermediateRefinement, segImmediateRefinement, segImmediateLosslessRefine:
		return d.processRefinementRegion(hdr, data)
	case segSymbolDict:
		return d.processSymbolDict(hdr, data)
	case segPatternDict:
		return d.processPatternDict(hdr, data)
	case segIntermediateTextRegion, segImmediateTextRegion, segImmediateLosslessText:
		return d.processTextRegion(hdr, data)
	case segIntermediateHalftone, segImmediateHalftone, segImmediateLosslessHalf:
		return d.processHalftoneRegion(hdr, data)
	case segTables:
		return d.processCustomTable(hdr, data)
	case segEndOfFile, segEndOfStripe, segProfiles, segExtension:
		return nil
	default:
		// skip unknown segment types
		return nil
	}
}

func (d *decoder) processPageInfo(data []byte) error {
	if len(data) < 19 {
		return fmt.Errorf("page info too short: %d bytes", len(data))
	}
	width := binary.BigEndian.Uint32(data[0:4])
	height := binary.BigEndian.Uint32(data[4:8])
	// bytes 8-11: X resolution
	// bytes 12-15: Y resolution
	flags := data[16]
	// byte 17-18: striping info

	d.pageWidth = int(width)
	if height == 0xFFFFFFFF {
		// unknown page height: use pre-scanned height from
		// end-of-stripe segments
		if d.prescannedHeight <= 0 {
			return errors.New("unknown page height without end-of-stripe segments")
		}
		d.pageHeight = d.prescannedHeight
	} else {
		d.pageHeight = int(height)
	}

	// default pixel value (bit 2 of flags)
	defaultPixel := flags&0x04 != 0

	if d.pageHeight > 0 {
		if err := checkBitmapSize(d.pageWidth, d.pageHeight); err != nil {
			return err
		}
		pixelBytes := int64(d.pageWidth+7) / 8 * int64(d.pageHeight)
		if d.inputSize > 0 && pixelBytes > int64(d.inputSize)*maxExpansion {
			return errors.New("page bitmap too large for input size")
		}
		d.pageBitmap = bitmap.New(d.pageWidth, d.pageHeight)
		if defaultPixel { // fill with 1 bits (black)
			for i := range d.pageBitmap.Pix {
				d.pageBitmap.Pix[i] = 0xFF
			}
		}
	}
	return nil
}

func (d *decoder) processCustomTable(hdr *segmentHeader, data []byte) error {
	t, err := parseCustomHuffTable(data)
	if err != nil {
		return err
	}
	d.segments[hdr.Number] = segmentResult{header: hdr, table: t}
	return nil
}

// customTable returns the next custom Huffman table from the referred-to
// segment list, advancing the index past non-table segments.
func (d *decoder) customTable(refs []uint32, idx *int) (*huffTable, error) {
	for *idx < len(refs) {
		ref := refs[*idx]
		*idx++
		seg, ok := d.segments[ref]
		if ok && seg.table != nil {
			return seg.table, nil
		}
	}
	return nil, errors.New("missing custom Huffman table segment")
}

func (d *decoder) processGenericRegion(hdr *segmentHeader, data []byte) error {
	if len(data) < 18 {
		return errors.New("generic region data too short")
	}

	// region segment information field (17 bytes)
	rsi := parseRegionSegmentInfo(data[:17])

	// generic region flags (1 byte)
	flags := data[17]
	mmr := flags&0x01 != 0
	gbTemplate := int((flags >> 1) & 0x03)
	tpgdon := flags&0x08 != 0
	extTemplate := flags&0x10 != 0

	offset := 18

	p := &genericRegionParams{
		MMR:         mmr,
		Width:       int(rsi.Width),
		Height:      int(rsi.Height),
		Template:    gbTemplate,
		TPGDON:      tpgdon,
		ExtTemplate: extTemplate,
	}

	if !mmr {
		// read AT flags
		var atBytes int
		switch {
		case gbTemplate == 0 && extTemplate:
			atBytes = 24
		case gbTemplate == 0:
			atBytes = 8
		default:
			atBytes = 2
		}
		if offset+atBytes > len(data) {
			return errors.New("AT flags truncated")
		}
		for i := 0; i < atBytes/2; i++ {
			p.ATX[i] = int8(data[offset+i*2])
			p.ATY[i] = int8(data[offset+i*2+1])
		}
		offset += atBytes
	}

	// decode the bitmap
	if err := checkBitmapSize(p.Width, p.Height); err != nil {
		return err
	}
	pixelBytes := int64(p.Width+7) / 8 * int64(p.Height)
	if d.inputSize > 0 && pixelBytes > int64(d.inputSize)*maxExpansion {
		return fmt.Errorf("generic region %dx%d too large for %d bytes of input",
			p.Width, p.Height, d.inputSize)
	}
	var bm *bitmap.Bitmap
	var err error
	if mmr {
		bm, _, err = decodeMMR(data[offset:], p.Width, p.Height)
		if err != nil {
			return err
		}
	} else {
		dec := newMQDecoder(data[offset:])
		bm, err = decodeGenericRegion(dec, p, nil)
		if err != nil {
			return err
		}
	}

	// composite onto page (skip for intermediate segments)
	if hdr.Type != segIntermediateGeneric && d.pageBitmap != nil {
		d.pageBitmap.Combine(bm, int(rsi.X), int(rsi.Y), rsi.CombOp)
	}

	// store for potential reference
	d.segments[hdr.Number] = segmentResult{header: hdr, bm: bm}
	return nil
}

func (d *decoder) processRefinementRegion(hdr *segmentHeader, data []byte) error {
	if len(data) < 18 {
		return errors.New("refinement region data too short")
	}

	rsi := parseRegionSegmentInfo(data[:17])

	flags := data[17]
	grTemplate := int(flags & 1)
	tpgron := flags&0x02 != 0

	offset := 18
	var atx, aty [2]int8
	if grTemplate == 0 {
		if len(data) < 22 {
			return errors.New("refinement AT flags truncated")
		}
		atx[0] = int8(data[18])
		aty[0] = int8(data[19])
		atx[1] = int8(data[20])
		aty[1] = int8(data[21])
		offset = 22
	}

	if err := checkBitmapSize(int(rsi.Width), int(rsi.Height)); err != nil {
		return err
	}
	pixelBytes := int64(rsi.Width+7) / 8 * int64(rsi.Height)
	if d.inputSize > 0 && pixelBytes > int64(d.inputSize)*maxExpansion {
		return fmt.Errorf("refinement region %dx%d too large for %d bytes of input",
			rsi.Width, rsi.Height, d.inputSize)
	}

	// obtain reference bitmap
	var ref *bitmap.Bitmap
	if len(hdr.RefSegments) > 0 {
		// case d: reference from a referred-to segment
		seg, ok := d.segments[hdr.RefSegments[0]]
		if !ok || seg.bm == nil {
			return fmt.Errorf("refinement reference segment %d not found", hdr.RefSegments[0])
		}
		ref = seg.bm
	} else {
		// case c: reference from page buffer
		if d.pageBitmap == nil {
			return errors.New("refinement region: no page buffer")
		}
		w := int(rsi.Width)
		h := int(rsi.Height)
		x0 := int(rsi.X)
		y0 := int(rsi.Y)
		ref = bitmap.New(w, h)
		for py := range h {
			for px := range w {
				ref.SetPixel(px, py, d.pageBitmap.GetPixel(x0+px, y0+py))
			}
		}
	}

	p := &refinementParams{
		Width:     int(rsi.Width),
		Height:    int(rsi.Height),
		Template:  grTemplate,
		TPGRON:    tpgron,
		Reference: ref,
	}
	copy(p.ATX[:], atx[:])
	copy(p.ATY[:], aty[:])

	dec := newMQDecoder(data[offset:])
	bm, err := decodeRefinementRegion(dec, p, nil)
	if err != nil {
		return err
	}

	// intermediate refinement: store result for later reference, no compositing
	if hdr.Type != segIntermediateRefinement && d.pageBitmap != nil {
		if len(hdr.RefSegments) > 0 {
			// case d: composite using combOp
			d.pageBitmap.Combine(bm, int(rsi.X), int(rsi.Y), rsi.CombOp)
		} else {
			// case c: replace the page buffer region
			d.pageBitmap.Combine(bm, int(rsi.X), int(rsi.Y), bitmap.CombOpReplace)
		}
	}

	d.segments[hdr.Number] = segmentResult{header: hdr, bm: bm}
	return nil
}

func (d *decoder) processSymbolDict(hdr *segmentHeader, data []byte) error {
	symbols, err := d.decodeSymbolDictionary(hdr, data)
	if err != nil {
		return err
	}
	d.segments[hdr.Number] = segmentResult{header: hdr, symbols: symbols}
	return nil
}

func (d *decoder) processTextRegion(hdr *segmentHeader, data []byte) error {
	if len(data) < 17+2 {
		return errors.New("text region data too short")
	}

	// region segment information field (17 bytes)
	rsi := parseRegionSegmentInfo(data[:17])

	// text region flags (2 bytes, big-endian)
	flags := binary.BigEndian.Uint16(data[17:19])
	sbhuff := flags&1 != 0
	sbrefine := flags&2 != 0
	sbstrips := 1 << ((flags >> 2) & 3)
	refCorner := int((flags >> 4) & 3)
	transposed := flags&0x40 != 0
	combOp := bitmap.CombOp((flags >> 7) & 3)
	defPixel := int((flags >> 9) & 1)
	sbdsOffset := int((flags >> 10) & 0x1F)
	if sbdsOffset >= 16 {
		sbdsOffset -= 32 // sign extend 5-bit
	}
	sbrTemplate := int((flags >> 15) & 1)

	offset := 19

	// Huffman tags field (2 bytes, only present if SBHUFF=1)
	var huffFS, huffDS, huffDT *huffTable
	var huffRDW, huffRDH, huffRDX, huffRDY, huffRSIZE *huffTable
	if sbhuff {
		if offset+2 > len(data) {
			return errors.New("text region Huffman tags truncated")
		}
		htags := binary.BigEndian.Uint16(data[offset : offset+2])
		offset += 2

		// custom table index: tracks which referred-to table segment
		// to use next when a parameter selects value 3 (user-supplied)
		tableIdx := 0

		switch htags & 3 {
		case 0:
			huffFS = huffTableB6
		case 1:
			huffFS = huffTableB7
		case 3:
			var err error
			huffFS, err = d.customTable(hdr.RefSegments, &tableIdx)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported SBHUFFFS selection %d", htags&3)
		}
		switch (htags >> 2) & 3 {
		case 0:
			huffDS = huffTableB8
		case 1:
			huffDS = huffTableB9
		case 2:
			huffDS = huffTableB10
		case 3:
			var err error
			huffDS, err = d.customTable(hdr.RefSegments, &tableIdx)
			if err != nil {
				return err
			}
		}
		switch (htags >> 4) & 3 {
		case 0:
			huffDT = huffTableB11
		case 1:
			huffDT = huffTableB12
		case 2:
			huffDT = huffTableB13
		case 3:
			var err error
			huffDT, err = d.customTable(hdr.RefSegments, &tableIdx)
			if err != nil {
				return err
			}
		}

		if sbrefine {
			selectRefTable := func(sel uint16) (*huffTable, error) {
				switch sel {
				case 0:
					return huffTableB14, nil
				case 1:
					return huffTableB15, nil
				case 3:
					return d.customTable(hdr.RefSegments, &tableIdx)
				default:
					return nil, fmt.Errorf("unsupported refinement table selection %d", sel)
				}
			}
			var err error
			huffRDW, err = selectRefTable((htags >> 6) & 3)
			if err != nil {
				return err
			}
			huffRDH, err = selectRefTable((htags >> 8) & 3)
			if err != nil {
				return err
			}
			huffRDX, err = selectRefTable((htags >> 10) & 3)
			if err != nil {
				return err
			}
			huffRDY, err = selectRefTable((htags >> 12) & 3)
			if err != nil {
				return err
			}
			switch (htags >> 14) & 1 {
			case 0:
				huffRSIZE = huffTableB1
			case 1:
				huffRSIZE, err = d.customTable(hdr.RefSegments, &tableIdx)
				if err != nil {
					return err
				}
			}
		}
	}

	// refinement AT flags
	var ratx [2]int8
	var raty [2]int8
	if sbrefine && sbrTemplate == 0 {
		if offset+4 > len(data) {
			return errors.New("text region refinement AT truncated")
		}
		ratx[0] = int8(data[offset])
		raty[0] = int8(data[offset+1])
		ratx[1] = int8(data[offset+2])
		raty[1] = int8(data[offset+3])
		offset += 4
	}

	// number of instances (4 bytes)
	if offset+4 > len(data) {
		return errors.New("text region num instances truncated")
	}
	numInstances := int(binary.BigEndian.Uint32(data[offset : offset+4]))
	offset += 4

	// each instance needs at least a few bits of encoded data
	if numInstances > len(data)*8 {
		return fmt.Errorf("text region: %d instances too large for %d bytes of data",
			numInstances, len(data))
	}

	// collect symbols from referred segments, skipping SDs whose
	// exports are already included by a later SD in the ref list
	// (an SD that refers to earlier SDs re-exports their symbols)
	refSet := make(map[uint32]bool)
	for _, refNum := range hdr.RefSegments {
		refSet[refNum] = true
	}
	var symbols []*bitmap.Bitmap
	for _, refNum := range hdr.RefSegments {
		ref, ok := d.segments[refNum]
		if !ok || ref.symbols == nil {
			continue
		}
		// skip this SD if a later referenced SD already refers to it
		subsumed := false
		if ref.header != nil && ref.header.Type == segSymbolDict {
			for _, laterRef := range hdr.RefSegments {
				if laterRef <= refNum {
					continue
				}
				if laterSeg, ok := d.segments[laterRef]; ok && laterSeg.header != nil {
					if slices.Contains(laterSeg.header.RefSegments, refNum) {
						subsumed = true
					}
				}
				if subsumed {
					break
				}
			}
		}
		if !subsumed {
			symbols = append(symbols, ref.symbols...)
		}
	}

	var bm *bitmap.Bitmap
	if sbhuff {
		// Huffman text region
		hr := newHuffReader(data[offset:])
		symIDTable, err := decodeSymIDHuffTable(hr, len(symbols))
		if err != nil {
			return err
		}

		hp := &textRegionHuffParams{
			Width:        int(rsi.Width),
			Height:       int(rsi.Height),
			NumInstances: numInstances,
			Strips:       sbstrips,
			Symbols:      symbols,
			DefPixel:     defPixel,
			CombOp:       combOp,
			Transposed:   transposed,
			RefCorner:    refCorner,
			DSOffset:     sbdsOffset,
			SBRefine:     sbrefine,
			RTemplate:    sbrTemplate,
			FSTable:      huffFS,
			DSTable:      huffDS,
			DTTable:      huffDT,
			RDWTable:     huffRDW,
			RDHTable:     huffRDH,
			RDXTable:     huffRDX,
			RDYTable:     huffRDY,
			RSIZETable:   huffRSIZE,
			SymIDTable:   symIDTable,
		}
		copy(hp.RATX[:], ratx[:])
		copy(hp.RATY[:], raty[:])

		bm, err = decodeTextRegionHuffman(hr, hp)
		if err != nil {
			return err
		}
	} else {
		// arithmetic text region
		p := &textRegionParams{
			SBHuff:       sbhuff,
			SBRefine:     sbrefine,
			Width:        int(rsi.Width),
			Height:       int(rsi.Height),
			NumInstances: numInstances,
			Strips:       sbstrips,
			Symbols:      symbols,
			SymCodeLen:   textRegionSymCodeLen(len(symbols)),
			DefPixel:     defPixel,
			CombOp:       combOp,
			Transposed:   transposed,
			RefCorner:    refCorner,
			DSOffset:     sbdsOffset,
			RTemplate:    sbrTemplate,
		}
		copy(p.RATX[:], ratx[:])
		copy(p.RATY[:], raty[:])

		dec := newMQDecoder(data[offset:])
		var err error
		bm, err = decodeTextRegion(dec, p)
		if err != nil {
			return err
		}
	}

	// composite onto page (skip for intermediate segments)
	if hdr.Type != segIntermediateTextRegion && d.pageBitmap != nil {
		d.pageBitmap.Combine(bm, int(rsi.X), int(rsi.Y), rsi.CombOp)
	}

	d.segments[hdr.Number] = segmentResult{header: hdr, bm: bm}
	return nil
}

// splitBitmapH splits a collective bitmap horizontally into sub-bitmaps
// with the given widths. All sub-bitmaps have the same height as src.
func splitBitmapH(src *bitmap.Bitmap, widths []int) []*bitmap.Bitmap {
	result := make([]*bitmap.Bitmap, len(widths))
	xOff := 0
	for i, w := range widths {
		sub := bitmap.New(w, src.Height())
		for y := range src.Height() {
			for x := range w {
				sub.SetPixel(x, y, src.GetPixel(xOff+x, y))
			}
		}
		result[i] = sub
		xOff += w
	}
	return result
}
