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

package appearance

import (
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.8.3.2

// IconFit describes how a button annotation's icon is positioned and scaled
// within its annotation rectangle.
type IconFit struct {
	// ScaleWhen specifies under which circumstances the icon is scaled.
	// If set, it must be one of [IconScaleAlways], [IconScaleWhenBigger],
	// [IconScaleWhenSmaller], or [IconScaleNever].
	// When encoding, an empty value is omitted.
	ScaleWhen IconScaleWhen

	// Scaling specifies the type of scaling used.
	// If set, it must be one of [IconScalingAnamorphic] or
	// [IconScalingProportional].
	// When encoding, an empty value is omitted.
	Scaling IconScaling

	// LeftoverSpace (optional) gives the fraction of leftover space to allocate
	// at the left and bottom of the icon. Both values are between 0 and 1.
	// It applies only to proportional scaling.
	LeftoverSpace *[2]float64

	// FitToBounds (PDF 1.5), if set, scales the icon to fit within the
	// annotation bounds without regard to the border line width.
	FitToBounds bool

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*IconFit)(nil)

// ExtractIconFit reads an icon fit dictionary from the PDF object obj.
// If obj is absent or resolves to null, ExtractIconFit returns (nil, nil).
func ExtractIconFit(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*IconFit, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	res := &IconFit{SingleUse: isDirect}

	if sw, err := pdf.Optional(x.GetName(path, dict["SW"])); err != nil {
		return nil, err
	} else {
		res.ScaleWhen = IconScaleWhen(sw)
	}

	if s, err := pdf.Optional(x.GetName(path, dict["S"])); err != nil {
		return nil, err
	} else {
		res.Scaling = IconScaling(s)
	}

	if a, err := pdf.Optional(x.GetArray(path, dict["A"])); err != nil {
		return nil, err
	} else if len(a) == 2 {
		v0, err0 := pdf.GetNumber(x.R, a[0])
		v1, err1 := pdf.GetNumber(x.R, a[1])
		if err0 == nil && err1 == nil &&
			v0 >= 0 && v0 <= 1 && v1 >= 0 && v1 <= 1 {
			res.LeftoverSpace = &[2]float64{float64(v0), float64(v1)}
		}
	}

	if fb, err := pdf.Optional(x.GetBoolean(path, dict["FB"])); err != nil {
		return nil, err
	} else {
		res.FitToBounds = bool(fb)
	}

	return res, nil
}

func (f *IconFit) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "icon fit dictionary", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	if f.ScaleWhen != "" {
		dict["SW"] = pdf.Name(f.ScaleWhen)
	}
	if f.Scaling != "" {
		dict["S"] = pdf.Name(f.Scaling)
	}
	if f.LeftoverSpace != nil {
		a, b := f.LeftoverSpace[0], f.LeftoverSpace[1]
		if a < 0 || a > 1 || b < 0 || b > 1 {
			return nil, errors.New("icon fit leftover space out of range")
		}
		dict["A"] = pdf.Array{pdf.Number(a), pdf.Number(b)}
	}
	if f.FitToBounds {
		if err := pdf.CheckVersion(e.Out(), "icon fit FB entry", pdf.V1_5); err != nil {
			return nil, err
		}
		dict["FB"] = pdf.Boolean(true)
	}

	if f.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// IconScaleWhen specifies the circumstances under which a button icon is
// scaled to fit the annotation rectangle.
type IconScaleWhen pdf.Name

const (
	IconScaleAlways      IconScaleWhen = "A" // always scale
	IconScaleWhenBigger  IconScaleWhen = "B" // scale only when bigger than the rectangle
	IconScaleWhenSmaller IconScaleWhen = "S" // scale only when smaller than the rectangle
	IconScaleNever       IconScaleWhen = "N" // never scale
)

// IconScaling specifies the type of scaling used to fit a button icon into
// the annotation rectangle.
type IconScaling pdf.Name

const (
	IconScalingAnamorphic   IconScaling = "A" // fill the rectangle, ignoring aspect ratio
	IconScalingProportional IconScaling = "P" // preserve the icon's aspect ratio
)
