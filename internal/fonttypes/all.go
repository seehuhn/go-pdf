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
type Sample struct {
	Label       string
	Description string
	Type        font.EmbeddingType
	MakeFont    func(*pdf.ResourceManager) font.Layouter
}

// All is a list of example fonts, covering all supported font and
// embedding types.
var All = []*Sample{
	{
		Label:       "CFFSimple1",
		Description: "a simple CFF font",
		Type:        font.CFFSimple,
		MakeFont:    CFFSimple,
	},
	{
		Label:       "CFFSimple2",
		Description: "a simple CFF font (with CIDFont operators)",
		Type:        font.CFFSimple,
		MakeFont:    CFFCIDSimple,
	},
	{
		Label:       "OpenTypeCFFSimple1",
		Description: "a simple OpenType/CFF font",
		Type:        font.OpenTypeCFFSimple,
		MakeFont:    OpenTypeCFFSimple,
	},
	{
		Label:       "OpenTypeCFFSimple2",
		Description: "a simple OpenType/CFF font (with CIDFont operators)",
		Type:        font.OpenTypeCFFSimple,
		MakeFont:    OpenTypeCFFCIDSimple,
	},
	{
		Label:       "TrueTypeSimple",
		Description: "a simple TrueType font",
		Type:        font.TrueTypeSimple,
		MakeFont:    TrueTypeSimple,
	},
	{
		Label:       "OpenTypeGlyfSimple",
		Description: "a simple OpenType/Glyf font",
		Type:        font.OpenTypeGlyfSimple,
		MakeFont:    OpenTypeGlyfSimple,
	},
	{
		Label:       "BuiltIn",
		Description: "a built-in Type 1 font",
		Type:        font.Type1,
		MakeFont:    Standard,
	},
	{
		Label:       "Type1a",
		Description: "an embedded Type 1 font with metrics",
		Type:        font.Type1,
		MakeFont:    Type1WithMetrics,
	},
	{
		Label:       "Type1b",
		Description: "an embedded Type 1 font without metrics",
		Type:        font.Type1,
		MakeFont:    Type1WithoutMetrics,
	},
	{
		Label:       "Type3",
		Description: "a Type 3 font",
		Type:        font.Type3,
		MakeFont:    Type3,
	},

	{
		Label:       "CFFComposite1",
		Description: "a composite CFF font",
		Type:        font.CFFComposite,
		MakeFont:    CFFComposite,
	},
	{
		Label:       "CFFComposite2",
		Description: "a composite CFF font (with CIDFont operators)",
		Type:        font.CFFComposite,
		MakeFont:    CFFCIDComposite,
	},
	{
		Label:       "CFFComposite3",
		Description: "a composite CFF font (CIDFont operators and 2 private dicts)",
		Type:        font.CFFComposite,
		MakeFont:    CFFCID2Composite,
	},
	{
		Label:       "OpenTypeCFFComposite1",
		Description: "a composite OpenType/CFF font",
		Type:        font.OpenTypeCFFComposite,
		MakeFont:    OpenTypeCFFComposite,
	},
	{
		Label:       "OpenTypeCFFComposite2",
		Description: "a composite OpenType/CFF font (with CIDFont operators)",
		Type:        font.OpenTypeCFFComposite,
		MakeFont:    OpenTypeCFFCIDComposite,
	},
	{
		Label:       "OpenTypeCFFComposite3",
		Description: "a composite OpenType/CFF font (CIDFont operators and 2 private dicts)",
		Type:        font.OpenTypeCFFComposite,
		MakeFont:    OpenTypeCFFCID2Composite,
	},
	{
		Label:       "TrueTypeComposite",
		Description: "a composite TrueType font",
		Type:        font.TrueTypeComposite,
		MakeFont:    TrueTypeComposite,
	},
	{
		Label:       "OpenTypeGlyfComposite",
		Description: "a composite OpenType/Glyf font",
		Type:        font.OpenTypeGlyfComposite,
		MakeFont:    OpenTypeGlyfComposite,
	},
}
