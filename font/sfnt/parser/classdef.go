package parser

import (
	"seehuhn.de/go/pdf/font"
)

func (g *gTab) readClassDefTable(pos int64) (classDef, error) {
	res, ok := g.classDefCache[pos]
	if ok {
		return res, nil
	}

	s := &State{
		A: pos,
	}
	err := g.Exec(s,
		CmdSeek,
		CmdRead16, TypeUInt, // format
	)
	if err != nil {
		return nil, err
	}
	format := int(s.A)

	res = make(classDef)
	switch format {
	case 1:
		err = g.Exec(s,
			CmdRead16, TypeUInt, // startGlyphID
			CmdStash,
			CmdRead16, TypeUInt, // glyphCount
			CmdLoop,
			CmdRead16, TypeUInt, // classValueArray[i]
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		stash := s.GetStash()
		startGlyphID := font.GlyphID(stash[0])
		for class, gid := range stash[1:] {
			if class == 0 {
				continue
			}
			res[startGlyphID+font.GlyphID(gid)] = class
		}
	case 2:
		err = g.Exec(s,
			CmdRead16, TypeUInt, // classRangeCount
			CmdLoop,
			CmdRead16, TypeUInt, // classRangeRecords[i].startGlyphID
			CmdStash,
			CmdRead16, TypeUInt, // classRangeRecords[i].endGlyphID
			CmdStash,
			CmdRead16, TypeUInt, // classRangeRecords[i].class
			CmdStash,
			CmdEndLoop,
		)
		if err != nil {
			return nil, err
		}
		stash := s.GetStash()
		for len(stash) > 0 {
			start := int(stash[0])
			end := int(stash[1])
			class := int(stash[2])
			if class == 0 {
				continue
			}
			if end < start {
				return nil, g.error("corrupt classDef record")
			}
			for gid := start; gid <= end; gid++ {
				res[font.GlyphID(gid)] = class
			}
			stash = stash[3:]
		}
	default:
		return nil, g.error("unsupported classDef format %d", format)
	}

	return res, nil
}

type classDef map[font.GlyphID]int
