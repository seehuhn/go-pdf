// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package simpleenc

import (
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/bits"
	"slices"
	"sync"

	"golang.org/x/text/unicode/norm"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
	"seehuhn.de/go/pdf/font/subset"
)

// Simple manages the encoding and metadata of glyphs for simple PDF fonts.
//
// It constructs a mapping from single-byte codes to
//   - character IDs (CIDs)
//   - glyph widths
//   - text content
//
// If a glyph is used with different text content (for example space and
// no-break space), different codes are allocated to allow for different
// ToUnicode mappings.
//
// A Simple is safe for concurrent use: any number of goroutines may read it
// while another allocates codes.  Codes are only ever added, never changed or
// reassigned, so a code once returned stays valid for the life of the table.
//
// Each method sees the codes allocated when it was called.  Results of
// separate calls need not agree with each other, so a caller which combines
// them, for example to build a font dictionary out of MappedCodes, Encoding
// and ToUnicode, must not allow codes to be allocated in between.
type Simple struct {
	// mu guards every field below.  Readers hold it only for the duration of
	// one lookup, never across a yield to a caller, so a caller may allocate
	// codes from inside a loop over an iterator.
	mu sync.RWMutex

	code   map[gidText]byte
	info   map[byte]*codeInfo
	notdef *codeInfo

	glyphName     map[glyph.ID]string
	glyphNameUsed map[string]bool

	// fontName is the PostScript name of the font, without any subset tag.
	// It decides which glyph list a glyph name is looked up in.
	fontName string

	baseEnc *pdfenc.Encoding

	err error
}

type gidText struct {
	gid  glyph.ID
	text string
}

type codeInfo struct {
	GID   glyph.ID
	Width float64 // PDF glyph space units
	Text  string
}

// NewSimple creates and initialises a new Table object.
//
// The notdefWidth parameter is the default width of the ".notdef" glyph,
// in PDF glyph space units.
//
// fontName is the name the font gives itself.  A font program taken out of a
// PDF file names the subset it is rather than the font, so any subset tag is
// dropped: the name is used to decide which glyph list a glyph name belongs
// to, and that is a property of the font.
func NewSimple(notdefWidth float64, fontName string, base *pdfenc.Encoding) *Simple {
	_, psName := subset.Split(fontName)
	gd := &Simple{
		code:          make(map[gidText]byte),
		info:          make(map[byte]*codeInfo),
		notdef:        &codeInfo{GID: 0, Text: "", Width: notdefWidth},
		glyphName:     make(map[glyph.ID]string),
		glyphNameUsed: make(map[string]bool),
		fontName:      psName,
		baseEnc:       base,
	}
	gd.glyphName[0] = ".notdef"
	gd.glyphNameUsed[".notdef"] = true
	return gd
}

// WritingMode implements the [font.EmbeddedOld] interface.
func (*Simple) WritingMode() font.WritingMode {
	return font.Horizontal
}

// Codes returns an iterator over the characters in the PDF string. Each code
// includes the CID, width, and associated text. Missing glyphs map to CID 0
// (notdef).
func (t *Simple) Codes(s pdf.String) iter.Seq[font.Code] {
	return func(yield func(font.Code) bool) {
		var code font.Code
		for _, c := range s {
			info := t.lookup(c)
			if info.GID == 0 {
				code.CID = 0
			} else {
				code.CID = cid.CID(c) + 1 // CID 0 is reserved for .notdef
			}
			code.Width = info.Width / 1000
			code.UseWordSpacing = (c == 0x20)
			code.Text = info.Text
			if !yield(code) {
				return
			}
		}
	}
}

type Code struct {
	CID   cid.CID
	Width float64
	Text  string
}

// MappedCodes iterates over the codes allocated so far.  The order is
// unspecified.  The [Code] pointer is only valid until the next iteration; a
// caller which needs to keep the data must copy it.
func (t *Simple) MappedCodes() iter.Seq2[byte, *Code] {
	return func(yield func(byte, *Code) bool) {
		// the table is walked under the lock and yielded from outside it,
		// which is sound because entries are never changed once stored
		t.mu.RLock()
		type entry struct {
			c    byte
			info *codeInfo
		}
		entries := make([]entry, 0, len(t.info))
		for c, info := range t.info {
			entries = append(entries, entry{c, info})
		}
		t.mu.RUnlock()

		var code Code
		for _, e := range entries {
			code.CID = cid.CID(e.c) + 1 // CID 0 is reserved for .notdef
			code.Width = e.info.Width
			code.Text = e.info.Text
			if !yield(e.c, &code) {
				return
			}
		}
	}
}

// GetCode returns the code for the given glyph ID and text.
// If the code is not found, the function returns (0,false).
func (t *Simple) GetCode(gid glyph.ID, text string) (byte, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	k := gidText{gid: gid, text: text}
	c, ok := t.code[k]
	return c, ok
}

// Encode returns the code for the given glyph ID and text, allocating a new
// one on first use.  A new code also gets a unique glyph name for the glyph.
//
// The new glyph name is chosen using a heuristic based on baseGlyphName
// (optional) and text. The new code is chosen using a heuristic based on the
// glyph name and first rune in text.
//
// The last argument is the width of the glyph in PDF glyph space units.  It
// is used only when a code is allocated; a glyph already encoded keeps the
// width it was given.
//
// Only 256 codes are available. Once all codes are used up, the function
// returns an error.
func (t *Simple) Encode(gid glyph.ID, baseGlyphName, text string, width float64) (byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := gidText{gid: gid, text: text} // TODO(voss): should this include the width?
	if c, ok := t.code[key]; ok {
		return c, nil
	}

	if len(t.info) >= 256 {
		t.err = ErrOverflow
		return 0, ErrOverflow
	}

	glyphName := t.makeGlyphName(gid, baseGlyphName, text)

	var r rune
	rr := []rune(names.ToUnicode(glyphName, t.fontName))
	if len(rr) == 0 {
		rr = []rune(norm.NFD.String(text))
	}
	if len(rr) > 0 {
		r = rr[0]
	}

	bestScore := -1
	bestCode := byte(0)
	for codeInt := range 256 {
		code := byte(codeInt)
		if _, used := t.info[code]; used {
			continue
		}

		// We checked above that at most 255 codes are used.  Thus, at least
		// one code is free and we always reach this point.

		score := 0
		stdName := t.baseEnc.Encoding[code]
		if stdName == glyphName {
			bestCode = code
			break
		} else if stdName == ".notdef" || stdName == "" {
			score += 100
		} else if !(code == 32 && glyphName != "space") {
			score += 10
		}
		score += bits.TrailingZeros16(uint16(r) ^ uint16(code))
		if score > bestScore {
			bestScore = score
			bestCode = code
		}
	}

	t.info[bestCode] = &codeInfo{GID: gid, Width: width, Text: text}
	t.code[key] = bestCode

	return bestCode, nil
}

// makeGlyphName returns a unique name for the given glyph.
func (t *Simple) makeGlyphName(gid glyph.ID, defaultGlyphName, text string) string {
	if name, ok := t.glyphName[gid]; ok {
		return name
	}

	glyphName := defaultGlyphName
	if !names.IsValid(glyphName) {
		glyphName = names.FromUnicode(text)
	}

	alt := 0
	base := glyphName
nameLoop:
	for !names.IsValid(glyphName) || t.glyphNameUsed[glyphName] {
		if len(base) == 0 || len(glyphName) > 31 {
			// Try one more name than gd.glyphNameUsed has elements.
			// This guarantees that we find a free name.
			for idx := len(t.glyphNameUsed); idx >= 0; idx-- {
				glyphName = fmt.Sprintf("orn%03d", idx) // at most 256 glyphs, so 3 digits are enough
				if !t.glyphNameUsed[glyphName] {
					break nameLoop
				}
			}
		}
		alt++
		glyphName = fmt.Sprintf("%s.alt%d", base, alt)
	}
	t.glyphName[gid] = glyphName
	t.glyphNameUsed[glyphName] = true
	return glyphName
}

// get returns the entry for a code, or the notdef entry for a code not
// allocated.  The caller holds t.mu.
func (t *Simple) get(c byte) *codeInfo {
	info, ok := t.info[c]
	if !ok {
		return t.notdef
	}
	return info
}

// lookup is [Simple.get] for a caller which does not hold t.mu.  The entry
// returned stays valid after the lock is released, because entries are never
// changed once stored.
func (t *Simple) lookup(c byte) *codeInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.get(c)
}

func (t *Simple) GID(c byte) glyph.ID {
	return t.lookup(c).GID
}

// Width returns the width of the glyph for the given code, in PDF glyph space
// units.
func (t *Simple) Width(c byte) float64 {
	return t.lookup(c).Width
}

// GlyphName returns the chosen glyph name for the given glyph ID.
func (t *Simple) GlyphName(gid glyph.ID) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.glyphName[gid]
}

// Error returns the first error that occurred during encoding.
// The only possible error is ErrOverflow, if more than 256 distinct glyphs
// are used.
func (t *Simple) Error() error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.err
}

// Glyphs returns a sorted list of the glyphs used.
func (t *Simple) Glyphs() []glyph.ID {
	t.mu.RLock()
	defer t.mu.RUnlock()

	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for k := range t.code {
		gidIsUsed[k.gid] = struct{}{}
	}
	glyphs := slices.Sorted(maps.Keys(gidIsUsed))
	return glyphs
}

// Encoding returns the Type1 encoding corresponding to the glyph data.
func (t *Simple) Encoding() encoding.Simple {
	t.mu.RLock()
	defer t.mu.RUnlock()

	enc := make(map[byte]string)
	for k, c := range t.code {
		enc[c] = t.glyphName[k.gid]
	}
	return func(c byte) string { return enc[c] }
}

// DefaultWidth returns a good value for the MissingWidth entry in the font
// descriptor.
func (t *Simple) DefaultWidth() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	w1 := t.get(0).Width
	n1 := 1
	for c := 1; c < 256; c++ {
		if w := t.get(byte(c)).Width; w != w1 {
			break
		}
		n1++
	}

	w2 := t.get(255).Width
	n2 := 1
	for c := 254; c >= 0; c-- {
		if w := t.get(byte(c)).Width; w != w2 {
			break
		}
		n2++
	}

	if max(n1, n2) == 1 && w1 != w2 {
		// Only one value would be covered by the default width.
		// We can just as well store this one value in the Widths array
		// instead of in the font descriptor.
		return 0
	} else if n1 >= n2 {
		return w1
	}
	return w2
}

// IsSymbolic returns true if glyphs outside the standard Latin character set
// are used.
func (t *Simple) IsSymbolic() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for glyphName := range t.glyphNameUsed {
		if glyphName == ".notdef" {
			continue
		}
		if !pdfenc.StandardLatin.Has[glyphName] {
			return true
		}
	}
	return false
}

func (t *Simple) CodesRemaining() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return max(256-len(t.info), 0)
}

func (t *Simple) Codec() *charcode.Codec {
	return charcode.SimpleCodec
}

// ErrOverflow is returned by [Simple.Encode] once all 256 codes are in use.
var ErrOverflow = errors.New("too many glyphs")

// GlyphID returns the glyph a CID selects, for the CID scheme of simple
// fonts: CID 0 is the notdef glyph and CID c+1 is the glyph of code c.  ok is
// false for a code no glyph has been assigned to.
func (t *Simple) GlyphID(c cid.CID) (glyph.ID, bool) {
	if c == 0 {
		return 0, true
	}
	if c > 256 {
		return 0, false
	}
	gid := t.GID(byte(c - 1))
	return gid, gid != 0
}
