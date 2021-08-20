package builtin

import (
	"errors"
	"fmt"
	"sort"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/names"
)

// Embed returns a Font structure representing one of the 14 builtin fonts.
// The valid font names are given in FontNames.
func Embed(w *pdf.Writer, ref string, fontName string) (*font.Font, error) {
	afm, err := Afm(fontName)
	if err != nil {
		return nil, err
	}
	return EmbedAfm(w, ref, afm)
}

// EmbedAfm returns a Font structure representing a simple Type 1 font,
// described by `afm`.
func EmbedAfm(w *pdf.Writer, ref string, afm *AfmInfo) (*font.Font, error) {
	if len(afm.Code) == 0 {
		return nil, errors.New("no glyphs in font")
	}

	fontRef := w.Alloc()
	b := newBuiltin(afm, fontRef, ref)
	w.OnClose(b.WriteFontDict)

	layout := b.FullLayout
	if b.afm.IsFixedPitch {
		layout = b.SimpleLayout
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

	hasOverflow  bool
	missingRunes map[rune]bool
}

func newBuiltin(afm *AfmInfo, fontRef *pdf.Reference, name string) *builtin {
	cmap := make(map[rune]font.GlyphID)
	char := make([]rune, len(afm.Code))
	thisFontEnc := make(fontEnc)
	for gid, code := range afm.Code {
		rr := names.ToUnicode(afm.Name[gid], afm.IsDingbats)
		if len(rr) != 1 {
			// Some names produce no or more than one unicode runes.  Not sure
			// how to handle these ...
			continue
		}

		r := rr[0]
		cmap[r] = font.GlyphID(gid)
		char[gid] = r
		if code >= 0 {
			thisFontEnc[r] = byte(code)
		}
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
			{name: "", enc: thisFontEnc},
			{name: "WinAnsiEncoding", enc: font.WinAnsiEncoding},
			{name: "MacRomanEncoding", enc: font.MacRomanEncoding},
			{name: "MacExpertEncoding", enc: font.MacExpertEncoding},
		},
	}

	return b
}

func (b *builtin) RuneToGid(r rune) font.GlyphID {
	gid, ok := b.cmap[r]
	if !ok {
		if b.missingRunes == nil {
			b.missingRunes = make(map[rune]bool)
		}
		b.missingRunes[r] = true
	}
	return gid
}

// simple layout without ligatures and kerning
func (b *builtin) SimpleLayout(rr []rune) []font.Glyph {
	if len(rr) == 0 {
		return nil
	}

	res := make([]font.Glyph, len(rr))
	for i, r := range rr {
		gid := b.RuneToGid(r)
		res[i].Chars = []rune{r}
		res[i].Gid = gid
		res[i].Advance = b.afm.Width[gid]
	}

	return res
}

func (b *builtin) FullLayout(rr []rune) []font.Glyph {
	if len(rr) == 0 {
		return nil
	}

	var res []font.Glyph
	last := font.Glyph{
		Gid:   b.RuneToGid(rr[0]),
		Chars: []rune{rr[0]},
	}
	for _, r := range rr[1:] {
		gid := b.RuneToGid(r)
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

	if !found {
		// A simple font can only encode 256 different characters. If we run
		// out of character codes, just return 0 here and report an error when
		// we try to write the font dictionary at the end.
		b.hasOverflow = true
		return pdf.String{0}
	}

	b.enc[gid] = c
	b.used[c] = true
	if c != 0 {
		for _, cand := range b.candidates {
			if cTest, ok := cand.enc.Encode(r); ok && cTest == c {
				cand.hits++
			}
		}
	}

	return pdf.String{c}
}

func (b *builtin) WriteFontDict(w *pdf.Writer) error {
	if b.hasOverflow {
		return errors.New("too many different glyphs for simple font " + b.name)
	}
	if b.missingRunes != nil {
		var rr []rune
		for r := range b.missingRunes {
			rr = append(rr, r)
		}
		return fmt.Errorf("font %q lacks runes %q", b.name, string(rr))
	}

	// See section 9.6.2.1 of PDF 32000-1:2008.
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
		if cTest, ok := best.enc.Encode(r); !ok || cTest != code {
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
	var next byte
	first := true
	for _, d := range diff {
		if first || d.code != next {
			Differences = append(Differences, pdf.Integer(d.code))
		}
		Differences = append(Differences, pdf.Name(d.name))
		next = d.code + 1
		first = false
	}

	return pdf.Dict{
		"Type":         pdf.Name("Encoding"),
		"BaseEncoding": baseName,
		"Differences":  Differences,
	}
}

type fontEnc map[rune]byte

func (fe fontEnc) Encode(r rune) (byte, bool) {
	c, ok := fe[r]
	return c, ok
}

// this is half of font.Encoding
type encoder interface {
	Encode(r rune) (byte, bool)
}

type candidate struct {
	name string
	enc  encoder
	hits int
}
