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
	"seehuhn.de/go/pdf/font/type1"
)

// Sample is an example of a font of the given [EmbeddingType].
type Sample struct {
	Label       string
	Description string
	Type        font.EmbeddingType
	Font        font.Font
}

// Embed implements the [font.Font] interface.
func (f *Sample) Embed(w pdf.Putter, opt *font.Options) (font.Layouter, error) {
	if opt == nil {
		opt = &font.Options{
			ResName:   pdf.Name(f.Label),
			Composite: f.Type.IsComposite(),
		}
	}
	return f.Font.Embed(w, opt)
}

// All is a list of example fonts, covering all supported font and
// embedding types.
var All = []*Sample{
	{
		Label:       "CFFSimple1",
		Description: "a simple CFF font",
		Type:        font.CFFSimple,
		Font:        CFF,
	},
	{
		Label:       "CFFSimple2",
		Description: "a simple CFF font (with CIDFont operators)",
		Type:        font.CFFSimple,
		Font:        CFFCID,
	},
	{
		Label:       "OpenTypeCFFSimple1",
		Description: "a simple OpenType/CFF font",
		Type:        font.OpenTypeCFFSimple,
		Font:        OpenTypeCFF,
	},
	{
		Label:       "OpenTypeCFFSimple2",
		Description: "a simple OpenType/CFF font (with CIDFont operators)",
		Type:        font.OpenTypeCFFSimple,
		Font:        OpenTypeCFFCID,
	},
	{
		Label:       "TrueTypeSimple",
		Description: "a simple TrueType font",
		Type:        font.TrueTypeSimple,
		Font:        TrueType,
	},
	{
		Label:       "OpenTypeGlyfSimple",
		Description: "a simple OpenType/Glyf font",
		Type:        font.OpenTypeGlyfSimple,
		Font:        OpenTypeGlyf,
	},
	{
		Label:       "BuiltIn",
		Description: "a built-in Type 1 font",
		Type:        font.Type1,
		Font:        type1.Helvetica,
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
		Font:        CFF,
	},
	{
		Label:       "CFFComposite2",
		Description: "a composite CFF font (with CIDFont operators)",
		Type:        font.CFFComposite,
		Font:        CFFCID,
	},
	{
		Label:       "CFFComposite3",
		Description: "a composite CFF font (CIDFont operators and 2 private dicts)",
		Type:        font.CFFComposite,
		Font:        CFFCID2,
	},
	{
		Label:       "OpenTypeCFFComposite1",
		Description: "a composite OpenType/CFF font",
		Type:        font.OpenTypeCFFComposite,
		Font:        OpenTypeCFF,
	},
	{
		Label:       "OpenTypeCFFComposite2",
		Description: "a composite OpenType/CFF font (with CIDFont operators)",
		Type:        font.OpenTypeCFFComposite,
		Font:        OpenTypeCFFCID,
	},
	{
		Label:       "OpenTypeCFFComposite3",
		Description: "a composite OpenType/CFF font (CIDFont operators and 2 private dicts)",
		Type:        font.OpenTypeCFFComposite,
		Font:        OpenTypeCFFCID2,
	},
	{
		Label:       "TrueTypeComposite",
		Description: "a composite TrueType font",
		Type:        font.TrueTypeComposite,
		Font:        TrueType,
	},
	{
		Label:       "OpenTypeGlyfComposite",
		Description: "a composite OpenType/Glyf font",
		Type:        font.OpenTypeGlyfComposite,
		Font:        OpenTypeGlyf,
	},
}
