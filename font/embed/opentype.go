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

package embed

import (
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/internal/vfinstance"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/sfnt"
)

// Options controls how [OpenTypeFont] embeds a font.
type Options struct {
	// Language is the language used when selecting layout features.
	Language language.Tag

	// GsubFeatures enables or disables individual GSUB features.
	GsubFeatures map[string]bool

	// GposFeatures enables or disables individual GPOS features.
	GposFeatures map[string]bool

	// CMap, if non-nil, selects a composite font written with this CMap,
	// for example [cmap.IdentityH].  A nil CMap selects a simple font.
	CMap *cmap.File

	// Variations pins the axes of a variable font before embedding.  Keys are
	// variation axis tags; omitted axes keep their default value.  A variable
	// font is always instanced, even when this is nil.
	Variations map[string]float64
}

// OpenTypeFile loads and embeds an OpenType/TrueType font.
func OpenTypeFile(fname string, opt *Options) (font.Layouter, error) {
	info, err := sfnt.ReadFile(fname)
	if err != nil {
		return nil, err
	}

	return OpenTypeFont(info, opt)
}

// OpenTypeFont embeds an OpenType/TrueType font.
func OpenTypeFont(info *sfnt.Font, opt *Options) (font.Layouter, error) {
	if opt == nil {
		opt = &Options{}
	}

	info, err := vfinstance.Apply(info, opt.Variations)
	if err != nil {
		return nil, err
	}

	var F font.Layouter
	if info.IsCFF() {
		if opt.CMap != nil {
			o := &cff.OptionsComposite{
				Language:     opt.Language,
				GsubFeatures: opt.GsubFeatures,
				GposFeatures: opt.GposFeatures,
				CMap:         opt.CMap,
			}
			F, err = cff.NewComposite(info, o)
		} else {
			o := &cff.OptionsSimple{
				Language:     opt.Language,
				GsubFeatures: opt.GsubFeatures,
				GposFeatures: opt.GposFeatures,
			}
			F, err = cff.NewSimple(info, o)
		}
	} else {
		if opt.CMap != nil {
			o := &truetype.OptionsComposite{
				Language:     opt.Language,
				GsubFeatures: opt.GsubFeatures,
				GposFeatures: opt.GposFeatures,
				CMap:         opt.CMap,
			}
			F, err = truetype.NewComposite(info, o)
		} else {
			o := &truetype.OptionsSimple{
				Language:     opt.Language,
				GsubFeatures: opt.GsubFeatures,
				GposFeatures: opt.GposFeatures,
			}
			F, err = truetype.NewSimple(info, o)
		}
	}
	if err != nil {
		return nil, err
	}
	return F, nil
}
