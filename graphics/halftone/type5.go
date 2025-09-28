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
)

// PDF 2.0 sections: 10.6.5.1 10.6.5.6

// Type5 represents a Type 5 halftone dictionary that defines separate screens
// for multiple colorants.
type Type5 struct {
	// Default is the halftone for colorants without specific entries.
	// Must not be Type 5. Required to have transfer function if nonprimary colorants exist.
	Default Halftone

	// Colorants maps colorant names to their specific halftone dictionaries.
	// Standard names include: "Cyan", "Magenta", "Yellow", "Black" (CMYK);
	// "Red", "Green", "Blue" (RGB); "Gray" (DeviceGray).
	// Spot colors use specific colorant names.
	Colorants map[pdf.Name]Halftone
}

var _ Halftone = (*Type5)(nil)

// extractType5 reads a Type 5 halftone from a PDF dictionary.
func extractType5(x *pdf.Extractor, dict pdf.Dict) (*Type5, error) {
	h := &Type5{}

	// TODO(voss): avoid infinite recursion!!!
	if ht, err := pdf.ExtractorGet(x, dict["Default"], Extract); err != nil {
		return nil, err
	} else if ht != nil {
		if ht.HalftoneType() == 5 {
			return nil, pdf.Error("invalid Default halftone")
		}
		h.Default = ht
	} else {
		return nil, pdf.Error("missing Default halftone")
	}

	h.Colorants = make(map[pdf.Name]Halftone)
	for colorant, val := range dict {
		switch colorant {
		case "Type", "HalftoneType", "HalftoneName", "Default":
			continue
		}

		// TODO(voss): avoid infinite recursion!!!
		if ht, err := pdf.ExtractorGet(x, val, Extract); err != nil {
			return nil, err
		} else if ht != nil {
			if ht.HalftoneType() == 5 {
				return nil, pdf.Error("invalid Default halftone")
			}
			h.Colorants[colorant] = ht
		}
	}

	return h, nil
}

func (h *Type5) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "halftone screening", pdf.V1_2); err != nil {
		return nil, err
	}

	if h.Default == nil {
		return nil, errors.New("missing default halftone")
	}
	if h.Default.HalftoneType() == 5 {
		return nil, errors.New("default halftone cannot be Type 5")
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(5),
	}

	// Add optional fields
	opt := rm.Out().GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if h.Default != nil {
		if h.Default.GetTransferFunction() == nil {
			return nil, errors.New("missing transfer function")
		}
		defaultEmbedded, err := rm.Embed(h.Default)
		if err != nil {
			return nil, err
		}
		dict["Default"] = defaultEmbedded
	}

	for colorant, ht := range h.Colorants {
		var isPrimary bool
		switch colorant {
		case "Type", "HalftoneType", "HalftoneName", "Default":
			return nil, fmt.Errorf("invalid colorant name %q", colorant)
		case "Cyan", "Magenta", "Yellow", "Black", "Red", "Green", "Blue", "Gray":
			isPrimary = true
		}

		if ht.HalftoneType() == 5 {
			return nil, fmt.Errorf("colorant halftone for %q cannot be Type 5", colorant)
		}

		if !isPrimary && ht.GetTransferFunction() == nil {
			return nil, errors.New("missing transfer function")
		}

		colorantEmbedded, err := rm.Embed(ht)
		if err != nil {
			return nil, err
		}
		dict[colorant] = colorantEmbedded
	}

	// We always embed halftone dictionaries as indirect objects.
	ref := rm.Alloc()
	if err := rm.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

// HalftoneType returns 5.
// This implements the [Halftone] interface.
func (h *Type5) HalftoneType() int {
	return 5
}

// GetTransferFunction returns nil.
// This implements the [Halftone] interface.
func (h *Type5) GetTransferFunction() pdf.Function {
	return nil
}
