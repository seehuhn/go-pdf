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

package softclip

import (
	"errors"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/form"
)

// PDF 2.0 sections: 11.6.5.1

// Type specifies how mask values are derived from a transparency group.
type Type uint8

const (
	// Alpha indicates that the group's computed alpha shall be used,
	// disregarding its colour.
	Alpha Type = 0

	// Luminosity indicates that the group's computed colour shall be
	// converted to a single-component luminosity value.
	Luminosity Type = 1
)

// Mask represents a soft-mask dictionary (PDF 1.4, section 11.6.5.1).
// Use a nil *Mask to represent /None (absence of soft mask).
type Mask struct {
	// S specifies the derivation method: Alpha or Luminosity.
	S Type

	// G is the transparency group XObject used as the mask source.
	// G.Group must be non-nil (i.e., G must be a transparency group XObject).
	G *form.Form

	// BC is the backdrop color for Luminosity masks (optional).
	// Length must match the number of color space components in G.
	BC []float64

	// TR is the transfer function (optional).
	// nil means identity (the default).
	TR pdf.Function
}

// Embed implements the [pdf.Embedder] interface.
func (m *Mask) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "soft mask", pdf.V1_4); err != nil {
		return nil, err
	}

	if m.G == nil {
		return nil, errors.New("soft mask: missing transparency group (G)")
	}
	if m.G.Group == nil {
		return nil, errors.New("soft mask: G must be a transparency group XObject")
	}

	dict := pdf.Dict{}
	if rm.Out().GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Mask")
	}

	// S - subtype (required)
	switch m.S {
	case Alpha:
		dict["S"] = pdf.Name("Alpha")
	case Luminosity:
		dict["S"] = pdf.Name("Luminosity")
	default:
		return nil, errors.New("soft mask: invalid subtype")
	}

	// G - transparency group XObject (required)
	gRef, err := rm.Embed(m.G)
	if err != nil {
		return nil, err
	}
	dict["G"] = gRef

	// BC - backdrop color (optional, Luminosity only)
	if m.S == Luminosity && len(m.BC) > 0 {
		bc := make(pdf.Array, len(m.BC))
		for i, v := range m.BC {
			bc[i] = pdf.Number(v)
		}
		dict["BC"] = bc
	}

	// TR - transfer function (optional)
	if m.TR != nil {
		trObj, err := rm.Embed(m.TR)
		if err != nil {
			return nil, err
		}
		dict["TR"] = trObj
	}

	return dict, nil
}

// Equal reports whether two Mask values are equal.
func (m *Mask) Equal(other *Mask) bool {
	if m == nil || other == nil {
		return m == nil && other == nil
	}

	if m.S != other.S {
		return false
	}

	if (m.G == nil) != (other.G == nil) {
		return false
	}
	if m.G != nil && !m.G.Equal(other.G) {
		return false
	}

	if !slices.Equal(m.BC, other.BC) {
		return false
	}

	if !function.Equal(m.TR, other.TR) {
		return false
	}

	return true
}
