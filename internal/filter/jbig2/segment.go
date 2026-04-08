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
	"io"

	"seehuhn.de/go/pdf/graphics/bitmap"
)

// segment types
const (
	segSymbolDict               = 0
	segIntermediateTextRegion   = 4
	segImmediateTextRegion      = 6
	segImmediateLosslessText    = 7
	segPatternDict              = 16
	segIntermediateHalftone     = 20
	segImmediateHalftone        = 22
	segImmediateLosslessHalf    = 23
	segIntermediateGeneric      = 36
	segImmediateGeneric         = 38
	segImmediateLosslessGeneric = 39
	segIntermediateRefinement   = 40
	segImmediateRefinement      = 42
	segImmediateLosslessRefine  = 43
	segPageInfo                 = 48
	segEndOfPage                = 49
	segEndOfStripe              = 50
	segEndOfFile                = 51
	segProfiles                 = 52
	segTables                   = 53
	segExtension                = 62
)

// segmentHeader represents a parsed JBIG2 segment header.
type segmentHeader struct {
	Number      uint32
	Type        int
	PageAssoc   uint32
	RefSegments []uint32
	DataLength  uint32 // 0xFFFFFFFF = unknown
}

// parseSegmentHeader parses a segment header from r.
func parseSegmentHeader(r io.Reader) (*segmentHeader, error) {
	// segment number (4 bytes big-endian)
	var numBuf [4]byte
	if _, err := io.ReadFull(r, numBuf[:]); err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("segment number: %w", err)
	}
	h := &segmentHeader{
		Number: binary.BigEndian.Uint32(numBuf[:]),
	}

	// header flags (1 byte)
	var flags [1]byte
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return nil, fmt.Errorf("segment flags: %w", err)
	}
	h.Type = int(flags[0] & 0x3F)
	pageAssocLarge := flags[0]&0x40 != 0

	// referred-to count and retention flags
	var countBuf [1]byte
	if _, err := io.ReadFull(r, countBuf[:]); err != nil {
		return nil, fmt.Errorf("segment ref count: %w", err)
	}

	refCount := int(countBuf[0] >> 5)
	if refCount == 7 {
		// long form: read 3 more bytes for the full count
		var longBuf [3]byte
		if _, err := io.ReadFull(r, longBuf[:]); err != nil {
			return nil, fmt.Errorf("segment long ref count: %w", err)
		}
		refCount = int(countBuf[0]&0x1F)<<24 | int(longBuf[0])<<16 | int(longBuf[1])<<8 | int(longBuf[2])
		if refCount > maxRefCount {
			return nil, fmt.Errorf("segment ref count %d exceeds limit", refCount)
		}
		// skip retention flag bytes
		retBytes := (refCount + 8) / 8
		if _, err := io.ReadFull(r, make([]byte, retBytes)); err != nil {
			return nil, fmt.Errorf("segment retention flags: %w", err)
		}
	}

	// referred-to segment numbers
	var refSize int
	if h.Number <= 256 {
		refSize = 1
	} else if h.Number <= 65536 {
		refSize = 2
	} else {
		refSize = 4
	}

	h.RefSegments = make([]uint32, refCount)
	for i := 0; i < refCount; i++ {
		buf := make([]byte, refSize)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, fmt.Errorf("segment ref %d: %w", i, err)
		}
		switch refSize {
		case 1:
			h.RefSegments[i] = uint32(buf[0])
		case 2:
			h.RefSegments[i] = uint32(binary.BigEndian.Uint16(buf))
		case 4:
			h.RefSegments[i] = binary.BigEndian.Uint32(buf)
		}
	}

	// page association
	if pageAssocLarge {
		var paBuf [4]byte
		if _, err := io.ReadFull(r, paBuf[:]); err != nil {
			return nil, fmt.Errorf("segment page assoc: %w", err)
		}
		h.PageAssoc = binary.BigEndian.Uint32(paBuf[:])
	} else {
		var paBuf [1]byte
		if _, err := io.ReadFull(r, paBuf[:]); err != nil {
			return nil, fmt.Errorf("segment page assoc: %w", err)
		}
		h.PageAssoc = uint32(paBuf[0])
	}

	// data length (4 bytes)
	var dlBuf [4]byte
	if _, err := io.ReadFull(r, dlBuf[:]); err != nil {
		return nil, fmt.Errorf("segment data length: %w", err)
	}
	h.DataLength = binary.BigEndian.Uint32(dlBuf[:])

	return h, nil
}

// regionSegmentInfo holds the common fields from the region segment
// information field (Section 7.4.1).
type regionSegmentInfo struct {
	Width  uint32
	Height uint32
	X      uint32
	Y      uint32
	CombOp bitmap.CombOp
}

// parseRegionSegmentInfo parses the 17-byte region segment information field.
func parseRegionSegmentInfo(data []byte) regionSegmentInfo {
	return regionSegmentInfo{
		Width:  binary.BigEndian.Uint32(data[0:4]),
		Height: binary.BigEndian.Uint32(data[4:8]),
		X:      binary.BigEndian.Uint32(data[8:12]),
		Y:      binary.BigEndian.Uint32(data[12:16]),
		CombOp: bitmap.CombOp(data[16] & 0x07),
	}
}
