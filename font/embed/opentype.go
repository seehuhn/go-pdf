// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
