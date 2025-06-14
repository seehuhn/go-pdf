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
	"math/bits"
	"slices"

	"golang.org/x/exp/maps"
	"golang.org/x/text/unicode/norm"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
)

// Simple manages the encoding and metadata of glyphs for simple PDF fonts.
//
// It constructs a mapping from single-byte codes to
//   - character IDs (CIDs),
//   - glyph widths, and
//   - associated text content.
//
// If a glyph is used with different text content (for example space and
// no-break space), different codes are allocated to allow for different
// ToUnicode mappings.
type Simple struct {
	code   map[gidText]byte
	info   map[byte]*codeInfo
	notdef *codeInfo

	glyphName     map[glyph.ID]string
	glyphNameUsed map[string]bool

	fontName string
	baseEnc  *pdfenc.Encoding

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
func NewSimple(notdefWidth float64, fontName string, base *pdfenc.Encoding) *Simple {
	gd := &Simple{
		code:          make(map[gidText]byte),
		info:          make(map[byte]*codeInfo),
		notdef:        &codeInfo{GID: 0, Text: "", Width: notdefWidth},
		glyphName:     make(map[glyph.ID]string),
		glyphNameUsed: make(map[string]bool),
		fontName:      fontName,
		baseEnc:       base,
	}
	gd.glyphName[0] = ".notdef"
	gd.glyphNameUsed[".notdef"] = true
	return gd
}

// WritingMode implements the [font.Embedded] interface.
func (*Simple) WritingMode() font.WritingMode {
	return font.Horizontal
}

// Codes returns an iterator over the characters in the PDF string. Each code
// includes the CID, width, and associated text. Missing glyphs map to CID 0
// (notdef).
func (t *Simple) Codes(s pdf.String) iter.Seq[*font.Code] {
	return func(yield func(*font.Code) bool) {
		var code font.Code
		for _, c := range s {
			info := t.get(c)
			if info.GID == 0 {
				code.CID = 0
			} else {
				code.CID = cid.CID(c) + 1 // CID 0 is reserved for .notdef
			}
			code.Width = info.Width
			code.UseWordSpacing = (c == 0x20)
			code.Text = info.Text
			if !yield(&code) {
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

func (t *Simple) MappedCodes() iter.Seq2[byte, *Code] {
	return func(yield func(byte, *Code) bool) {
		var code Code
		for c, info := range t.info {
			code.CID = cid.CID(c) + 1 // CID 0 is reserved for .notdef
			code.Width = info.Width
			code.Text = info.Text
			if !yield(c, &code) {
				return
			}
		}
	}
}

// GetCode returns the code for the given glyph ID and text.
// If the code is not found, the function returns (0,false).
func (t *Simple) GetCode(gid glyph.ID, text string) (byte, bool) {
	k := gidText{gid: gid, text: text}
	c, ok := t.code[k]
	return c, ok
}

// AllocateCode allocates a new code for the given glyph ID and text. It also
// allocates a unique glyph name for the glyph.
//
// The new glyph name is chosen using a heuristic based on baseGlyphName
// (optional) and text. The new code is chosen using a heuristic based on the
// glyph name and first rune in text.
//
// The last argument is the width of the glyph in PDF glyph space units.
//
// Only 256 codes are available. Once all codes are used up, the function
// returns an error.
func (t *Simple) AllocateCode(gid glyph.ID, baseGlyphName, text string, width float64) (byte, error) {
	key := gidText{gid: gid, text: text} // TODO(voss): should this include the width?
	if _, ok := t.code[key]; ok {
		return 0, ErrDuplicateCode
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
	for codeInt := 0; codeInt < 256; codeInt++ {
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
	for {
		if names.IsValid(glyphName) && !t.glyphNameUsed[glyphName] {
			break
		}
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

func (t *Simple) get(c byte) *codeInfo {
	info, ok := t.info[c]
	if !ok {
		return t.notdef
	}
	return info
}

func (t *Simple) GID(c byte) glyph.ID {
	return t.get(c).GID
}

// Width returns the width of the glyph for the given code, in PDF glyph space
// units.
func (t *Simple) Width(c byte) float64 {
	return t.get(c).Width
}

// GlyphName returns the chosen glyph name for the given glyph ID.
func (t *Simple) GlyphName(gid glyph.ID) string {
	return t.glyphName[gid]
}

// Error returns the first error that occurred during encoding.
// The only possible error is ErrOverflow, if more than 256 distinct glyphs
// are used.
func (t *Simple) Error() error {
	return t.err
}

// Glyphs returns a sorted list of the glyphs used.
func (t *Simple) Glyphs() []glyph.ID {
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for k := range t.code {
		gidIsUsed[k.gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)
	return glyphs
}

// Encoding returns the Type1 encoding corresponding to the glyph data.
func (t *Simple) Encoding() encoding.Simple {
	enc := make(map[byte]string)
	for k, c := range t.code {
		enc[c] = t.glyphName[k.gid]
	}
	return func(c byte) string { return enc[c] }
}

// DefaultWidth returns a good value for the MissingWidth entry in the font
// descriptor.
func (t *Simple) DefaultWidth() float64 {
	w1 := t.Width(0)
	n1 := 1
	for c := 1; c < 256; c++ {
		if w := t.Width(byte(c)); w != w1 {
			break
		}
		n1++
	}

	w2 := t.Width(255)
	n2 := 1
	for c := 254; c >= 0; c-- {
		if w := t.Width(byte(c)); w != w2 {
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

var (
	ErrDuplicateCode = errors.New("duplicate code")
	ErrOverflow      = errors.New("too many glyphs")
)
