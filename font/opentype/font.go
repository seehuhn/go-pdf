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

package opentype

import (
	"golang.org/x/text/language"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/encoding/cidenc"
	"seehuhn.de/go/pdf/font/internal/vfinstance"
	"seehuhn.de/go/sfnt"
)

// OptionsSimple contains options for creating a simple OpenType font.
type OptionsSimple struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	// Variations pins the axes of a variable font before embedding.  Keys are
	// variation axis tags; omitted axes keep their default value.  A variable
	// font is always instanced, even when this is nil.
	Variations map[string]float64
}

// OptionsComposite contains options for creating a composite OpenType font.
type OptionsComposite struct {
	Language     language.Tag
	GsubFeatures map[string]bool
	GposFeatures map[string]bool

	WritingMode  font.WritingMode
	MakeGIDToCID func() cmap.GIDToCID
	MakeEncoder  func(cid0Width float64, wMode font.WritingMode) cidenc.CIDEncoder

	// Variations pins the axes of a variable font before embedding.  Keys are
	// variation axis tags; omitted axes keep their default value.  A variable
	// font is always instanced, even when this is nil.
	Variations map[string]float64
}

// NewSimple creates a simple OpenType font from an sfnt.Font.
// The function automatically chooses between SimpleGlyf and SimpleCFF
// based on the font's outline format.
func NewSimple(info *sfnt.Font, opt *OptionsSimple) (font.Layouter, error) {
	if opt == nil {
		opt = &OptionsSimple{}
	}

	info, err := vfinstance.Apply(info, opt.Variations)
	if err != nil {
		return nil, err
	}

	if info.IsCFF() {
		return newSimpleCFF(info, opt)
	}
	return newSimpleGlyf(info, opt)
}

// NewComposite creates a composite OpenType font from an sfnt.Font.
// The function automatically chooses between CompositeGlyf and CompositeCFF
// based on the font's outline format.
func NewComposite(info *sfnt.Font, opt *OptionsComposite) (font.Layouter, error) {
	if opt == nil {
		opt = &OptionsComposite{}
	}

	info, err := vfinstance.Apply(info, opt.Variations)
	if err != nil {
		return nil, err
	}

	if info.IsCFF() {
		return newCompositeCFF(info, opt)
	}
	return newCompositeGlyf(info, opt)
}
