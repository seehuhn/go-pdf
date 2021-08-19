package builtin

import (
	"fmt"
	"sort"
	"unicode"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

// Simple fonts
// Type=Font, Subtype=Type1

// Embed returns a Font structure representing one of the builtin fonts.
func Embed(w *pdf.Writer, ref string, fontName string) (*font.Font, error) {
	afm, err := ReadAfm(fontName)
	if err != nil {
		return nil, err
	}
	return EmbedAfm(w, ref, afm)
}

// EmbedAfm returns a Font structure representing a simple Type 1 font,
// described by the structure `afm`.
func EmbedAfm(w *pdf.Writer, ref string, afm *AfmInfo) (*font.Font, error) {
	fontRef := w.Alloc()
	b := newBuiltin(afm, fontRef, ref)
	w.OnClose(b.WriteFontDict)

	layout := b.Layout
	if b.afm.IsFixedPitch {
		layout = b.LayoutSimple
	}

	res := &font.Font{
		Name:        pdf.Name(ref),
		Ref:         fontRef,
		Layout:      layout,
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
	fontRef *pdf.Reference
	name    string
	afm     *AfmInfo

	cmap map[rune]font.GlyphID
	char []rune

	enc        map[font.GlyphID]byte
	used       map[byte]bool
	candidates []*candidate
	overflow   bool
}

func newBuiltin(afm *AfmInfo, fontRef *pdf.Reference, name string) *builtin {
	cmap := make(map[rune]font.GlyphID)
	char := make([]rune, len(afm.Code))
	fe := &fontEnc{
		To:   make(map[rune]byte),
		From: make([]rune, 256),
	}
	for gid, code := range afm.Code {
		rr := names.ToUnicode(afm.Name[gid], afm.IsDingbats)

		var r rune
		if len(rr) == 1 {
			r = rr[0]
		} else {
			// ".notdef" and invalid names give len(rr) == 0.
			r = unicode.ReplacementChar
		}

		cmap[r] = font.GlyphID(gid)
		char[gid] = r
		fe.To[r] = code
		fe.From[code] = r
	}

	b := &builtin{
		name:    name,
		afm:     afm,
		fontRef: fontRef,
		cmap:    cmap,
		char:    char,
		enc:     make(map[font.GlyphID]byte),
		used:    make(map[byte]bool),

		candidates: []*candidate{
			{name: "", enc: fe},
			{name: "WinAnsiEncoding", enc: font.WinAnsiEncoding},
			{name: "MacRomanEncoding", enc: font.MacRomanEncoding},
			// TODO(voss): add MacExpertEncoding, once it is implemented
		},
	}
	b.enc[0] = 0
	b.used[0] = true

	return b
}

// simple layout without ligatures and kerning
func (b *builtin) LayoutSimple(rr []rune) []font.Glyph {
	if len(rr) == 0 {
		return nil
	}

	res := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := b.cmap[r]
		res[i].Chars = []rune{r}
		res[i].Gid = gid
		res[i].Advance = b.afm.Width[gid]
	}

	return res
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

	r := b.char[gid]
	hits := -1
	found := false
	for _, cand := range b.candidates {
		if cand.hits < hits {
			continue
		}
		cCand, ok := cand.enc.Encode(r)
		if ok && !b.used[cCand] {
			c = cCand
			hits = cand.hits
			found = true
		}
	}
	if !found && len(b.used) < 256 {
		for i := 255; i >= 0; i-- {
			c = byte(i)
			if !b.used[c] {
				found = true
				break
			}
		}
	}
	fmt.Println(b.afm.Name[gid], b.name, c, found)

	if !found {
		// A simple font can only encode 256 different characters. If we run out of
		// character codes, just keep c==0 here and report an error when we try to
		// write the font dictionary.
		b.overflow = true
	}

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

func (b *builtin) DescribeEncoding() pdf.Object {
	best := b.candidates[0]
	for _, cand := range b.candidates[1:] {
		if cand.hits > best.hits {
			best = cand
		}
	}
	var baseName pdf.Object
	if best.name != "" {
		baseName = pdf.Name(best.name)
	}

	type D struct {
		code byte
		char rune
		name string
	}
	var diff []D
	for gid, code := range b.enc {
		r := b.char[gid]
		if best.enc.Decode(code) != r {
			diff = append(diff, D{
				code: code,
				char: r,
				name: b.afm.Name[gid],
			})
		}
	}
	if len(diff) == 0 {
		return baseName
	}

	Differences := pdf.Array{}
	sort.Slice(diff, func(i, j int) bool {
		return diff[i].code < diff[j].code
	})
	next := byte(255)
	for _, d := range diff {
		if d.code != next {
			Differences = append(Differences, pdf.Integer(d.code))
		}
		Differences = append(Differences, pdf.Name(d.name))
		next = d.code + 1
	}

	return pdf.Dict{
		"Type":         pdf.Name("Encoding"),
		"BaseEncoding": baseName,
		"Differences":  Differences,
	}
}

func (b *builtin) WriteFontDict(w *pdf.Writer) error {
	// See section 9.6.2.1 of PDF 32000-1:2008.

	// TODO(voss): if we run out of character codes, report an error here
	Font := pdf.Dict{
		"Type":     pdf.Name("Font"),
		"Subtype":  pdf.Name("Type1"),
		"BaseFont": pdf.Name(b.afm.FontName),
	}

	enc := b.DescribeEncoding()
	if enc != nil {
		Font["Encoding"] = enc
	}

	if w.Version == pdf.V1_0 {
		Font["Name"] = pdf.Name(b.name)
	}

	_, err := w.Write(Font, b.fontRef)
	return err
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
