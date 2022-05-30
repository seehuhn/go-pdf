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
	"seehuhn.de/go/pdf/font/sfnt/opentype/classdef"
	"seehuhn.de/go/pdf/font/sfnt/opentype/coverage"
	"seehuhn.de/go/pdf/font/sfnt/opentype/gtab"
	"seehuhn.de/go/pdf/font/sfntcff"
)

// Parse decodes the textual description of a LookupList.
func Parse(fontInfo *sfntcff.Info, input string) (lookups gtab.LookupList, err error) {
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
		case isIdentifier(item, "GSUB1"):
			l := p.readGsub1()
			lookups = append(lookups, l)
		case isIdentifier(item, "GSUB2"):
			l := p.readGsub2()
			lookups = append(lookups, l)
		case isIdentifier(item, "GSUB3"):
			l := p.readGsub3()
			lookups = append(lookups, l)
		case isIdentifier(item, "GSUB4"):
			l := p.readGsub4()
			lookups = append(lookups, l)
		case isIdentifier(item, "GSUB5"):
			l := p.readSeqCtx(5)
			lookups = append(lookups, l)
		case isIdentifier(item, "GSUB6"):
			l := p.readChainedSeqCtx(6)
			lookups = append(lookups, l)
		case isIdentifier(item, "GPOS1"):
			l := p.readGpos1()
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

	// TODO(voss): be more clever in choosing format 1/2 subtables,
	// or change the format so that the user has to make the decision.
	cov := makeCoverageTable(maps.Keys(res))

	isConstDelta := true
	var delta font.GlyphID
	first := true
	for gid, idx := range res {
		if first {
			first = false
			delta = font.GlyphID(idx) - gid
		} else if font.GlyphID(idx)-gid != delta {
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
		subst := make([]font.GlyphID, len(cov))
		for gid, i := range cov {
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

	cov := makeCoverageTable(maps.Keys(data))
	repl := make([][]font.GlyphID, len(cov))
	for gid, i := range cov {
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

	cov := makeCoverageTable(maps.Keys(res))
	repl := make([][]font.GlyphID, len(cov))
	for gid, i := range cov {
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

	cov := makeCoverageTable(maps.Keys(data))
	repl := make([][]gtab.Ligature, len(cov))
	for gid, i := range cov {
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

func (p *parser) readGpos1() *gtab.LookupTable {
	p.optional(itemColon)
	p.optional(itemEOL)
	flags := p.readLookupFlags()

	from := p.readGlyphList()
	p.required(itemArrow, "\"->\"")
	adj := p.readGposValueRecord()

	cov := makeCoverageTable(from)

	subtable := &gtab.Gpos1_1{
		Cov:    cov,
		Adjust: adj,
	}

	return &gtab.LookupTable{
		Meta: &gtab.LookupMetaInfo{
			LookupType: 1,
			LookupFlag: flags,
		},
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

	inputClasses := make(classdef.Table)
	inputClassIdx := make(map[string]uint16)

gsubLoop:
	for {
		next := p.peek()
		switch {
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
				res[key] = append(res[key], &gtab.SeqRule{
					Input:   input[1:],
					Actions: actions,
				})

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			cov := makeCoverageTable(maps.Keys(res))

			rules := make([][]*gtab.SeqRule, len(cov))
			for gid, i := range cov {
				rules[i] = res[gid]
			}

			subtable := &gtab.SeqContext1{
				Cov:   cov,
				Rules: rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

		case isIdentifier(next, "class"):
			className, glyphList := p.parseClassDef()
			if _, exists := inputClassIdx[className]; exists {
				p.fatal("duplicate class :%s:", className)
			}
			cls := uint16(len(inputClassIdx)) + 1
			inputClassIdx[className] = cls
			for _, gid := range glyphList {
				if _, alreadyUsed := inputClasses[gid]; alreadyUsed {
					p.fatal("overlapping classes for glyph %d", gid)
				}
				inputClasses[gid] = cls
			}
			p.optional(itemEOL)
			continue gsubLoop

		case next.typ == itemSlash: // format 2
			p.required(itemSlash, "/")
			firstGlyphs := p.readGlyphList()
			p.required(itemSlash, "/")

			numClasses := len(inputClassIdx) + 1
			rules := make([][]*gtab.ClassSeqRule, numClasses)

			for {
				inputClassNames := p.readClassNames()
				p.required(itemArrow, "\"->\"")
				actions := p.readNestedLookups()

				input := make([]uint16, len(inputClassNames))
				for i, className := range inputClassNames {
					cls, exists := inputClassIdx[className]
					if className != "" && !exists {
						p.fatal("undefined class :%s:", className)
					}
					input[i] = cls
				}

				rule := &gtab.ClassSeqRule{
					Input:   input[1:],
					Actions: actions,
				}
				rules[input[0]] = append(rules[input[0]], rule)

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			cov := makeCoverageTable(firstGlyphs)

			subtable := &gtab.SeqContext2{
				Cov:   cov,
				Input: inputClasses,
				Rules: rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

			inputClasses = make(classdef.Table) // make sure to not change subtable.Input
			maps.Clear(inputClassIdx)

		case next.typ == itemSquareBracketOpen: // format 3
			var input []coverage.Table
			for {
				input = append(input, makeCoverageTable(p.readGlyphSet()))
				if p.optional(itemArrow) {
					break
				}
			}
			actions := p.readNestedLookups()

			subtable := &gtab.SeqContext3{
				Input:   input,
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

	inputClasses := make(classdef.Table)
	inputClassIdx := make(map[string]uint16)
	backtrackClasses := make(classdef.Table)
	backtrackClassIdx := make(map[string]uint16)
	lookaheadClasses := make(classdef.Table)
	lookaheadClassIdx := make(map[string]uint16)

gsubLoop:
	for {
		next := p.peek()
		switch {
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
					Input:     input[1:],
					Lookahead: lookahead,
					Actions:   nested,
				})

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			cov := makeCoverageTable(maps.Keys(res))

			rules := make([][]*gtab.ChainedSeqRule, len(cov))
			for gid, i := range cov {
				rules[i] = res[gid]
			}

			subtable := &gtab.ChainedSeqContext1{
				Cov:   cov,
				Rules: rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

		case isIdentifier(next, "inputclass"):
			className, glyphList := p.parseClassDef()
			if _, exists := inputClassIdx[className]; exists {
				p.fatal("duplicate input class :%s:", className)
			}
			cls := uint16(len(inputClassIdx)) + 1
			inputClassIdx[className] = cls
			for _, gid := range glyphList {
				if _, alreadyUsed := inputClasses[gid]; alreadyUsed {
					p.fatal("overlapping input classes for glyph %d", gid)
				}
				inputClasses[gid] = cls
			}
			p.optional(itemEOL)
			continue gsubLoop

		case isIdentifier(next, "backtrackclass"):
			className, glyphList := p.parseClassDef()
			if _, exists := backtrackClassIdx[className]; exists {
				p.fatal("duplicate backtrack class :%s:", className)
			}
			cls := uint16(len(backtrackClassIdx)) + 1
			backtrackClassIdx[className] = cls
			for _, gid := range glyphList {
				if _, alreadyUsed := backtrackClasses[gid]; alreadyUsed {
					p.fatal("overlapping backtrack classes for glyph %d", gid)
				}
				backtrackClasses[gid] = cls
			}
			p.optional(itemEOL)
			continue gsubLoop

		case isIdentifier(next, "lookaheadclass"):
			className, glyphList := p.parseClassDef()
			if _, exists := lookaheadClassIdx[className]; exists {
				p.fatal("duplicate lookahead class :%s:", className)
			}
			cls := uint16(len(lookaheadClassIdx)) + 1
			lookaheadClassIdx[className] = cls
			for _, gid := range glyphList {
				if _, alreadyUsed := lookaheadClasses[gid]; alreadyUsed {
					p.fatal("overlapping lookahead classes for glyph %d", gid)
				}
				lookaheadClasses[gid] = cls
			}
			p.optional(itemEOL)
			continue gsubLoop

		case next.typ == itemSlash: // format 2
			p.required(itemSlash, "/")
			firstGlyphs := p.readGlyphList()
			p.required(itemSlash, "/")

			numClasses := len(inputClassIdx) + 1
			rules := make([][]*gtab.ChainedClassSeqRule, numClasses)

			for {
				backtrackClassNames := p.readClassNames()
				p.required(itemBar, "|")
				inputClassNames := p.readClassNames()
				p.required(itemBar, "|")
				lookaheadClassNames := p.readClassNames()
				p.required(itemArrow, "\"->\"")
				actions := p.readNestedLookups()

				input := make([]uint16, len(inputClassNames))
				for i, className := range inputClassNames {
					cls, exists := inputClassIdx[className]
					if className != "" && !exists {
						p.fatal("undefined class :%s:", className)
					}
					input[i] = cls
				}

				backtrack := make([]uint16, len(backtrackClassNames))
				for i, className := range backtrackClassNames {
					cls, exists := backtrackClassIdx[className]
					if className != "" && !exists {
						p.fatal("undefined class :%s:", className)
					}
					backtrack[i] = cls
				}

				lookahead := make([]uint16, len(lookaheadClassNames))
				for i, className := range lookaheadClassNames {
					cls, exists := lookaheadClassIdx[className]
					if className != "" && !exists {
						p.fatal("undefined class :%s:", className)
					}
					lookahead[i] = cls
				}

				rule := &gtab.ChainedClassSeqRule{
					Backtrack: rev(backtrack),
					Input:     input[1:],
					Lookahead: lookahead,
					Actions:   actions,
				}
				rules[input[0]] = append(rules[input[0]], rule)

				if !p.optional(itemComma) {
					break
				}
				p.optional(itemEOL)
			}

			cov := makeCoverageTable(firstGlyphs)

			subtable := &gtab.ChainedSeqContext2{
				Cov:       cov,
				Backtrack: backtrackClasses,
				Input:     inputClasses,
				Lookahead: lookaheadClasses,
				Rules:     rules,
			}
			lookup.Subtables = append(lookup.Subtables, subtable)

			inputClasses = make(classdef.Table) // make sure to not change subtable.Input
			maps.Clear(inputClassIdx)
			backtrackClasses = make(classdef.Table) // make sure to not change subtable.Backtrack
			maps.Clear(backtrackClassIdx)
			lookaheadClasses = make(classdef.Table) // make sure to not change subtable.Lookahead
			maps.Clear(lookaheadClassIdx)

		case next.typ == itemSquareBracketOpen: // format 3
			var input, backtrack, lookahead []coverage.Table
			for {
				backtrack = append(backtrack, makeCoverageTable(p.readGlyphSet()))
				if p.optional(itemBar) {
					break
				}
			}
			for {
				input = append(input, makeCoverageTable(p.readGlyphSet()))
				if p.optional(itemBar) {
					break
				}
			}
			for {
				lookahead = append(lookahead, makeCoverageTable(p.readGlyphSet()))
				if p.optional(itemArrow) {
					break
				}
			}
			actions := p.readNestedLookups()

			subtable := &gtab.ChainedSeqContext3{
				Backtrack: rev(backtrack),
				Input:     input,
				Lookahead: lookahead,
				Actions:   actions,
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

func (p *parser) parseClassDef() (string, []font.GlyphID) {
	p.readIdentifier() // "class"
	p.required(itemColon, ":")
	className := p.readIdentifier()
	p.required(itemColon, ":")
	p.optional(itemEqual)
	gidList := p.readGlyphSet()
	if len(gidList) == 0 {
		p.fatal("empty class :%s:", className)
	}
	return className, gidList
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

// readGlyphSet returns a set of glyph IDs.
// This sorts the glyphs in order of increasing GID and removes duplicates
func (p *parser) readGlyphSet() []font.GlyphID {
	p.required(itemSquareBracketOpen, "[")
	res := p.readGlyphList()
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	p.required(itemSquareBracketClose, "]")
	return unique(res)
}

func (p *parser) readNestedLookups() gtab.SeqLookups {
	var res gtab.SeqLookups
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
	return name
}

// readClassNames reads a list of one or more class names.
// At least one class name must be present or an error is raised.
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

func (p *parser) readInteger() int {
	item := p.readItem()
	if item.typ != itemInteger {
		p.fatal("expected integer, got %s", item)
	}
	x, err := strconv.Atoi(item.val)
	if err != nil {
		p.fatal("invalid integer: %q", item.val)
	}
	return x
}

func (p *parser) readInt16() int16 {
	x := p.readInteger()
	if x < -32768 || x > 32767 {
		p.fatal("int16 out of range: %d", x)
	}
	return int16(x)
}

func (p *parser) readGposValueRecord() *gtab.GposValueRecord {
	res := &gtab.GposValueRecord{}
valueRecordLoop:
	for {
		next := p.readItem()
		switch {
		case isIdentifier(next, "x"):
			res.XPlacement = p.readInt16()
		case isIdentifier(next, "y"):
			res.YPlacement = p.readInt16()
		case isIdentifier(next, "dx"):
			res.XAdvance = p.readInt16()
		case isIdentifier(next, "dy"):
			res.YAdvance = p.readInt16()
		default:
			p.backlog = append(p.backlog, next)
			break valueRecordLoop
		}
	}
	return res
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

func isIdentifier(i item, val string) bool {
	if i.typ != itemIdentifier {
		return false
	}
	return i.val == val
}

func makeCoverageTable(in []font.GlyphID) coverage.Table {
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	in = unique(in)
	cov := make(coverage.Table, len(in))
	for i, gid := range in {
		cov[gid] = i
	}
	return cov
}

// unique removes duplicates from a sorted slice.
func unique[T comparable](seq []T) []T {
	if len(seq) < 2 {
		return seq
	}

	pos := 1
	for i := 1; i < len(seq); i++ {
		if seq[i] == seq[i-1] {
			continue
		}
		seq[pos] = seq[i]
		pos++
	}

	return seq[:pos]
}

// rev reverses the order of items in a slice.
// The slice is modified in-place, and also returned.
func rev[T any](seq []T) []T {
	for i, j := 0, len(seq)-1; i < j; i, j = i+1, j-1 {
		seq[i], seq[j] = seq[j], seq[i]
	}
	return seq
}

func copyRev[T any](seq []T) []T {
	n := len(seq)
	res := make([]T, n)
	for i := 0; i < n; i++ {
		res[i] = seq[n-i-1]
	}
	return res
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
