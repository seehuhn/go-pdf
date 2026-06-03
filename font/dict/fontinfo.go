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

package dict

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/postscript/cid"

	"seehuhn.de/go/sfnt/glyph"
	"seehuhn.de/go/sfnt/os2"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
	"seehuhn.de/go/pdf/graphics/content"
)

// FontInfoSimple holds information about a simple font (Type 1, TrueType,
// or simple CFF).
//
// To resolve a code byte to a specific glyph in the embedded font, dispatch
// on FontFile.Type and use the matching helper:
//
//   - Type1:                        [type1glyphs.FromStream]
//   - CFFSimple, OpenTypeCFFSimple: [cffglyphs.FromStream]
//   - TrueType, OpenTypeGlyf:       [sfntglyphs.NewTrueTypeSelector]
//
// Codes for which Encoding(code) returns [encoding.UseBuiltin] are resolved
// through that helper.
type FontInfoSimple struct {
	// PostScriptName is the PostScript name of the font.
	PostScriptName string

	// FontFile contains the embedded font file stream.
	// If the font is not embedded, this is nil.
	FontFile *glyphdata.Stream

	// Encoding is the font's character encoding.
	Encoding encoding.Simple

	// IsSymbolic is true if some glyphs are outside the Adobe Standard Latin
	// character set (see [pdfenc.StandardLatin]).
	IsSymbolic bool

	// IsSerif is true if the glyphs have serifs.
	IsSerif bool

	// IsFixedPitch is true if all glyphs have the same width.
	IsFixedPitch bool

	// IsItalic is true if the glyphs are slanted.
	IsItalic bool

	// FontWeight is the visual weight (thickness) of the glyphs.
	// A value of 0 means the weight is unknown.
	FontWeight os2.Weight
}

// FontInfoCID holds information about a CID-keyed font,
// used for Type 0 CIDFonts.
type FontInfoCID struct {
	// PostScriptName is the PostScript name of the font.
	PostScriptName string

	// FontFile contains the embedded font file stream.
	// If the font is not embedded, this is nil.
	FontFile *glyphdata.Stream

	// IsSerif is true if the glyphs have serifs.
	IsSerif bool

	// IsFixedPitch is true if all glyphs have the same width.
	IsFixedPitch bool

	// IsItalic is true if the glyphs are slanted.
	IsItalic bool

	// FontWeight is the visual weight (thickness) of the glyphs.
	// A value of 0 means the weight is unknown.
	FontWeight os2.Weight
}

// FontInfoGlyfEmbedded holds information about an embedded TrueType font
// program (glyf table), used for Type 2 CIDFonts.
type FontInfoGlyfEmbedded struct {
	// PostScriptName is the PostScript name of the font.
	PostScriptName string

	// FontFile contains the embedded font file stream.
	FontFile *glyphdata.Stream

	// CIDToGID maps CIDs to Glyph IDs (GIDs) for the embedded TrueType font.
	CIDToGID []glyph.ID
}

// FontInfoGlyfExternal holds information about an external TrueType font
// program, used for Type 2 CIDFonts.
type FontInfoGlyfExternal struct {
	// PostScriptName is the PostScript name of the font.
	PostScriptName string

	// ROS describes the character collection (Registry, Ordering, Supplement)
	// covered by the font. This must correspond to one of the predefined PDF CMaps.
	ROS *cid.SystemInfo

	// IsSerif is true if the glyphs have serifs.
	IsSerif bool

	// IsFixedPitch is true if all glyphs have the same width.
	IsFixedPitch bool

	// IsItalic is true if the glyphs are slanted.
	IsItalic bool

	// FontWeight is the visual weight (thickness) of the glyphs.
	// A value of 0 means the weight is unknown.
	FontWeight os2.Weight
}

// FontInfoType3 holds information specific to a Type 3 font.
type FontInfoType3 struct {
	// CharProcs maps glyph names to their content streams.
	CharProcs map[pdf.Name]*CharProc

	// The FontMatrix maps glyph space to text space.
	FontMatrix matrix.Matrix

	// Resources (optional) holds named resources shared by all glyph content
	// streams that don't have their own resource dictionary.
	Resources *content.Resources

	// Encoding maps character codes to glyph names.
	Encoding encoding.Simple

	// FontBBox (optional) is the font bounding box in glyph space units.
	FontBBox *pdf.Rectangle
}
