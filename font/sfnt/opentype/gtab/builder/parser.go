package builder

import (
	"fmt"
	"strconv"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfntcff"
)

func parse(fontInfo *sfntcff.Info, input string) {
	numGlyphs := fontInfo.NumGlyphs()
	byName := make(map[string]font.GlyphID)
	for i := font.GlyphID(0); i < font.GlyphID(numGlyphs); i++ {
		glyphName := fontInfo.GlyphName(i)
		if glyphName != "" {
			byName[string(glyphName)] = i
		}
	}

	_, tokens := lex(input)
	p := &parser{
		tokens: tokens,

		fontInfo: fontInfo,
		byName:   byName,

		classes: make(map[string]classdef.Table),
	}
	gg := p.parseGlyphList()
	fmt.Println(gg)
	fmt.Println(p.next())
}

type parser struct {
	tokens  <-chan item
	backlog []item

	fontInfo *sfntcff.Info
	byName   map[string]font.GlyphID

	classes map[string]classdef.Table
}

func (p *parser) next() item {
	if len(p.backlog) > 0 {
		n := len(p.backlog) - 1
		item := p.backlog[n]
		p.backlog = p.backlog[:n]
		return item
	}
	return <-p.tokens
}

func decodeString(s string) <-chan rune {
	c := make(chan rune)
	go func() {
		s := s[1 : len(s)-1]
		escape := false
		for _, r := range s {
			if escape {
				escape = false
				switch r {
				case 'n':
					c <- '\n'
				case 'r':
					c <- '\r'
				case 't':
					c <- '\t'
				default:
					c <- r
				}
				continue
			}
			if r == '\\' {
				escape = true
				continue
			}
			c <- r
		}
		close(c)
	}()
	return c
}

func (p *parser) parseGlyphList() []font.GlyphID {
	var res []font.GlyphID
	var unmapped []rune
	var invalid []int

	var item item
	for {
		item = p.next()
		switch item.typ {
		case itemIdentifier:
			gid, ok := p.byName[item.val]
			if !ok {
				goto done
			}
			res = append(res, gid)
		case itemString:
			for r := range decodeString(item.val) {
				gid := p.fontInfo.CMap.Lookup(r)
				if gid != 0 {
					res = append(res, gid)
				} else {
					unmapped = append(unmapped, r)
				}
			}
		case itemInteger:
			gid, err := strconv.Atoi(item.val)
			if err != nil || gid < 0 || gid >= p.fontInfo.NumGlyphs() {
				invalid = append(invalid, gid)
			} else {
				res = append(res, font.GlyphID(gid))
			}
		default:
			goto done
		}
	}
done:
	p.backlog = append(p.backlog, item)

	if len(unmapped) > 0 {
		panic(fmt.Sprintf("unmapped characters: %q", string(unmapped)))
	} else if len(invalid) > 0 {
		panic(fmt.Sprintf("unmapped glyph IDs: %v", invalid))
	}

	return res
}
