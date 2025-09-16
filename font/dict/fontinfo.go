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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/pdf/font/glyphdata"
)

// FontInfoSimple holds information about a simple font (Type1 or TrueType).
type FontInfoSimple struct {
	// PostScriptName is the PostScript name of the font.
	PostScriptName string

	// FontFile contains the embedded font file stream.
	// If the font is not embedded, this is nil.
	FontFile *glyphdata.Stream

	// Encoding is the font's character encoding.
	Encoding encoding.Simple

	IsSymbolic bool
}

// FontInfoCID holds information about a CID-keyed font.
type FontInfoCID struct {
	// PostScriptName is the PostScript name of the font.
	PostScriptName string

	// FontFile contains the embedded font file stream.
	// If the font is not embedded, this is nil.
	FontFile *glyphdata.Stream

	// CIDIsUsed maps CIDs to a boolean indicating if the CID is used in the font.
	CIDIsUsed map[cid.CID]bool
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
}

// FontInfoType3 holds information specific to a Type 3 font.
type FontInfoType3 struct {
	// CharProcs maps the name of each glyph to the content stream which paints
	// the glyph for that character.
	CharProcs map[pdf.Name]pdf.Reference

	// The FontMatrix maps glyph space to text space.
	FontMatrix matrix.Matrix

	// Resources (optional) holds named resources directly used by content
	// streams referenced by CharProcs, when the content stream does not itself
	// have a resource dictionary.
	//
	// TODO(voss): Should this be a pdf.Object instead, so that an
	// indirect reference can be used on writing?
	Resources *pdf.Resources
}
