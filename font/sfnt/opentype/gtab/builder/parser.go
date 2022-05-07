// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package builder

import (
	"fmt"
	"sort"
	"strconv"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfntcff"
)

func parse(fontInfo *sfntcff.Info, input string) (lookups gtab.LookupList, err error) {
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

		classes: make(map[string][]font.GlyphID),
	}

	defer func() {
		if r := recover(); r != nil {
			for range tokens {
				// drain the lexer
			}
			if e, ok := r.(*parseError); ok {
				err = e
			} else {
				panic(r)
			}
		}
	}()

	lookups = p.parse()
	return
}

type parser struct {
	tokens  <-chan item
	backlog []item

	fontInfo *sfntcff.Info
	byName   map[string]font.GlyphID

	classes map[string][]font.GlyphID
}

func (p *parser) parse() (lookups gtab.LookupList) {
	for {
		item := p.next()
		switch {
		case item.typ == itemEOF:
			return
		case item.typ == itemError:
			p.fatal(item.val)
		case item.typ == itemSemicolon || item.typ == itemEOL:
			// pass
		case item.typ == itemIdentifier && item.val == "class":
			p.parseClassDef()
		case item.typ == itemIdentifier && item.val == "GSUB_1":
			l := p.parseGsub1()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_2":
			l := p.parseGsub2()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_3":
			l := p.parseGsub3()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_4":
			l := p.parseGsub4()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_5":
			l := p.parseSeqCtx()
			lookups = append(lookups, l)
		default:
			p.fatal(fmt.Sprintf("unexpected %s", item))
		}
	}
}

func (p *parser) parseGsub1() *gtab.LookupTable {
	res := make(map[font.GlyphID]font.GlyphID)

	p.optional(itemColon)
	p.optional(itemEOL)
	for {
		from := p.parseGlyphList()
		p.expect(itemArrow, "\"->\"")
		to := p.parseGlyphList()
		if len(from) != len(to) {
			p.fatal(fmt.Sprintf("length mismatch: %v vs. %v", from, to))
		}
		for i, fromGid := range from {
			if _, ok := res[fromGid]; ok {
				p.fatal(fmt.Sprintf("duplicate mapping for GID %d", fromGid))
			}
			res[fromGid] = to[i]
		}

		sep := p.next()
		if sep.typ != itemComma {
			p.backlog = append(p.backlog, sep)
			break
		}
		p.optional(itemEOL)
	}

	if len(res) == 0 {
		p.fatal("no substitutions found")
	}

	// length of a coverage table:
	//   - format 1: 4 + 2*n
	//   - format 2: 4 + 6*n.ranges
	// length of a GSUB 1.1 subtable: 6 + len(cov)
	// length of a GSUB 1.2 subtable: 6 + 2*n + len(cov)
	in := maps.Keys(res)
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}

	isConstDelta := true
	delta := res[in[0]] - in[0]
	for _, gid := range in[1:] {
		if res[gid] != delta+gid {
			isConstDelta = false
			break
		}
	}
	var subtable gtab.Subtable
	if isConstDelta {
		subtable = &gtab.Gsub1_1{
			Cov:   cov,
			Delta: delta,
		}
	} else {
		subst := make([]font.GlyphID, len(in))
		for i, gid := range in {
			subst[i] = res[gid]
		}
		subtable = &gtab.Gsub1_2{
			Cov:                cov,
			SubstituteGlyphIDs: subst,
		}
	}
	return &gtab.LookupTable{
		Meta:      &gtab.LookupMetaInfo{LookupType: 1},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) parseGsub2() *gtab.LookupTable {
	res := make(map[font.GlyphID][]font.GlyphID)

	p.optional(itemColon)
	p.optional(itemEOL)
	for {
		from := p.parseGlyphList()
		if len(from) != 1 {
			p.fatal(fmt.Sprintf("expected single glyph, got %v", from))
		}
		p.expect(itemArrow, "\"->\"")
		to := p.parseGlyphList()
		if len(to) == 0 {
			p.fatal(fmt.Sprintf("expected at least one glyph at %s", p.next()))
		}

		fromGid := from[0]
		if _, ok := res[fromGid]; ok {
			p.fatal(fmt.Sprintf("duplicate mapping for GID %d", fromGid))
		}
		res[fromGid] = to

		sep := p.next()
		if sep.typ != itemComma {
			p.backlog = append(p.backlog, sep)
			break
		}
		p.optional(itemEOL)
	}

	if len(res) == 0 {
		p.fatal("no substitutions found")
	}

	in := maps.Keys(res)
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}
	repl := make([][]font.GlyphID, len(in))
	for i, gid := range in {
		repl[i] = res[gid]
	}
	subtable := &gtab.Gsub2_1{
		Cov:  cov,
		Repl: repl,
	}

	return &gtab.LookupTable{
		Meta:      &gtab.LookupMetaInfo{LookupType: 2},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) parseGsub3() *gtab.LookupTable {
	res := make(map[font.GlyphID][]font.GlyphID)

	p.optional(itemColon)
	p.optional(itemEOL)
	for {
		from := p.parseGlyphList()
		if len(from) != 1 {
			p.fatal(fmt.Sprintf("expected single glyph, got %v", from))
		}
		p.expect(itemArrow, "\"->\"")
		to := p.parseGlyphSet()
		if len(to) == 0 {
			p.fatal(fmt.Sprintf("expected at least one glyph at %s", p.next()))
		}

		fromGid := from[0]
		if _, ok := res[fromGid]; ok {
			p.fatal(fmt.Sprintf("duplicate mapping for GID %d", fromGid))
		}
		res[fromGid] = to

		sep := p.next()
		if sep.typ != itemComma {
			p.backlog = append(p.backlog, sep)
			break
		}
		p.optional(itemEOL)
	}

	if len(res) == 0 {
		p.fatal("no substitutions found")
	}

	in := maps.Keys(res)
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}
	repl := make([][]font.GlyphID, len(in))
	for i, gid := range in {
		repl[i] = res[gid]
	}
	subtable := &gtab.Gsub3_1{
		Cov:        cov,
		Alternates: repl,
	}

	return &gtab.LookupTable{
		Meta:      &gtab.LookupMetaInfo{LookupType: 3},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) parseGsub4() *gtab.LookupTable {
	res := make(map[font.GlyphID][]gtab.Ligature)

	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.parseLookupFlags()
	for {
		from := p.parseGlyphList()
		if len(from) == 0 {
			p.fatal(fmt.Sprintf("expected at least one glyph at %s", p.next()))
		}
		p.expect(itemArrow, "\"->\"")
		to := p.parseGlyphList()
		if len(to) != 1 {
			p.fatal(fmt.Sprintf("expected single glyph, got %v", to))
		}

		key := from[0]
		res[key] = append(res[key], gtab.Ligature{In: from[1:], Out: to[0]})

		sep := p.next()
		if sep.typ != itemComma {
			p.backlog = append(p.backlog, sep)
			break
		}
		p.optional(itemEOL)
	}

	if len(res) == 0 {
		p.fatal("no substitutions found")
	}

	in := maps.Keys(res)
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}
	repl := make([][]gtab.Ligature, len(in))
	for i, gid := range in {
		ligs := res[gid]
		sort.Slice(ligs, func(i, j int) bool {
			if len(ligs[i].In) != len(ligs[j].In) {
				return len(ligs[i].In) > len(ligs[j].In)
			}
			for i, gidA := range ligs[i].In {
				gidB := ligs[j].In[i]
				if gidA != gidB {
					return gidA < gidB
				}
			}
			return ligs[i].Out < ligs[j].Out
		})
		repl[i] = ligs
	}
	subtable := &gtab.Gsub4_1{
		Cov:  cov,
		Repl: repl,
	}

	return &gtab.LookupTable{
		Meta:      &gtab.LookupMetaInfo{LookupType: 4, LookupFlag: flags},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) parseSeqCtx() *gtab.LookupTable {
	panic("not implemented")
}

func (p *parser) parseLookupFlags() gtab.LookupFlags {
	var flags gtab.LookupFlags
	var item item
	for {
		item = p.next()
		if item.typ != itemHyphen {
			break
		}
		which := p.readIdentifier()
		switch which {
		case "marks":
			flags |= gtab.LookupIgnoreMarks
		default:
			p.fatal(fmt.Sprintf("unknown lookup flag: %s", which))
		}
	}
	p.backlog = append(p.backlog, item)
	p.optional(itemEOL)
	return flags
}

func (p *parser) parseClassDef() {
	className := p.readIdentifier()
	if _, ok := p.classes[className]; ok {
		p.fatal(fmt.Sprintf("multiple definition for class %q", className))
	}
	p.optional(itemEqual)
	gidList := p.parseGlyphList()

	p.classes[className] = gidList
}

func (p *parser) parseGlyphList() []font.GlyphID {
	var res []font.GlyphID

	var item item
	hasHyphen := false
	for {
		item = p.next()

		var next []font.GlyphID
		switch item.typ {
		case itemIdentifier:
			gid, ok := p.byName[item.val]
			if !ok {
				goto done
			}
			next = append(next, gid)

		case itemString:
			for r := range decodeString(item.val) {
				gid := p.fontInfo.CMap.Lookup(r)
				if gid == 0 {
					p.fatal(fmt.Sprintf("rune %q not in mapped in font", r))
				}
				next = append(next, gid)
			}

		case itemInteger:
			x, err := strconv.Atoi(item.val)
			if err != nil || x < 0 || x >= p.fontInfo.NumGlyphs() {
				p.fatal(fmt.Sprintf("invalid glyph id %q", item.val))
			}
			next = append(next, font.GlyphID(x))

		case itemHyphen:
			if hasHyphen {
				p.fatal("consecutive hyphens in glyph list")
			}
			hasHyphen = true

		default:
			goto done
		}
		for _, gid := range next {
			if hasHyphen {
				if len(res) == 0 {
					p.fatal("invalid range")
				}
				start := res[len(res)-1]
				if gid < start {
					for i := int(start) - 1; i >= int(gid); i-- {
						res = append(res, font.GlyphID(i))
					}
				} else if gid > start {
					for i := start + 1; i <= gid; i++ {
						res = append(res, i)
					}
				}
				hasHyphen = false
			} else {
				res = append(res, gid)
			}
		}
	}
done:
	p.backlog = append(p.backlog, item)

	if hasHyphen {
		p.fatal("hyphenated range not terminated")
	}

	return res
}

func (p *parser) parseGlyphSet() []font.GlyphID {
	p.expect(itemSquareBracketOpen, "[")
	res := p.parseGlyphList()
	p.expect(itemSquareBracketClose, "]")
	return res
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

func (p *parser) readIdentifier() string {
	item := p.next()
	if item.typ != itemIdentifier {
		p.fatal(fmt.Sprintf("expected identifier, got %s", item))
	}
	return item.val
}

func (p *parser) expect(typ itemType, desc string) item {
	item := p.next()
	if item.typ != typ {
		p.fatal(fmt.Sprintf("expected %s, got %s", desc, item))
	}
	return item
}

func (p *parser) optional(typ itemType) {
	item := p.next()
	if item.typ != typ {
		p.backlog = append(p.backlog, item)
	}
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

type parseError struct {
	msg string
}

func (err *parseError) Error() string {
	return err.msg
}

func (p *parser) fatal(msg string) {
	panic(&parseError{msg})
}
