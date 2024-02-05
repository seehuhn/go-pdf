// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package debug

import (
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/internal/testfont"
)

// FontSample is an example of a font of the given [EmbeddingType].
type FontSample struct {
	Label       string
	Description string
	Type        font.EmbeddingType
	Font        font.Embedder
}

// MakeFontSamples generates a list of different fonts for testing.
//
// TODO(voss): remove the error return value and panic instead.
func MakeFontSamples() ([]*FontSample, error) {
	var res []*FontSample
	var F font.Embedder

	// a Type 1 font
	res = append(res, &FontSample{
		Label:       "Type1",
		Description: "an embedded Type 1 font",
		Type:        font.Type1,
		Font:        testfont.Type1,
	})

	// a built-in font
	res = append(res, &FontSample{
		Label:       "BuiltIn",
		Description: "a built-in font",
		Type:        font.Type1,
		Font:        type1.Helvetica,
	})

	// a CFF font, embedded directly ...
	otf := testfont.MakeCFFFont()
	F, err := cff.New(otf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "CFFSimple",
		Description: "a simple CFF font",
		Type:        font.CFFSimple,
		Font:        F,
	})

	// ... or with the OpenType wrapper
	F, err = opentype.New(otf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "OpenTypeCFFSimple",
		Description: "a simple OpenType/CFF font",
		Type:        font.OpenTypeCFFSimple,
		Font:        F,
	})

	// a TrueType font, embedded directly ...
	ttf := testfont.MakeGlyfFont()
	F, err = truetype.New(ttf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "TrueTypeSimple",
		Description: "a simple TrueType font",
		Type:        font.TrueTypeSimple,
		Font:        F,
	})

	// ... or using an OpenType wrapper
	F, err = opentype.New(ttf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "OpenTypeGlyfSimple",
		Description: "a simple OpenType/glyf font",
		Type:        font.OpenTypeGlyfSimple,
		Font:        F,
	})

	// a Type 3 font
	F, err = testfont.MakeType3()
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "Type3",
		Description: "a Type 3 font",
		Type:        font.Type3,
		Font:        F,
	})

	// a CFF font without CIDFont operators, embedded directly ...
	otf = testfont.MakeCFFFont()
	// outlines := otf.Outlines.(*cff.Outlines)
	// if len(outlines.Encoding) != 256 || outlines.ROS != nil || len(outlines.GIDToCID) != 0 {
	// 	panic("CFF font unexpectedly has CIDFont operators")
	// }
	F, err = cff.New(otf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "CFFComposite1",
		Description: "a composite CFF font (no CIDFont operators)",
		Type:        font.CFFComposite,
		Font:        F,
	})

	// ... or with the OpenType wrapper
	F, err = opentype.New(otf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "OpenTypeCFFComposite1",
		Description: "a composite OpenType/CFF font (no CIDFont operators)",
		Type:        font.OpenTypeCFFComposite,
		Font:        F,
	})

	// a CFF font with CIDFont operators, embedded directly ...
	otf = testfont.MakeCFFCIDFont()
	F, err = cff.New(otf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "CFFComposite2",
		Description: "a composite CFF font with CIDFont operators",
		Type:        font.CFFComposite,
		Font:        F,
	})

	// ... or with the OpenType wrapper
	F, err = opentype.New(otf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "OpenTypeCFFComposite2",
		Description: "a composite OpenType/CFF font with CIDFont operators",
		Type:        font.OpenTypeCFFComposite,
		Font:        F,
	})

	// a TrueType font, embedded directly ...
	ttf = testfont.MakeGlyfFont()
	F, err = truetype.New(ttf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "TrueTypeComposite",
		Description: "an composite TrueType font",
		Type:        font.TrueTypeComposite,
		Font:        F,
	})

	// ... or using an OpenType wrapper
	F, err = opentype.New(ttf)
	if err != nil {
		return nil, err
	}
	res = append(res, &FontSample{
		Label:       "OpenTypeGlyfComposite",
		Description: "a composite OpenType/glyf font",
		Type:        font.OpenTypeGlyfComposite,
		Font:        F,
	})

	return res, nil
}
