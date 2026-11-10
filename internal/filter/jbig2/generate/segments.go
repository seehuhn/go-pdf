// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// segment types used by the generator
const (
	segSymbolDict       = 0
	segImmediateText    = 6
	segImmediateGeneric = 38
	segLosslessGeneric  = 39
	segPageInfo         = 48
	segEndOfPage        = 49
)

var segTypeNames = map[int]string{
	0:  "SymbolDictionary",
	6:  "TextRegion_Immediate",
	7:  "TextRegion_ImmediateLossless",
	38: "GenericRegion_Immediate",
	39: "GenericRegion_ImmediateLossless",
	48: "PageInformation",
	49: "EndOfPage",
}

// A segment is one JBIG2 segment of an embedded stream (ITU-T T.88,
// section 7.2), with the header fields decoded.  The flag bits which the
// generator does not interpret are kept, so that a parsed segment is
// written back unchanged.
type segment struct {
	number uint32
	typ    int
	refs   []uint32
	page   uint32
	data   []byte

	deferredNonRetain bool // bit 7 of the segment header flags
	pageLong          bool // 4-byte page association field
	retainBits        byte // the low 5 bits of the referred-to segment count byte
}

// parseSegments splits a sequential-organisation stream without file
// header into its segments.
func parseSegments(buf []byte) ([]*segment, error) {
	var segs []*segment
	be := binary.BigEndian
	p := 0
	for p < len(buf) {
		if p+6 > len(buf) {
			return nil, errors.New("truncated segment header")
		}
		flags := buf[p+4]
		s := &segment{
			number:            be.Uint32(buf[p:]),
			typ:               int(flags & 0x3F),
			deferredNonRetain: flags&0x80 != 0,
			pageLong:          flags&0x40 != 0,
		}
		p += 5

		count := int(buf[p] >> 5)
		if count > 4 {
			return nil, errors.New("long form referred-to segment count not supported")
		}
		s.retainBits = buf[p] & 0x1F
		p++
		refSize := s.refSize()
		for range count {
			if p+refSize > len(buf) {
				return nil, errors.New("truncated segment header")
			}
			var r uint32
			for i := range refSize {
				r = r<<8 | uint32(buf[p+i])
			}
			s.refs = append(s.refs, r)
			p += refSize
		}

		if s.pageLong {
			if p+4 > len(buf) {
				return nil, errors.New("truncated segment header")
			}
			s.page = be.Uint32(buf[p:])
			p += 4
		} else {
			if p+1 > len(buf) {
				return nil, errors.New("truncated segment header")
			}
			s.page = uint32(buf[p])
			p++
		}

		if p+4 > len(buf) {
			return nil, errors.New("truncated segment header")
		}
		length := be.Uint32(buf[p:])
		p += 4
		if length == 0xFFFFFFFF || int(length) > len(buf)-p {
			return nil, fmt.Errorf("segment %d: unsupported data length", s.number)
		}
		s.data = buf[p : p+int(length)]
		p += int(length)

		segs = append(segs, s)
	}
	return segs, nil
}

// refSize returns the size of the referred-to segment numbers in the
// header (T.88, section 7.2.5).
func (s *segment) refSize() int {
	switch {
	case s.number > 65536:
		return 4
	case s.number > 256:
		return 2
	default:
		return 1
	}
}

// longPage reports whether the header uses the 4-byte page association
// field.
func (s *segment) longPage() bool {
	return s.pageLong || s.page > 255
}

// headerLen returns the length of the encoded segment header.
func (s *segment) headerLen() int {
	n := 4 + 1 + 1 + 4
	n += s.refSize() * len(s.refs)
	if s.longPage() {
		n += 4
	} else {
		n++
	}
	return n
}

// appendSegment appends the encoded segment to buf.
func appendSegment(buf []byte, s *segment) []byte {
	if len(s.refs) > 4 {
		panic("too many referred-to segments")
	}
	buf = binary.BigEndian.AppendUint32(buf, s.number)
	flags := byte(s.typ)
	if s.deferredNonRetain {
		flags |= 0x80
	}
	if s.longPage() {
		flags |= 0x40
	}
	buf = append(buf, flags, byte(len(s.refs)<<5)|s.retainBits&0x1F)
	refSize := s.refSize()
	for _, r := range s.refs {
		for i := refSize - 1; i >= 0; i-- {
			buf = append(buf, byte(r>>(8*i)))
		}
	}
	if s.longPage() {
		buf = binary.BigEndian.AppendUint32(buf, s.page)
	} else {
		buf = append(buf, byte(s.page))
	}
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(s.data)))
	return append(buf, s.data...)
}

func encodeSegments(segs []*segment) []byte {
	var buf []byte
	for _, s := range segs {
		buf = appendSegment(buf, s)
	}
	return buf
}

// pageInfoData returns the data of a page information segment
// (T.88, section 7.4.8) for a page of known height without striping.
func pageInfoData(w, h int, flags byte) []byte {
	data := make([]byte, 19)
	binary.BigEndian.PutUint32(data[0:], uint32(w))
	binary.BigEndian.PutUint32(data[4:], uint32(h))
	data[16] = flags
	return data
}

// regionInfoData returns a region segment information field (T.88,
// section 7.4.1).
func regionInfoData(w, h, x, y int, combOp byte) []byte {
	data := make([]byte, 17)
	binary.BigEndian.PutUint32(data[0:], uint32(w))
	binary.BigEndian.PutUint32(data[4:], uint32(h))
	binary.BigEndian.PutUint32(data[8:], uint32(x))
	binary.BigEndian.PutUint32(data[12:], uint32(y))
	data[16] = combOp
	return data
}

// appendEndOfPage appends an end of page segment for the page of the
// stream's page information segment.  The new segment takes the next free
// segment number.
func appendEndOfPage(segs []*segment) ([]*segment, error) {
	pi, err := findSegment(segs, segPageInfo)
	if err != nil {
		return nil, err
	}
	var next uint32
	for _, s := range segs {
		if s.typ == segEndOfPage {
			return nil, errors.New("stream already has an end of page segment")
		}
		next = max(next, s.number+1)
	}
	return append(segs, &segment{number: next, typ: segEndOfPage, page: pi.page}), nil
}

// findSegment returns the only segment of the given type.
func findSegment(segs []*segment, typ int) (*segment, error) {
	var res *segment
	for _, s := range segs {
		if s.typ == typ {
			if res != nil {
				return nil, fmt.Errorf("more than one segment of type %d", typ)
			}
			res = s
		}
	}
	if res == nil {
		return nil, fmt.Errorf("no segment of type %d", typ)
	}
	return res, nil
}
