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

package annotation

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.4

// BorderEffect represents a border effect dictionary that specifies
// an effect applied to an annotation's border.
type BorderEffect struct {
	// Style is the border effect style.
	//  - "S" (Solid): no effect.
	//  - "C" (Cloudy): border effect.
	//
	// When writing annotations, an empty Style value can be used
	// as a shorthand for "S".
	Style pdf.Name

	// Intensity (meaningful only when Style is "C") specifies
	// the intensity of the cloudy border effect.
	// Valid range is 0.0 to 2.0.
	Intensity float64

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*BorderEffect)(nil)

func ExtractBorderEffect(x *pdf.Extractor, obj pdf.Object) (*BorderEffect, error) {
	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing border effect dictionary")
	}

	effect := &BorderEffect{}

	if style, err := pdf.Optional(x.GetName(dict["S"])); err != nil {
		return nil, err
	} else if style != "" {
		effect.Style = style
	} else { // default to solid
		effect.Style = "S"
	}

	if effect.Style == "C" {
		if intensity, err := pdf.Optional(x.GetNumber(dict["I"])); err != nil {
			return nil, err
		} else if intensity >= 0.0 && intensity <= 2.0 {
			effect.Intensity = float64(intensity)
		} else {
			effect.Intensity = 1.0
		}
	}

	_, isIndirect := obj.(pdf.Reference)
	effect.SingleUse = !isIndirect && !x.IsIndirect

	return effect, nil
}

func (be *BorderEffect) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "border effect dictionary", pdf.V1_5); err != nil {
		return nil, err
	}

	d := pdf.Dict{}

	if be.Style != "S" && be.Style != "" {
		d["S"] = be.Style
	}

	if be.Style == "C" {
		if be.Intensity < 0.0 || be.Intensity > 2.0 {
			return nil, errors.New("invalid Intensity value")
		} else if be.Intensity != 0 {
			d["I"] = pdf.Number(be.Intensity)
		}
	} else {
		if be.Intensity != 0 {
			return nil, errors.New("unexpected Intensity value")
		}
	}

	if be.SingleUse {
		return d, nil
	}
	ref := rm.Alloc()
	err := rm.Out().Put(ref, d)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
