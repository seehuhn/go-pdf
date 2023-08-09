// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package type1

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/subset"
	"seehuhn.de/go/postscript/type1"
	"seehuhn.de/go/sfnt/glyph"
)

type embedded struct {
	*Font

	w pdf.Putter
	pdf.Resource

	cmap.SimpleEncoder
	closed bool
}

func (f *Font) Embed(w pdf.Putter, resName pdf.Name) (font.Embedded, error) {
	e := &embedded{
		Font: f,
		w:    w,
		Resource: pdf.Resource{
			Ref:  w.Alloc(),
			Name: resName,
		},
		SimpleEncoder: cmap.NewSimpleEncoder(),
	}
	return e, nil
}

func (e *embedded) Close() error {
	if e.closed {
		return nil
	}
	e.closed = true

	if e.SimpleEncoder.Overflow() {
		return fmt.Errorf("too many distinct glyphs used in font %q (%s)",
			e.Name, e.outlines.FontInfo.FontName)
	}
	e.SimpleEncoder = cmap.NewFrozenSimpleEncoder(e.SimpleEncoder)

	encodingGid := e.SimpleEncoder.Encoding()
	encoding := make([]string, 256)
	for i, gid := range encodingGid {
		encoding[i] = e.names[gid]
	}

	psFont := e.outlines
	var subsetTag string
	if psFont.Outlines != nil {
		psSubset := &type1.Font{}
		*psSubset = *psFont
		psSubset.Outlines = make(map[string]*type1.Glyph)
		psSubset.GlyphInfo = make(map[string]*type1.GlyphInfo)

		if _, ok := psFont.Outlines[".notdef"]; ok {
			psSubset.Outlines[".notdef"] = psFont.Outlines[".notdef"]
			psSubset.GlyphInfo[".notdef"] = psFont.GlyphInfo[".notdef"]
		}
		for _, name := range encoding {
			if _, ok := psFont.Outlines[name]; ok {
				psSubset.Outlines[name] = psFont.Outlines[name]
				psSubset.GlyphInfo[name] = psFont.GlyphInfo[name]
			}
		}
		psSubset.Encoding = encoding

		var ss []subset.Glyph
		for origGid, name := range e.names {
			if _, ok := psSubset.Outlines[name]; ok {
				ss = append(ss, subset.Glyph{
					OrigGID: glyph.ID(origGid),
					CID:     type1.CID(len(ss)),
				})
			}
		}
		subsetTag = subset.Tag(ss, psFont.NumGlyphs())
	}

	// TODO(voss): implement ToUnicode

	t1 := &EmbedInfo{
		PSFont:    psFont,
		SubsetTag: subsetTag,
		Encoding:  encoding,
		ResName:   e.Name,
	}
	return t1.Embed(e.w, e.Ref)
}
