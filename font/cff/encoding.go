package cff

import (
	"errors"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/parser"
	"seehuhn.de/go/pdf/font/type1"
)

func readEncoding(p *parser.Parser, charset []int32) ([]font.GlyphID, error) {
	format, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}
	hasSupplement := (format & 128) != 0
	format &= 127

	res := make([]font.GlyphID, 256)
	switch format {
	case 0:
		nCodes, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		code := make([]byte, nCodes)
		_, err = io.ReadFull(p, code)
		if err != nil {
			return nil, err
		}
		for i := font.GlyphID(1); i <= font.GlyphID(nCodes); i++ {
			res[code[i]] = i
		}
	case 1:
		nRanges, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		current := font.GlyphID(1)
		for i := 0; i < int(nRanges); i++ {
			first, err := p.ReadUInt8()
			if err != nil {
				return nil, err
			}
			nLeft, err := p.ReadUInt8()
			if err != nil {
				return nil, err
			}
			if int(first)+int(nLeft) > 255 {
				return nil, errors.New("invalid encoding")
			}
			for j := int(first); j <= int(first+nLeft); j++ {
				res[j] = current
				current++
			}
		}
	default:
		return nil, errors.New("cff: unsupported encoding format")
	}

	if hasSupplement {
		lookup := make(map[uint16]font.GlyphID)
		for gid, sid := range charset {
			lookup[uint16(sid)] = font.GlyphID(gid)
		}
		nSups, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(nSups); i++ {
			code, err := p.ReadUInt8()
			if err != nil {
				return nil, err
			}
			sid, err := p.ReadUInt16()
			if err != nil {
				return nil, err
			}
			gid := lookup[sid]
			if gid != 0 {
				res[code] = gid
			}
		}
	}

	return res, nil
}

func stdEncoding(glyphs []*Glyph) []font.GlyphID {
	lookup := make(map[pdf.Name]font.GlyphID)
	for i, g := range glyphs {
		lookup[g.Name] = font.GlyphID(i)
	}

	res := make([]font.GlyphID, 256)
	for code, name := range type1.StandardEncoding {
		if gid, ok := lookup[name]; ok {
			res[code] = gid
		}
	}
	return res
}

func expertEncoding(glyphs []*Glyph) []font.GlyphID {
	lookup := make(map[pdf.Name]font.GlyphID)
	for i, g := range glyphs {
		lookup[g.Name] = font.GlyphID(i)
	}

	res := make([]font.GlyphID, 256)
	for code, name := range type1.ExpertEncoding {
		if gid, ok := lookup[name]; ok {
			res[code] = gid
		}
	}
	return res
}

func isStandardEncoding(encoding []font.GlyphID, glyphs []*Glyph) bool {
	for code, gid := range encoding {
		if gid == 0 {
			continue
		}
		if type1.StandardEncoding[byte(code)] != glyphs[gid].Name {
			return false
		}
	}
	return true
}

func encodeEncoding(encoding []font.GlyphID, glyphs []*Glyph) ([]byte, error) {
	panic("not implemented")
}
