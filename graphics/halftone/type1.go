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
	"seehuhn.de/go/pdf/graphics/transfer"
)

// PDF 2.0 sections: 10.6.3 10.6.5.1 10.6.5.2

// Type1 represents a Type 1 halftone dictionary that defines screens using
// frequency, angle, and spot function parameters.
type Type1 struct {
	// Frequency is the screen frequency (halftone cells per inch).
	Frequency float64

	// Angle is the screen angle in (degrees counterclockwise) relative to the
	// device coordinate system.
	Angle float64

	// SpotFunction defines the pixel adjustment order for gray levels. This
	// can either be a predefined spot function (e.g., [SimpleDot], [Round],
	// [Ellipse]) or a custom function object.  The function must map the cell
	// [-1, 1]x[-1, 1] to [-1, 1].
	//
	// Note that this library does not support writing halftone dictionaries
	// with non-standard named spot functions.
	SpotFunction pdf.Function

	// AccurateScreens enables a more precise but computationally expensive
	// halftone algorithm.
	AccurateScreens bool

	// TransferFunction (optional) overrides the current transfer function for
	// this component. Use [transfer.Identity] for the identity function.
	TransferFunction pdf.Function
}

var _ Halftone = (*Type1)(nil)

// extractType1 reads a Type 1 halftone from a PDF dictionary.
func extractType1(x *pdf.Extractor, dict pdf.Dict) (*Type1, error) {
	h := &Type1{}

	if freq, err := pdf.GetNumber(x.R, dict["Frequency"]); err != nil {
		return nil, err
	} else if freq > 0 {
		h.Frequency = float64(freq)
	} else {
		return nil, pdf.Error("invalid halftone frequency")
	}

	// Angle is not technically required, but we can default to 0.
	if angle, err := pdf.Optional(pdf.GetNumber(x.R, dict["Angle"])); err != nil {
		return nil, err
	} else {
		h.Angle = float64(angle)
	}

	// SpotFunction is not technically optional, but we can default to SimpleDot.
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
		return nil, fmt.Errorf("invalid spot function shape %dx%d != 2x1", nIn, nOut)
	}

	if accurateScreens, ok := dict["AccurateScreens"]; ok {
		accurate, err := pdf.GetBoolean(x.R, accurateScreens)
		if err != nil {
			return nil, err
		}
		h.AccurateScreens = bool(accurate)
	}

	if tf, err := pdf.Resolve(x.R, dict["TransferFunction"]); err != nil {
		return nil, err
	} else if tf == pdf.Name("Identity") {
		h.TransferFunction = transfer.Identity
	} else {
		if F, err := pdf.Optional(function.Extract(x, tf)); err != nil {
			return nil, err
		} else if isValidTransferFunction(F) {
			h.TransferFunction = F
		}
	}

	return h, nil
}

func (h *Type1) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "halftone screening", pdf.V1_2); err != nil {
		return nil, err
	}

	if h.Frequency <= 0 {
		return nil, fmt.Errorf("invalid halftone frequency %g", h.Frequency)
	}
	if h.SpotFunction == nil {
		return nil, errors.New("missing spot function")
	}

	var spotObj pdf.Object
	if spot := h.SpotFunction; spot != nil {
		nIn, nOut := spot.Shape()
		if nIn != 2 || nOut != 1 {
			return nil, fmt.Errorf("wrong spot function shape %dx%d != 2x1", nIn, nOut)
		}

		if obj, ok := spotToName[spot]; ok {
			spotObj = obj
		} else {
			obj, err := rm.Embed(spot)
			if err != nil {
				return nil, err
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
	opt := rm.Out().GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if h.AccurateScreens {
		dict["AccurateScreens"] = pdf.Boolean(true)
	}

	if h.TransferFunction == transfer.Identity {
		dict["TransferFunction"] = pdf.Name("Identity")
	} else if h.TransferFunction != nil {
		if !isValidTransferFunction(h.TransferFunction) {
			return nil, errors.New("invalid transfer function shape")
		}
		ref, err := rm.Embed(h.TransferFunction)
		if err != nil {
			return nil, err
		}
		dict["TransferFunction"] = ref
	}

	// We always embed halftone dictionaries as indirect objects.
	ref := rm.Alloc()
	if err := rm.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// HalftoneType returns 1.
// This implements the [graphics.Halftone] interface.
func (h *Type1) HalftoneType() int {
	return 1
}

// GetTransferFunction returns the transfer function given in the halftone.
// This implements the [graphics.Halftone] interface.
func (h *Type1) GetTransferFunction() pdf.Function {
	return h.TransferFunction
}

func isValidTransferFunction(F pdf.Function) bool {
	if F == nil {
		return false
	}
	nIn, nOut := F.Shape()
	return nIn == 1 && nOut == 1
}
