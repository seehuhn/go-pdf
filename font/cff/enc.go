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

package cff

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"strings"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/postscript/type1/names"

	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/pdfenc"
)

const maxNameLength = 31

// GlyphData manages the encoding and metadata of glyphs for simple PDF fonts.
//
// It constructs a mapping from single-byte codes to
//   - character IDs (CIDs),
//   - glyph widths, and
//   - associated text content.
//
// If a glyph is used with different text content, different codes are used
// to allow for different ToUnicode mappings.
type GlyphData struct {
	code   map[glyphKey]byte
	info   map[byte]*codeInfo
	notdef *codeInfo

	glyphName     map[glyph.ID]string
	glyphNameUsed map[string]bool

	isZapfDingbats bool
	overflow       bool
	encoding       *pdfenc.Encoding
}

type glyphKey struct {
	gid  glyph.ID
	text string
}

type codeInfo struct {
	Width float64
	Text  string
}

// NewGlyphData creates a glyphData and registers the .notdef glyph.
func NewGlyphData(notdefWidth float64, isZapfDingbats bool, base *pdfenc.Encoding) *GlyphData {
	gd := &GlyphData{
		code:           make(map[glyphKey]byte),
		info:           make(map[byte]*codeInfo),
		notdef:         &codeInfo{Width: notdefWidth},
		glyphName:      make(map[glyph.ID]string),
		glyphNameUsed:  make(map[string]bool),
		isZapfDingbats: isZapfDingbats,
		encoding:       base,
	}
	gd.glyphName[0] = ".notdef"
	gd.glyphNameUsed[".notdef"] = true
	return gd
}

// Code looks up the code for a (gid,text) pair, returning the code byte
// and whether it already exists.
func (gd *GlyphData) Code(gid glyph.ID, text string) (byte, bool) {
	k := glyphKey{gid: gid, text: text}
	c, ok := gd.code[k]
	return c, ok
}

// Get returns the CID, width and text for the given code,
// The .notdef glyph is used for undefined codes.
func (gd *GlyphData) Get(c byte) (cid.CID, float64, string) {
	info, ok := gd.info[c]
	if !ok {
		info = gd.notdef
		return 0, info.Width, info.Text
	}
	glyphCID := cid.CID(c) + 1 // CID 0 is reserved for .notdef
	return glyphCID, info.Width, info.Text
}

var ErrOverflow = errors.New("too many glyphs")

// NewCode finds a new code for the combination of glyph ID and text.
//
// If baseGlyphName is a valid glyph name, it is used as the default name for
// the glyph.  The new code is chosen using a heuristic based on the glyph name
// and first rune in text.
//
// Only 256 codes are available. Once all codes are used, the function returns
// an error.
func (gd *GlyphData) NewCode(gid glyph.ID, baseGlyphName, text string, width float64) (byte, error) {
	key := glyphKey{gid: gid, text: text}
	if c, ok := gd.code[key]; ok {
		return c, nil
	}

	if len(gd.info) >= 256 {
		gd.overflow = true
		return 0, ErrOverflow
	}

	glyphName := gd.makeGlyphName(gid, baseGlyphName, text)

	var r rune
	rr := names.ToUnicode(glyphName, gd.isZapfDingbats)
	if len(rr) > 0 {
		r = rr[0]
	}

	bestScore := -1
	bestCode := byte(0)
	for codeInt := 0; codeInt < 256; codeInt++ {
		code := byte(codeInt)
		if _, used := gd.info[code]; used {
			continue
		}

		// We checked above that at most 255 codes are used, so we reach this
		// point at least once.

		score := 0
		stdName := gd.encoding.Encoding[code]
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

	gd.info[bestCode] = &codeInfo{Width: width, Text: text}
	gd.code[key] = bestCode

	return bestCode, nil
}

// makeGlyphName returns a unique name for the given glyph.
func (gd *GlyphData) makeGlyphName(gid glyph.ID, defaultGlyphName, text string) string {
	if name, ok := gd.glyphName[gid]; ok {
		return name
	}

	glyphName := defaultGlyphName
	if !isValid(glyphName) {
		var parts []string
		for _, r := range text {
			parts = append(parts, names.FromUnicode(r))
		}
		if len(parts) > 0 {
			glyphName = strings.Join(parts, "_")
		}
	}

	alt := 0
	base := glyphName
nameLoop:
	for {
		if isValid(glyphName) && !gd.glyphNameUsed[glyphName] {
			break
		}
		if len(base) == 0 || len(glyphName) > maxNameLength {
			// Try one more name than gd.glyphNameUsed has elements.
			// This guarantees that we find a free name.
			for idx := len(gd.glyphNameUsed); idx >= 0; idx-- {
				glyphName = fmt.Sprintf("orn%03d", idx) // at most 256 glyphs, so 3 digits are enough
				if !gd.glyphNameUsed[glyphName] {
					break nameLoop
				}
			}
		}
		alt++
		glyphName = fmt.Sprintf("%s.alt%d", base, alt)
	}
	gd.glyphName[gid] = glyphName
	gd.glyphNameUsed[glyphName] = true
	return glyphName
}

// isValid checks if s is a valid glyph name.
//
// See https://github.com/adobe-type-tools/agl-specification for details.
func isValid(s string) bool {
	if s == ".notdef" {
		return true
	}

	if len(s) < 1 || len(s) > maxNameLength {
		return false
	}

	firstChar := s[0]
	if (firstChar >= '0' && firstChar <= '9') || firstChar == '.' {
		return false
	}

	for _, char := range s {
		if !(char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' ||
			char >= '0' && char <= '9' || char == '.' || char == '_') {
			return false
		}
	}

	return true
}

func (gd *GlyphData) GlyphName(gid glyph.ID) string {
	return gd.glyphName[gid]
}

// Overflow checks if more than 256 codes are required.
func (gd *GlyphData) Overflow() bool {
	return gd.overflow
}

// Subset returns a sorted list of the glyphs used.
func (gd *GlyphData) Subset() []glyph.ID {
	gidIsUsed := make(map[glyph.ID]struct{})
	gidIsUsed[0] = struct{}{} // always include .notdef
	for k := range gd.code {
		gidIsUsed[k.gid] = struct{}{}
	}
	glyphs := maps.Keys(gidIsUsed)
	slices.Sort(glyphs)
	return glyphs
}

// Encoding returns the Type1 encoding corresponding to the glyph data.
func (gd *GlyphData) Encoding() encoding.Type1 {
	enc := make(map[byte]string)
	for k, c := range gd.code {
		enc[c] = gd.glyphName[k.gid]
	}
	return func(c byte) string { return enc[c] }
}

// DefaultWidth returns a good value for the DefaultWidth entry in the font
// descriptor.
func (gd *GlyphData) DefaultWidth() float64 {
	_, w1, _ := gd.Get(0)
	n1 := 1
	for c := 1; c < 256; c++ {
		if _, w, _ := gd.Get(byte(c)); w != w1 {
			break
		}
		n1++
	}

	_, w2, _ := gd.Get(255)
	n2 := 1
	for c := 254; c >= 0; c-- {
		if _, w, _ := gd.Get(byte(c)); w != w2 {
			break
		}
		n2++
	}

	if n1 >= n2 {
		return w1
	}
	return w2
}
