// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package fonttypes

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Sample is an example of a font of the given [EmbeddingType].
//
// This type implements the [font.Font] interface.
type Sample struct {
	Label       string
	Description string
	Type        font.EmbeddingType
	Font        font.Font
}

// Embed implements the [font.Font] interface.
func (f *Sample) Embed(w pdf.Putter) (font.Layouter, error) {
	return f.Font.Embed(w)
}

// All is a list of example fonts, covering all supported font and
// embedding types.
var All = []*Sample{
	{
		Label:       "CFFSimple1",
		Description: "a simple CFF font",
		Type:        font.CFFSimple,
		Font:        CFFSimple,
	},
	{
		Label:       "CFFSimple2",
		Description: "a simple CFF font (with CIDFont operators)",
		Type:        font.CFFSimple,
		Font:        CFFCIDSimple,
	},
	{
		Label:       "OpenTypeCFFSimple1",
		Description: "a simple OpenType/CFF font",
		Type:        font.OpenTypeCFFSimple,
		Font:        OpenTypeCFFSimple,
	},
	{
		Label:       "OpenTypeCFFSimple2",
		Description: "a simple OpenType/CFF font (with CIDFont operators)",
		Type:        font.OpenTypeCFFSimple,
		Font:        OpenTypeCFFCIDSimple,
	},
	{
		Label:       "TrueTypeSimple",
		Description: "a simple TrueType font",
		Type:        font.TrueTypeSimple,
		Font:        TrueTypeSimple,
	},
	{
		Label:       "OpenTypeGlyfSimple",
		Description: "a simple OpenType/Glyf font",
		Type:        font.OpenTypeGlyfSimple,
		Font:        OpenTypeGlyfSimple,
	},
	{
		Label:       "BuiltIn",
		Description: "a built-in Type 1 font",
		Type:        font.Type1,
		Font:        Standard,
	},
	{
		Label:       "Type1",
		Description: "an embedded Type 1 font",
		Type:        font.Type1,
		Font:        Type1,
	},
	{
		Label:       "Type3",
		Description: "a Type 3 font",
		Type:        font.Type3,
		Font:        Type3,
	},

	{
		Label:       "CFFComposite1",
		Description: "a composite CFF font",
		Type:        font.CFFComposite,
		Font:        CFFComposite,
	},
	{
		Label:       "CFFComposite2",
		Description: "a composite CFF font (with CIDFont operators)",
		Type:        font.CFFComposite,
		Font:        CFFCIDComposite,
	},
	{
		Label:       "CFFComposite3",
		Description: "a composite CFF font (CIDFont operators and 2 private dicts)",
		Type:        font.CFFComposite,
		Font:        CFFCID2Composite,
	},
	{
		Label:       "OpenTypeCFFComposite1",
		Description: "a composite OpenType/CFF font",
		Type:        font.OpenTypeCFFComposite,
		Font:        OpenTypeCFFComposite,
	},
	{
		Label:       "OpenTypeCFFComposite2",
		Description: "a composite OpenType/CFF font (with CIDFont operators)",
		Type:        font.OpenTypeCFFComposite,
		Font:        OpenTypeCFFCIDComposite,
	},
	{
		Label:       "OpenTypeCFFComposite3",
		Description: "a composite OpenType/CFF font (CIDFont operators and 2 private dicts)",
		Type:        font.OpenTypeCFFComposite,
		Font:        OpenTypeCFFCID2Composite,
	},
	{
		Label:       "TrueTypeComposite",
		Description: "a composite TrueType font",
		Type:        font.TrueTypeComposite,
		Font:        TrueTypeComposite,
	},
	{
		Label:       "OpenTypeGlyfComposite",
		Description: "a composite OpenType/Glyf font",
		Type:        font.OpenTypeGlyfComposite,
		Font:        OpenTypeGlyfComposite,
	},
}
