// Copyright 2021 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package font

import (
	"fmt"
	"iter"

	"seehuhn.de/go/pdf/font/charcode"
	"seehuhn.de/go/postscript/cid"
	"seehuhn.de/go/sfnt/glyph"

	"seehuhn.de/go/pdf"
)

// Instance represents a font as used in a PDF content stream.
//
// Instances may be shared across goroutines.  Implementations used for
// reading are safe for concurrent use.  For an instance used for writing,
// [Instance.Codes] and the other per-code accessors may be called by any
// number of goroutines while one goroutine allocates codes through
// [Layouter.Encode] or [Layouter.Layout]; each call sees the codes allocated
// at the time it was made.
//
// Building the font dictionary is a writer-side operation, and it combines
// several such views, so a code allocated while it runs can leave the result
// inconsistent.  [Instance.FontInfo] builds the dictionary when called, but
// the Embed method only defers the work to the resource manager: the
// dictionary is built when the resource manager is closed, so that it picks
// up every code allocated before the file is written.  A caller which writes
// the file from another goroutine must therefore stop allocating codes from
// before Embed until the resource manager has been closed.
//
// To extract a font from a PDF file, use
// [seehuhn.de/go/pdf/graphics/extract.Font].
type Instance interface {
	// PostScriptName returns a human-readable name for the font.
	// For most font types, this is the PostScript name of the font.
	PostScriptName() string

	// WritingMode returns whether the font is used for horizontal or vertical
	// writing.
	WritingMode() WritingMode

	// Codec returns the codec for the encoding used by this font.
	Codec() *charcode.Codec

	// Codes iterates over the character codes in a PDF string, yielding
	// information about each character code.
	Codes(s pdf.String) iter.Seq[Code]

	// FontInfo returns information required to load the font file and to
	// extract the the glyph corresponding to a character identifier. The
	// result is a pointer to one of the FontInfo* types defined in the
	// font/dict package.
	//
	// For an instance used for writing, this must be called from the
	// goroutine which allocates codes.
	FontInfo() any

	// ResourceName returns the font's preferred resource-dictionary key.
	// When non-empty, this must equal the key under which this font is
	// referenced in the /Font subdictionary of the current resource
	// dictionary (PDF 2.0 Table 109 / 110).  When empty, the builder is
	// free to allocate a name.
	ResourceName() pdf.Name

	pdf.Embedder
}

type Layouter interface {
	Instance

	// Encode converts a glyph ID to a character code (for use with the
	// instance's codec).  The text argument is a hint for choosing
	// an appropriate text representation for the character code, in case
	// a new code is allocated.  The glyph's width is taken from the font.
	//
	// The function returns the character code, and a boolean indicating
	// whether the encoding was successful.  If the function returns false, the
	// glyph ID cannot be encoded with this font instance.
	//
	// Use the Codec to append the character code to PDF strings.
	Encode(gid glyph.ID, text string) (charcode.Code, bool)

	// CodesRemaining returns the number of character codes that can still be
	// allocated in this font instance.
	CodesRemaining() int

	// Layout appends a string to a glyph sequence.  The string is typeset at
	// the given point size and the resulting GlyphSeq is returned.
	//
	// If seq is nil, a new glyph sequence is allocated.  If seq is not
	// nil, the return value is guaranteed to be equal to seq.
	Layout(seq *GlyphSeq, ptSize float64, s string) *GlyphSeq

	// GetGeometry returns font metrics required for typesetting.
	GetGeometry() *Geometry

	// IsBlank reports whether the glyph with the given ID is a blank glyph.
	IsBlank(gid glyph.ID) bool
}

// WritingMode is the "writing mode" of a PDF font (horizontal or vertical).
type WritingMode int

func (m WritingMode) String() string {
	switch m {
	case Horizontal:
		return "horizontal"
	case Vertical:
		return "vertical"
	default:
		return fmt.Sprintf("WritingMode(%d)", m)
	}
}

const (
	// Horizontal indicates horizontal writing mode.
	Horizontal WritingMode = 0

	// Vertical indicates vertical writing mode.
	Vertical WritingMode = 1
)

// InstancesEqual compares two font instances for semantic equality.
// Two fonts are equal if they have the same PostScript name, writing mode,
// and code space range.  Code space ranges are compared as sets of ranges:
// the order of the ranges and repeated ranges do not matter, but two code
// spaces which cover the same codes with differently split ranges compare as
// different.
//
// The comparison is deliberately structural rather than exhaustive:
// instances with the same PostScript name share the embedded font
// program, so per-glyph differences (e.g. from variation axes applied
// unequally) cannot occur between equally named instances produced by
// this library.  Callers needing byte-level identity of encoded output
// should compare the embedded programs instead.
func InstancesEqual(a, b Instance) bool {
	if a == nil || b == nil {
		return a == b
	}

	if a.PostScriptName() != b.PostScriptName() {
		return false
	}

	if a.WritingMode() != b.WritingMode() {
		return false
	}

	codecA, codecB := a.Codec(), b.Codec()
	if (codecA == nil) != (codecB == nil) {
		return false
	}
	if codecA != nil && !codecA.Equal(codecB) {
		return false
	}

	return true
}

type Code struct {
	// CID allows to look up the glyph in the underlying font.
	CID cid.CID

	// Notdef specifies which glyph to show if the requested glyph is not
	// present in the font.
	Notdef cid.CID

	// Width is the horizontal glyph width in PDF text space units.  This
	// still needs to be scaled by the font size.  In horizontal writing
	// mode this is also the displacement applied to the text matrix after
	// the glyph is shown.
	Width float64

	// VerticalAdvance is the vertical component of the glyph displacement
	// in vertical writing mode, in PDF text space units.  This still needs to
	// be scaled by the font size.  It is the displacement applied to the
	// text matrix after the glyph is shown, and is normally negative, since
	// vertical text advances downward.  Zero means that the text position
	// does not move.  In horizontal writing mode this field is unused.
	VerticalAdvance float64

	// VerticalOriginX and VerticalOriginY give the position vector of the
	// glyph in vertical writing mode, in PDF text space units: the position
	// of the vertical glyph origin, relative to the horizontal glyph origin.
	// These still need to be scaled by the font size.  In vertical writing
	// mode the current text position is the vertical origin, so the glyph
	// outline is painted shifted by minus this vector.  In horizontal writing
	// mode these fields are unused.
	VerticalOriginX float64
	VerticalOriginY float64

	// Text is the textual representation of the character.
	Text string

	// UseWordSpacing indicates whether PDF word spacing is added for this
	// code. This is true if and only if the character code is a single byte
	// with the value 0x20 (irrespective of whether the character actually
	// represents a space).
	UseWordSpacing bool
}

// Must is a helper that wraps a call to a font constructor returning a
// [Layouter] and an error, and panics if the error is non-nil. It is intended
// for use in variable initialisations such as
//
//	var F = font.Must(cff.NewSimple(info, nil))
func Must[T Layouter](font T, err error) T {
	if err != nil {
		panic(err)
	}
	return font
}
