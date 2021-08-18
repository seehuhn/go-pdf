package builtin

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Simple fonts
// Type=Font, Subtype=Type1

// Embed returns a Font structure representing one of the builtin fonts.
func Embed(w *pdf.Writer, ref string, fname string) (*font.Font, error) {
	afm, err := readAfmFont(fname)
	if err != nil {
		return nil, err
	}

	fontRef := w.Alloc()
	b := newBuiltin(afm, fontRef)

	res := &font.Font{
		Name:        pdf.Name(ref),
		Ref:         fontRef,
		Layout:      b.Layout,
		Enc:         b.Enc,
		GlyphUnits:  1000,
		Ascent:      afm.Ascent,
		Descent:     afm.Descent,
		GlyphExtent: afm.GlyphExtent,
		Width:       afm.Width,
	}
	return res, nil
}

type builtin struct {
	afm     *afmFont
	fontRef *pdf.Reference
	cmap    map[rune]font.GlyphID

	enc        map[font.GlyphID]byte
	used       map[byte]bool
	candidates []*candidate
}

func newBuiltin(afm *afmFont, fontRef *pdf.Reference) *builtin {
	fe := &fontEnc{
		To:   make(map[rune]byte),
		From: make([]rune, 256),
	}
	for i, c := range afm.Code {
		r := afm.Chars[i]
		fe.To[r] = c
		fe.From[c] = r
	}

	b := &builtin{
		afm:     afm,
		fontRef: fontRef,
		cmap:    make(map[rune]font.GlyphID),
		enc:     make(map[font.GlyphID]byte),
		used:    make(map[byte]bool),

		candidates: []*candidate{
			{name: "", enc: fe},
			{name: "WinAnsiEncoding", enc: font.WinAnsiEncoding},
			{name: "MacRomanEncoding", enc: font.MacRomanEncoding},
			// TODO(voss): add MacExpertEncoding, once it is implemented
		},
	}
	for gid, r := range afm.Chars {
		b.cmap[r] = font.GlyphID(gid)
	}
	b.enc[0] = 0
	b.used[0] = true

	return b
}

func (b *builtin) Layout(rr []rune) []font.Glyph {
	if len(rr) == 0 {
		return nil
	}

	var res []font.Glyph
	last := font.Glyph{
		Chars: []rune{rr[0]},
		Gid:   b.cmap[rr[0]],
	}
	for _, r := range rr[1:] {
		gid := b.cmap[r]
		lig, ok := b.afm.Ligatures[font.GlyphPair{last.Gid, gid}]
		if ok {
			last.Gid = lig
			last.Chars = append(last.Chars, r)
		} else {
			res = append(res, last)
			last = font.Glyph{
				Chars: []rune{r},
				Gid:   gid,
			}
		}
	}
	res = append(res, last)

	for i, glyph := range res {
		gid := glyph.Gid

		kern := 0
		if i < len(res)-1 {
			kern = b.afm.Kern[font.GlyphPair{gid, res[i+1].Gid}]
		}

		res[i].Gid = gid
		res[i].Advance = b.afm.Width[gid] + kern
	}

	return res
}

func (b *builtin) Enc(gid font.GlyphID) pdf.String {
	c, ok := b.enc[gid]
	if ok {
		return pdf.String{c}
	}

	r := b.afm.Chars[gid]
	c = 0
	hits := -1
	for _, cand := range b.candidates {
		if cand.hits < hits {
			continue
		}
		cCand, ok := cand.enc.Encode(r)
		if ok && !b.used[cCand] {
			c = cCand
			hits = cand.hits
		}
	}
	if c == 0 && len(b.used) < 256 {
		for c = 255; c > 0; c-- {
			if !b.used[c] {
				break
			}
		}
	}
	// A simple font can only encode 255 different characters.
	// If the user tries to use more characters, map some of them to the
	// missing character glyph, using c=0.

	b.enc[gid] = c
	b.used[c] = true
	if c != 0 {
		for _, cand := range b.candidates {
			if cand.enc.Decode(c) == r {
				cand.hits++
			}
		}
	}

	return pdf.String{c}
}

type fontEnc struct {
	To   map[rune]byte
	From []rune
}

func (fe *fontEnc) Decode(c byte) rune {
	return fe.From[c]
}

func (fe *fontEnc) Encode(r rune) (byte, bool) {
	c, ok := fe.To[r]
	return c, ok
}

type candidate struct {
	name string
	enc  font.Encoding
	hits int
}
