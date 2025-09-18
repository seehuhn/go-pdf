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

package halftone

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
)

// PDF 2.0 sections: 10.6.3 10.6.5.1 10.6.5.2

// Type1 represents a Type 1 halftone dictionary that defines screens using
// frequency, angle, and spot function parameters.
type Type1 struct {
	// HalftoneName (optional) is the name of the halftone dictionary.
	HalftoneName string

	// Frequency is the screen frequency (halftone cells per inch).
	Frequency float64

	// Angle is the screen angle in (degrees counterclockwise) relative to the
	// device coordinate system.
	Angle float64

	// SpotFunction defines the pixel adjustment order for gray levels. This
	// can either be a predefined spot function (e.g., [SimpleDot], [Round],
	// [Ellipse]) or a custom function object.  The function must map
	// [-1, 1]x[-1, 1] to [-1, 1].
	SpotFunction pdf.Function

	// AccurateScreens enables a more precise but computationally expensive
	// halftone algorithm.
	AccurateScreens bool

	// TransferFunction (optional) overrides the current transfer function for
	// this component. Use pdf.Name("Identity") for the identity function.
	TransferFunction pdf.Object
}

var _ graphics.Halftone = (*Type1)(nil)

// HalftoneType returns 1.
// This implements the [graphics.Halftone] interface.
func (h *Type1) HalftoneType() int {
	return 1
}

func (h *Type1) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "halftone screening", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	if h.HalftoneName != "" {
		// If HalftoneName is provided, all other fields become optional.
		if h.Frequency < 0 {
			return nil, zero, fmt.Errorf("invalid halftone frequency %g", h.Frequency)
		}
	} else {
		if h.Frequency <= 0 {
			return nil, zero, fmt.Errorf("invalid halftone frequency %g", h.Frequency)
		}
		if h.SpotFunction == nil {
			return nil, zero, errors.New("missing spot function")
		}
	}

	var spotObj pdf.Object
	if spot := h.SpotFunction; spot != nil {
		nIn, nOut := spot.Shape()
		if nIn != 2 || nOut != 1 {
			return nil, zero, fmt.Errorf("wrong spot function shape %dx%d != 2x1", nIn, nOut)
		}

		if obj, ok := spotToName[spot]; ok {
			spotObj = obj
		} else {
			obj, _, err := pdf.ResourceManagerEmbed(rm, spot)
			if err != nil {
				return nil, zero, err
			}
			spotObj = obj
		}
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(1),
		"Frequency":    pdf.Number(h.Frequency),
		"Angle":        pdf.Number(h.Angle),
	}
	if spotObj != nil {
		dict["SpotFunction"] = spotObj
	}

	// Add optional fields
	opt := rm.Out.GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if h.HalftoneName != "" {
		dict["HalftoneName"] = pdf.String(h.HalftoneName)
	}

	if h.AccurateScreens {
		dict["AccurateScreens"] = pdf.Boolean(true)
	}

	if h.TransferFunction != nil {
		dict["TransferFunction"] = h.TransferFunction
	}

	return dict, zero, nil
}

// readType1 reads a Type 1 halftone from a PDF dictionary.
func readType1(x *pdf.Extractor, dict pdf.Dict) (*Type1, error) {
	h := &Type1{}

	if name, ok := dict["HalftoneName"]; ok {
		halftoneName, err := pdf.GetString(x.R, name)
		if err != nil {
			return nil, err
		}
		h.HalftoneName = string(halftoneName)
	}

	if freq, ok := dict["Frequency"]; ok {
		frequency, err := pdf.GetNumber(x.R, freq)
		if err != nil {
			return nil, err
		}
		h.Frequency = float64(frequency)
	}

	if angle, ok := dict["Angle"]; ok {
		angleVal, err := pdf.GetNumber(x.R, angle)
		if err != nil {
			return nil, err
		}
		h.Angle = float64(angleVal)
	}

	if spotObj, err := pdf.Resolve(x.R, dict["SpotFunction"]); err != nil {
		return nil, err
	} else {
		switch spot := spotObj.(type) {
		case pdf.Name:
			if fn, ok := nameToSpot[spot]; ok {
				h.SpotFunction = fn
			}
		case pdf.Array:
			for _, elem := range spot {
				if x, err := pdf.Optional(pdf.GetName(x.R, elem)); err == nil {
					if fn, ok := nameToSpot[x]; ok {
						h.SpotFunction = fn
						break
					}
				}
			}
		case pdf.Dict:
			spotFunc, err := function.Extract(x, spot)
			if err != nil {
				return nil, err
			}
			h.SpotFunction = spotFunc
		case *pdf.Stream:
			spotFunc, err := function.Extract(x, spot)
			if err != nil {
				return nil, err
			}
			h.SpotFunction = spotFunc
		}
	}
	if h.SpotFunction == nil {
		h.SpotFunction = SimpleDot
	} else if nIn, nOut := h.SpotFunction.Shape(); nIn != 2 || nOut != 1 {
		return nil, fmt.Errorf("wrong spot function shape %dx%d != 2x1", nIn, nOut)
	}

	if accurateScreens, ok := dict["AccurateScreens"]; ok {
		accurate, err := pdf.GetBoolean(x.R, accurateScreens)
		if err != nil {
			return nil, err
		}
		h.AccurateScreens = bool(accurate)
	}

	if transferFunc, ok := dict["TransferFunction"]; ok {
		h.TransferFunction = transferFunc
	}

	// Validate required fields if HalftoneName is not provided
	if h.HalftoneName == "" {
		if h.Frequency <= 0 {
			return nil, fmt.Errorf("invalid halftone frequency %g", h.Frequency)
		}
		if h.SpotFunction == nil {
			return nil, errors.New("missing spot function")
		}
	}

	return h, nil
}
