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
		item := p.readItem()
		switch {
		case item.typ == itemEOF:
			return
		case item.typ == itemError:
			p.fatal("%s", item.val)
		case item.typ == itemSemicolon || item.typ == itemEOL:
			// pass
		case item.typ == itemIdentifier && item.val == "class":
			p.parseClassDef()
		case item.typ == itemIdentifier && item.val == "GSUB_1":
			l := p.readGsub1()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_2":
			l := p.readGsub2()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_3":
			l := p.readGsub3()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_4":
			l := p.readGsub4()
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_5":
			l := p.readSeqCtx(5)
			lookups = append(lookups, l)
		case item.typ == itemIdentifier && item.val == "GSUB_6":
			l := p.readChainedSeqCtx(6)
			lookups = append(lookups, l)
		default:
			p.fatal("unexpected %s", item)
		}
	}
}

func (p *parser) readGsub1() *gtab.LookupTable {
	res := make(map[font.GlyphID]font.GlyphID)

	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()
	for {
		from := p.readGlyphList()
		p.required(itemArrow, "\"->\"")
		to := p.readGlyphList()
		if len(from) != len(to) {
			p.fatal("length mismatch: %v vs. %v", from, to)
		}
		for i, fromGid := range from {
			if _, ok := res[fromGid]; ok {
				p.fatal("duplicate mapping for GID %d", fromGid)
			}
			res[fromGid] = to[i]
		}

		if !p.optional(itemComma) {
			break
		}
		p.optional(itemEOL)
	}

	if len(res) == 0 {
		p.fatal("no substitutions found")
	}

	// TODO(voss): be more clever in choosing format 1/2 subtables
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
		Meta: &gtab.LookupMetaInfo{
			LookupType: 1,
			LookupFlag: flags,
		},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) readGsub2() *gtab.LookupTable {
	data := make(map[font.GlyphID][]font.GlyphID)

	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()
	for {
		from := p.readGlyphList()
		if len(from) != 1 {
			p.fatal("expected single glyph, got %v", from)
		}
		p.required(itemArrow, "\"->\"")
		to := p.readGlyphList()
		if len(to) == 0 {
			p.fatal("expected at least one glyph at %s", p.readItem())
		}

		fromGid := from[0]
		if _, ok := data[fromGid]; ok {
			p.fatal("duplicate mapping for GID %d", fromGid)
		}
		data[fromGid] = to

		if !p.optional(itemComma) {
			break
		}
		p.optional(itemEOL)
	}

	if len(data) == 0 {
		p.fatal("no substitutions found")
	}

	in := maps.Keys(data)
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}
	repl := make([][]font.GlyphID, len(in))
	for i, gid := range in {
		repl[i] = data[gid]
	}
	subtable := &gtab.Gsub2_1{
		Cov:  cov,
		Repl: repl,
	}

	return &gtab.LookupTable{
		Meta: &gtab.LookupMetaInfo{
			LookupType: 2,
			LookupFlag: flags,
		},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) readGsub3() *gtab.LookupTable {
	res := make(map[font.GlyphID][]font.GlyphID)

	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()
	for {
		from := p.readGlyphList()
		if len(from) != 1 {
			p.fatal("expected single glyph, got %v", from)
		}
		p.required(itemArrow, "\"->\"")
		to := p.readGlyphSet()
		if len(to) == 0 {
			p.fatal("expected at least one glyph at %s", p.readItem())
		}

		fromGid := from[0]
		if _, ok := res[fromGid]; ok {
			p.fatal("duplicate mapping for GID %d", fromGid)
		}
		res[fromGid] = to

		if !p.optional(itemComma) {
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
		Meta: &gtab.LookupMetaInfo{
			LookupType: 3,
			LookupFlag: flags,
		},
		Subtables: []gtab.Subtable{subtable},
	}
}

func (p *parser) readGsub4() *gtab.LookupTable {
	data := make(map[font.GlyphID][]gtab.Ligature)

	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()
	for {
		from := p.readGlyphList()
		if len(from) == 0 {
			p.fatal("expected at least one glyph at %s", p.readItem())
		}
		p.required(itemArrow, "\"->\"")
		to := p.readGlyphList()
		if len(to) != 1 {
			p.fatal("expected single glyph, got %v", to)
		}

		key := from[0]
		data[key] = append(data[key], gtab.Ligature{In: from[1:], Out: to[0]})

		if !p.optional(itemComma) {
			break
		}
		p.optional(itemEOL)
	}

	in := maps.Keys(data)
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}

	repl := make([][]gtab.Ligature, len(in))
	for i, gid := range in {
		repl[i] = data[gid]
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

func (p *parser) readSeqCtx(lookupType uint16) *gtab.LookupTable {
	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()

	lookup := &gtab.LookupTable{
		Meta:      &gtab.LookupMetaInfo{LookupType: lookupType, LookupFlag: flags},
		Subtables: []gtab.Subtable{},
	}

	for {
		next := p.peek()
		switch next.typ {
		default: // format 1
			res := make(map[font.GlyphID][]*gtab.SeqRule)
			for {
				input := p.readGlyphList()
				p.required(itemArrow, "\"->\"")
				actions := p.readNestedLookups()

				if len(input) == 0 {
					p.fatal("expected at least one glyph at %s", p.readItem())
				}

				key := input[0]
				res[key] = append(res[key], &gtab.SeqRule{In: input[1:], Actions: actions})

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}
			in := maps.Keys(res)
			sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
			cov := make(coverage.Table, len(in))
			for i, gid := range in {
				cov[gid] = i
			}

			rules := make([][]*gtab.SeqRule, len(in))
			for i, gid := range in {
				rules[i] = res[gid]
			}

			subtable := &gtab.SeqContext1{
				Cov:   cov,
				Rules: rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

		case itemSlash: // format 2
			p.required(itemSlash, "/")
			firstGlyphs := p.readGlyphList()
			p.required(itemSlash, "/")

			classIndex := make(map[string]uint16)

			data := make(map[uint16][]*gtab.ClassSeqRule)

			for {
				inputClasses := p.readClassNames()
				p.required(itemArrow, "\"->\"")
				actions := p.readNestedLookups()

				classIndices := make([]uint16, len(inputClasses))
				for i, className := range inputClasses {
					if _, ok := classIndex[className]; !ok && className != "" {
						_, ok := p.classes[className]
						if !ok {
							p.fatal("unknown class :%s:", className)
						}
						classIndex[className] = uint16(len(classIndex) + 1)
					}
					classIndices[i] = classIndex[className]
				}

				firstClass := classIndices[0]
				data[firstClass] = append(data[firstClass], &gtab.ClassSeqRule{
					In:      classIndices[1:],
					Actions: actions,
				})

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			cov := make(coverage.Table, len(firstGlyphs))
			for i, gid := range firstGlyphs {
				cov[gid] = i
			}

			classes := make(map[font.GlyphID]uint16)
			for className, classIdx := range classIndex {
				if classIdx == 0 {
					continue
				}
				cls := p.classes[className]
				for _, gid := range cls {
					_, ok := classes[gid]
					if ok {
						p.fatal("overlapping classes for glyph %d", gid)
					}
					classes[gid] = classIdx
				}
			}

			numClasses := len(classIndex) + 1
			rules := make([][]*gtab.ClassSeqRule, numClasses)
			for classIndex := range rules {
				rules[classIndex] = data[uint16(classIndex)]
			}

			subtable := &gtab.SeqContext2{
				Cov:     cov,
				Classes: classes,
				Rules:   rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

		case itemSquareBracketOpen: // format 3
			var in []coverage.Table
			for {
				glyphs := p.readGlyphSet()
				cov := make(coverage.Table, len(glyphs))
				for i, gid := range glyphs {
					cov[gid] = i
				}
				in = append(in, cov)
				if p.optional(itemArrow) {
					break
				}
			}
			actions := p.readNestedLookups()

			subtable := &gtab.SeqContext3{
				In:      in,
				Actions: actions,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)
		}
		if !p.optional(itemOr) {
			break
		}
		p.optional(itemEOL)
	}

	return lookup
}

func (p *parser) readChainedSeqCtx(lookupType uint16) *gtab.LookupTable {
	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()

	lookup := &gtab.LookupTable{
		Meta:      &gtab.LookupMetaInfo{LookupType: lookupType, LookupFlag: flags},
		Subtables: []gtab.Subtable{},
	}

	for {
		next := p.peek()
		switch next.typ {
		default: // format 1
			res := make(map[font.GlyphID][]*gtab.ChainedSeqRule)
			for {
				backtrack := p.readGlyphList()
				p.required(itemBar, "|")
				input := p.readGlyphList()
				p.required(itemBar, "|")
				lookahead := p.readGlyphList()
				p.required(itemArrow, "\"->\"")
				nested := p.readNestedLookups()

				if len(input) == 0 {
					p.fatal("expected at least one glyph at %s", p.readItem())
				}

				key := input[0]
				res[key] = append(res[key], &gtab.ChainedSeqRule{
					Backtrack: rev(backtrack),
					Input:     input,
					Lookahead: lookahead,
					Actions:   nested,
				})

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			in := maps.Keys(res)
			sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
			cov := make(coverage.Table, len(in))
			for i, gid := range in {
				cov[gid] = i
			}

			rules := make([][]*gtab.ChainedSeqRule, len(in))
			for i, gid := range in {
				rules[i] = res[gid]
			}

			subtable := &gtab.ChainedSeqContext1{
				Cov:   cov,
				Rules: rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

		case itemSlash: // format 2
			p.required(itemSlash, "/")
			firstGlyphs := p.readGlyphList()
			p.required(itemSlash, "/")

			classIndex := make(map[string]uint16)

			data := make(map[uint16][]*gtab.ClassSeqRule)

			for {
				backtrackClasses := p.readClassNames()
				p.required(itemBar, "|")
				inputClasses := p.readClassNames()
				p.required(itemBar, "|")
				lookaheadClasses := p.readClassNames()
				p.required(itemArrow, "\"->\"")
				actions := p.readNestedLookups()

				_ = backtrackClasses
				_ = lookaheadClasses
				// TODO(voss)

				classIndices := make([]uint16, len(inputClasses))
				for i, className := range inputClasses {
					if _, ok := classIndex[className]; !ok && className != "" {
						_, ok := p.classes[className]
						if !ok {
							p.fatal("unknown class :%s:", className)
						}
						classIndex[className] = uint16(len(classIndex) + 1)
					}
					classIndices[i] = classIndex[className]
				}

				firstClass := classIndices[0]
				data[firstClass] = append(data[firstClass], &gtab.ClassSeqRule{
					In:      classIndices[1:],
					Actions: actions,
				})

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			cov := make(coverage.Table, len(firstGlyphs))
			for i, gid := range firstGlyphs {
				cov[gid] = i
			}

			classes := make(map[font.GlyphID]uint16)
			for className, classIdx := range classIndex {
				if classIdx == 0 {
					continue
				}
				cls := p.classes[className]
				for _, gid := range cls {
					_, ok := classes[gid]
					if ok {
						p.fatal("overlapping classes for glyph %d", gid)
					}
					classes[gid] = classIdx
				}
			}

			numClasses := len(classIndex) + 1
			rules := make([][]*gtab.ClassSeqRule, numClasses)
			for classIndex := range rules {
				rules[classIndex] = data[uint16(classIndex)]
			}

			subtable := &gtab.SeqContext2{
				Cov:     cov,
				Classes: classes,
				Rules:   rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

		case itemSquareBracketOpen: // format 3
			var in []coverage.Table
			for {
				glyphs := p.readGlyphSet()
				cov := make(coverage.Table, len(glyphs))
				for i, gid := range glyphs {
					cov[gid] = i
				}
				in = append(in, cov)
				if p.optional(itemArrow) {
					break
				}
			}
			actions := p.readNestedLookups()

			subtable := &gtab.SeqContext3{
				In:      in,
				Actions: actions,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)
		}
		if !p.optional(itemOr) {
			break
		}
		p.optional(itemEOL)
	}

	return lookup
}

func (p *parser) readLookupFlags() gtab.LookupFlags {
	var flags gtab.LookupFlags
	for {
		if !p.optional(itemHyphen) {
			break
		}
		which := p.readIdentifier()
		switch which {
		case "marks":
			flags |= gtab.LookupIgnoreMarks
		case "ligs":
			flags |= gtab.LookupIgnoreLigatures
		case "base":
			flags |= gtab.LookupIgnoreBaseGlyphs
		default:
			p.fatal("unknown lookup flag: %s", which)
		}
	}
	p.optional(itemEOL)
	return flags
}

func (p *parser) parseClassDef() {
	p.required(itemColon, ":")
	className := p.readIdentifier()
	p.required(itemColon, ":")
	if _, ok := p.classes[className]; ok {
		p.fatal("multiple definition for class %q", className)
	}
	p.optional(itemEqual)
	gidList := p.readGlyphSet()

	p.classes[className] = gidList
}

func (p *parser) readGlyphList() []font.GlyphID {
	var res []font.GlyphID

	var item item
	hyphenSeen := false
	for {
		item = p.readItem()

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
					p.fatal("rune %q not in mapped in font", r)
				}
				next = append(next, gid)
			}

		case itemInteger:
			x, err := strconv.Atoi(item.val)
			if err != nil || x < 0 || x >= p.fontInfo.NumGlyphs() {
				p.fatal("invalid glyph id %q", item.val)
			}
			next = append(next, font.GlyphID(x))

		case itemHyphen:
			if hyphenSeen {
				p.fatal("consecutive hyphens in glyph list")
			}
			hyphenSeen = true

		default:
			goto done
		}

		for _, gid := range next {
			if hyphenSeen {
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
				hyphenSeen = false
			} else {
				res = append(res, gid)
			}
		}
	}
done:
	p.backlog = append(p.backlog, item)

	if hyphenSeen {
		p.fatal("hyphenated range not terminated")
	}

	return res
}

func (p *parser) readGlyphSet() []font.GlyphID {
	p.required(itemSquareBracketOpen, "[")
	res := p.readGlyphList()
	p.required(itemSquareBracketClose, "]")
	return res
}

func (p *parser) readNestedLookups() gtab.Nested {
	var res gtab.Nested
	for {
		item := p.readItem()
		if item.typ != itemInteger {
			p.backlog = append(p.backlog, item)
			return res
		}
		lookupIndex, err := strconv.Atoi(item.val)
		if err != nil {
			p.fatal("invalid lookup index: %q", item.val)
		}
		p.required(itemAt, "@")
		item = p.readItem()
		if item.typ != itemInteger {
			p.fatal("invalid lookup position: %q", item.val)
		}
		lookupPos, err := strconv.Atoi(item.val)
		if err != nil {
			p.fatal("invalid lookup position: %q", item.val)
		}
		res = append(res, gtab.SeqLookup{
			SequenceIndex:   uint16(lookupPos),
			LookupListIndex: gtab.LookupIndex(lookupIndex),
		})
	}
}

func (p *parser) readIdentifier() string {
	item := p.readItem()
	if item.typ != itemIdentifier {
		p.fatal("expected identifier, got %s", item)
	}
	return item.val
}

func (p *parser) readClassName() string {
	p.required(itemColon, ":")
	item := p.readItem()
	var name string
	switch item.typ {
	case itemIdentifier:
		name = item.val
		p.required(itemColon, ":")
	case itemColon:
		// pass
	default:
		p.fatal("expected class name, got %s", item)
	}
	if name == "" {
		return ""
	}
	if _, ok := p.classes[name]; !ok {
		p.fatal("unknown class :%s:", name)
	}
	return name
}

// read a list of one or more class names
func (p *parser) readClassNames() []string {
	var classNames []string
	for {
		classNames = append(classNames, p.readClassName())

		next := p.readItem()
		p.backlog = append(p.backlog, next)
		if next.typ != itemColon {
			return classNames
		}
	}
}

func (p *parser) readItem() item {
	if len(p.backlog) > 0 {
		n := len(p.backlog) - 1
		item := p.backlog[n]
		p.backlog = p.backlog[:n]
		return item
	}
	return <-p.tokens
}

func (p *parser) peek() item {
	next := p.readItem()
	p.backlog = append(p.backlog, next)
	return next
}

func (p *parser) required(typ itemType, desc string) item {
	item := p.readItem()
	if item.typ != typ {
		p.fatal("expected %s, got %s", desc, item)
	}
	return item
}

func (p *parser) optional(typ itemType) bool {
	item := p.readItem()
	if item.typ != typ {
		p.backlog = append(p.backlog, item)
		return false
	}
	return true
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

// rev reverses the order of glyphs in seq.
// The slice is modified in-place, and also returned.
func rev(seq []font.GlyphID) []font.GlyphID {
	for i, j := 0, len(seq)-1; i < j; i, j = i+1, j-1 {
		seq[i], seq[j] = seq[j], seq[i]
	}
	return seq
}

type parseError struct {
	msg string
}

func (err *parseError) Error() string {
	return err.msg
}

func (p *parser) fatal(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	panic(&parseError{msg})
}
