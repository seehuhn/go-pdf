package cff

import (
	"fmt"
	"io"
	"sort"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
)

// FdSelectFn maps glyphID values to private dicts in Font.Info.Private.
type FdSelectFn func(font.GlyphID) int

func readFDSelect(p *parser.Parser, nGlyphs int) (FdSelectFn, error) {
	format, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	switch format {
	case 0:
		buf := make([]uint8, nGlyphs)
		_, err := io.ReadFull(p, buf)
		if err != nil {
			return nil, err
		}
		return func(gid font.GlyphID) int {
			return int(buf[gid])
		}, nil
	case 3:
		nRanges, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		}
		if nGlyphs > 0 && nRanges == 0 {
			return nil, invalidSince("no FDSelect data found")
		}

		var end []font.GlyphID
		var fdIdx []uint8

		prev := uint16(0)
		for i := 0; i < int(nRanges); i++ {
			first, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			} else if i > 0 && first <= prev || i == 0 && first != 0 {
				return nil, invalidSince("FDSelect is invalid")
			}
			fd, err := p.ReadUInt8()
			if err != nil {
				return nil, err
			}
			if i > 0 {
				end = append(end, font.GlyphID(first))
			}
			fdIdx = append(fdIdx, fd)
			prev = first
		}
		sentinel, err := p.ReadUInt16()
		if err != nil {
			return nil, err
		} else if int(sentinel) != nGlyphs {
			return nil, invalidSince("wrong FDSelect sentinel")
		}
		end = append(end, font.GlyphID(nGlyphs))

		return func(gid font.GlyphID) int {
			idx := sort.Search(int(nRanges),
				func(i int) bool { return gid < end[i] })
			return int(fdIdx[idx])
		}, nil
	default:
		return nil, notSupported(fmt.Sprintf("FDSelect format %d", format))
	}
}

func (fdSelect FdSelectFn) encode(nGlyphs int) []byte {
	format0Length := nGlyphs + 1

	buf := []byte{3, 0, 0}
	var currendFD int
	nSeg := 0
	for i := 0; i < nGlyphs; i++ {
		fd := fdSelect(font.GlyphID(i))
		if i > 0 && fd == currendFD {
			continue
		}
		// new segment
		if len(buf)+3+2 >= format0Length {
			goto useFormat0
		}
		buf = append(buf, byte(i>>8), byte(i), byte(fd))
		nSeg++
		currendFD = fd
	}
	buf = append(buf, byte(nGlyphs>>8), byte(nGlyphs))
	buf[1], buf[2] = byte(nSeg>>8), byte(nSeg)
	return buf

useFormat0:
	buf = make([]byte, nGlyphs+1)
	for i := 0; i < nGlyphs; i++ {
		buf[i+1] = byte(fdSelect(font.GlyphID(i)))
	}
	return buf
}
