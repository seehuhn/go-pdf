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
	"golang.org/x/text/language"

	pstype1 "seehuhn.de/go/postscript/type1"

	scff "seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/gofont"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
)

// FontSample is an example of a font of the given [EmbeddingType].
type FontSample struct {
	Font font.Font
	Type font.EmbeddingType
}

// MakeFonts generates a list of different fonts for testing.
//
// TODO(voss): remove the error return value and panic instead.
func MakeFonts() ([]FontSample, error) {
	var res []FontSample
	var F font.Font

	opt := &font.Options{
		Language: language.English,
	}

	// a Type 1 font
	t1, err := gofont.Type1(gofont.GoRegular)
	if err != nil {
		return nil, err
	}
	F, err = type1.New(t1)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.Type1})

	// a built-in font
	res = append(res, FontSample{type1.Helvetica, font.Builtin})

	// a CFF font, embedded directly ...
	otf, err := gofont.OpenType(gofont.GoRegular)
	if err != nil {
		return nil, err
	}
	F, err = cff.NewSimple(otf, opt)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.CFFSimple})

	// ... or with the OpenType wrapper
	F, err = opentype.NewCFFSimple(otf, opt)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.OpenTypeCFFSimple})

	// a TrueType font, embedded directly ...
	ttf, err := gofont.TrueType(gofont.GoRegular)
	if err != nil {
		return nil, err
	}
	F, err = truetype.NewSimple(ttf, opt)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.TrueTypeSimple})

	// ... or using an OpenType wrapper
	F, err = opentype.NewGlyfSimple(ttf, opt)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.OpenTypeGlyfSimple})

	// a Type 3 font
	F, err = gofont.Type3(gofont.GoRegular)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.Type3})

	// a CFF font without CIDFont operators, embedded directly ...
	otf, err = gofont.OpenType(gofont.GoRegular)
	if err != nil {
		return nil, err
	}
	outlines := otf.Outlines.(*scff.Outlines)
	if len(outlines.Encoding) != 256 || outlines.ROS != nil || len(outlines.GIDToCID) != 0 {
		panic("CFF font unexpectedly has CIDFont operators")
	}
	F, err = cff.NewComposite(otf, opt)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.CFFComposite})

	// ... or with the OpenType wrapper
	F, err = opentype.NewCFFComposite(otf, nil)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.OpenTypeCFFComposite})

	// a CFF font with CIDFont operators, embedded directly ...
	otf, err = gofont.OpenType(gofont.GoRegular) // allocate a new copy
	if err != nil {
		return nil, err
	}
	outlines = otf.Outlines.(*scff.Outlines) // convert to use CIDFont operators
	outlines.Encoding = nil
	outlines.ROS = &pstype1.CIDSystemInfo{
		Registry:   "Seehuhn",
		Ordering:   "Sonderbar",
		Supplement: 0,
	}
	outlines.GIDToCID = make([]pstype1.CID, len(outlines.Glyphs))
	for i := range outlines.GIDToCID {
		outlines.GIDToCID[i] = pstype1.CID(i)
	}
	F, err = cff.NewComposite(otf, opt)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.CFFComposite})

	// ... or with the OpenType wrapper
	F, err = opentype.NewCFFComposite(otf, nil)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.OpenTypeCFFComposite})

	// a TrueType font, embedded directly ...
	ttf, err = gofont.TrueType(gofont.GoRegular)
	if err != nil {
		return nil, err
	}
	F, err = truetype.NewComposite(ttf, nil)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.TrueTypeComposite})

	// ... or using an OpenType wrapper
	F, err = opentype.NewGlyfComposite(ttf, nil)
	if err != nil {
		return nil, err
	}
	res = append(res, FontSample{F, font.OpenTypeGlyfComposite})

	return res, nil
}
