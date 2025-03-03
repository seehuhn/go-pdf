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
	"seehuhn.de/go/pdf/font"
)

// Sample is an example of a font of the given [EmbeddingType].
type Sample struct {
	Label       string
	Description string
	MakeFont    func() font.Layouter
	Composite   bool
}

// All is a list of example fonts, covering all supported font and
// embedding types.
var All = []*Sample{
	{
		Label:       "CFFSimple1",
		Description: "a simple CFF font",
		MakeFont:    CFFSimple,
		Composite:   false,
	},
	{
		Label:       "CFFSimple2",
		Description: "a simple CFF font (with CIDFont operators)",
		MakeFont:    CFFCIDSimple,
		Composite:   false,
	},
	{
		Label:       "OpenTypeCFFSimple1",
		Description: "a simple OpenType/CFF font",
		MakeFont:    OpenTypeCFFSimple,
		Composite:   false,
	},
	{
		Label:       "OpenTypeCFFSimple2",
		Description: "a simple OpenType/CFF font (with CIDFont operators)",
		MakeFont:    OpenTypeCFFCIDSimple,
		Composite:   false,
	},
	{
		Label:       "TrueTypeSimple",
		Description: "a simple TrueType font",
		MakeFont:    TrueTypeSimple,
		Composite:   false,
	},
	{
		Label:       "OpenTypeGlyfSimple",
		Description: "a simple OpenType/Glyf font",
		MakeFont:    OpenTypeGlyfSimple,
		Composite:   false,
	},
	{
		Label:       "Standard",
		Description: "a standard Type 1 font",
		MakeFont:    Standard,
		Composite:   false,
	},
	{
		Label:       "Type1a",
		Description: "an embedded Type 1 font with metrics",
		MakeFont:    Type1WithMetrics,
		Composite:   false,
	},
	{
		Label:       "Type1b",
		Description: "an embedded Type 1 font without metrics",
		MakeFont:    Type1WithoutMetrics,
		Composite:   false,
	},
	{
		Label:       "Type3",
		Description: "a Type 3 font",
		MakeFont:    Type3,
		Composite:   false,
	},

	{
		Label:       "CFFComposite1",
		Description: "a composite CFF font",
		MakeFont:    CFFComposite,
		Composite:   true,
	},
	{
		Label:       "CFFComposite2",
		Description: "a composite CFF font (with CIDFont operators)",
		MakeFont:    CFFCIDComposite,
		Composite:   true,
	},
	{
		Label:       "CFFComposite3",
		Description: "a composite CFF font (CIDFont operators and 2 private dicts)",
		MakeFont:    CFFCID2Composite,
		Composite:   true,
	},
	{
		Label:       "OpenTypeCFFComposite1",
		Description: "a composite OpenType/CFF font",
		MakeFont:    OpenTypeCFFComposite,
		Composite:   true,
	},
	{
		Label:       "OpenTypeCFFComposite2",
		Description: "a composite OpenType/CFF font (with CIDFont operators)",
		MakeFont:    OpenTypeCFFCIDComposite,
		Composite:   true,
	},
	{
		Label:       "OpenTypeCFFComposite3",
		Description: "a composite OpenType/CFF font (CIDFont operators and 2 private dicts)",
		MakeFont:    OpenTypeCFFCID2Composite,
		Composite:   true,
	},
	{
		Label:       "TrueTypeComposite",
		Description: "a composite TrueType font",
		MakeFont:    TrueTypeComposite,
		Composite:   true,
	},
	{
		Label:       "OpenTypeGlyfComposite",
		Description: "a composite OpenType/Glyf font",
		MakeFont:    OpenTypeGlyfComposite,
		Composite:   true,
	},
}
