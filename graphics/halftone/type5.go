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
	"seehuhn.de/go/pdf/graphics"
)

// PDF 2.0 sections: 10.6.5.1 10.6.5.6

// Type5 represents a Type 5 halftone dictionary that defines separate screens
// for multiple colorants.
type Type5 struct {
	// HalftoneName (optional) is the name of the halftone dictionary.
	HalftoneName string

	// Default is the halftone for colorants without specific entries.
	// Must not be Type 5. Required to have transfer function if nonprimary colorants exist.
	Default interface {
		pdf.Embedder[pdf.Unused]
		graphics.Halftone
	}

	// Colorants maps colorant names to their specific halftone dictionaries.
	// Standard names include: "Cyan", "Magenta", "Yellow", "Black" (CMYK);
	// "Red", "Green", "Blue" (RGB); "Gray" (DeviceGray).
	// Spot colors use specific colorant names.
	Colorants map[string]interface {
		pdf.Embedder[pdf.Unused]
		graphics.Halftone
	}
}

var _ graphics.Halftone = (*Type5)(nil)

// HalftoneType returns 5.
// This implements the [graphics.Halftone] interface.
func (h *Type5) HalftoneType() int {
	return 5
}

func (h *Type5) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "halftone screening", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	if h.HalftoneName == "" {
		if h.Default == nil {
			return nil, zero, errors.New("missing default halftone")
		}
		if h.Default.HalftoneType() == 5 {
			return nil, zero, errors.New("default halftone cannot be Type 5")
		}
	} else {
		// If HalftoneName is provided, all other fields become optional.
		if h.Default != nil && h.Default.HalftoneType() == 5 {
			return nil, zero, errors.New("default halftone cannot be Type 5")
		}
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(5),
	}

	// Add optional fields
	opt := rm.Out.GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if h.HalftoneName != "" {
		dict["HalftoneName"] = pdf.String(h.HalftoneName)
	}

	// Embed the default halftone if present
	if h.Default != nil {
		defaultEmbedded, _, err := pdf.ResourceManagerEmbed(rm, h.Default)
		if err != nil {
			return nil, zero, fmt.Errorf("failed to embed default halftone: %w", err)
		}
		dict["Default"] = defaultEmbedded
	}

	// Embed colorant-specific halftones
	for colorantName, halftone := range h.Colorants {
		if halftone.HalftoneType() == 5 {
			return nil, zero, fmt.Errorf("colorant halftone for %q cannot be Type 5", colorantName)
		}

		colorantEmbedded, _, err := pdf.ResourceManagerEmbed(rm, halftone)
		if err != nil {
			return nil, zero, fmt.Errorf("failed to embed halftone for colorant %q: %w", colorantName, err)
		}
		dict[pdf.Name(colorantName)] = colorantEmbedded
	}

	return dict, zero, nil
}

// readType5 reads a Type 5 halftone from a PDF dictionary.
func readType5(x *pdf.Extractor, dict pdf.Dict) (*Type5, error) {
	h := &Type5{}

	if name, ok := dict["HalftoneName"]; ok {
		halftoneName, err := pdf.GetString(x.R, name)
		if err != nil {
			return nil, err
		}
		h.HalftoneName = string(halftoneName)
	}

	// Read the Default halftone
	if defaultObj, ok := dict["Default"]; ok {
		defaultHalftone, err := Read(x, defaultObj)
		if err != nil {
			return nil, fmt.Errorf("failed to read default halftone: %w", err)
		}
		if defaultHalftone.HalftoneType() == 5 {
			return nil, &pdf.MalformedFileError{
				Err: errors.New("default halftone cannot be Type 5"),
			}
		}
		// Type assertion to ensure it matches the expected interface
		if embedder, ok := defaultHalftone.(interface {
			pdf.Embedder[pdf.Unused]
			graphics.Halftone
		}); ok {
			h.Default = embedder
		} else {
			return nil, fmt.Errorf("default halftone does not implement required interfaces")
		}
	} else if h.HalftoneName == "" {
		return nil, errors.New("missing default halftone")
	}

	// Read colorant-specific halftones
	// Standard colorant names to check for
	standardColorants := []string{"Cyan", "Magenta", "Yellow", "Black", "Red", "Green", "Blue", "Gray"}

	// Check for standard colorants
	for _, colorant := range standardColorants {
		if colorantObj, ok := dict[pdf.Name(colorant)]; ok {
			colorantHalftone, err := Read(x, colorantObj)
			if err != nil {
				return nil, fmt.Errorf("failed to read halftone for colorant %q: %w", colorant, err)
			}
			if colorantHalftone.HalftoneType() == 5 {
				return nil, &pdf.MalformedFileError{
					Err: fmt.Errorf("colorant halftone for %q cannot be Type 5", colorant),
				}
			}
			// Type assertion to ensure it matches the expected interface
			if embedder, ok := colorantHalftone.(interface {
				pdf.Embedder[pdf.Unused]
				graphics.Halftone
			}); ok {
				// Initialize Colorants map only if we have colorants to add
				if h.Colorants == nil {
					h.Colorants = make(map[string]interface {
						pdf.Embedder[pdf.Unused]
						graphics.Halftone
					})
				}
				h.Colorants[colorant] = embedder
			} else {
				return nil, fmt.Errorf("colorant halftone for %q does not implement required interfaces", colorant)
			}
		}
	}

	// Check for any other colorant entries (spot colors, etc.)
	for key, value := range dict {
		keyStr := string(key)
		// Skip known system keys
		if keyStr == "Type" || keyStr == "HalftoneType" || keyStr == "HalftoneName" || keyStr == "Default" {
			continue
		}

		// Skip if already processed as standard colorant
		isStandardColorant := false
		for _, standard := range standardColorants {
			if keyStr == standard {
				isStandardColorant = true
				break
			}
		}
		if isStandardColorant {
			continue
		}

		// Try to read as a halftone
		colorantHalftone, err := Read(x, value)
		if err != nil {
			// If it fails to read as a halftone, skip it (might be some other entry)
			continue
		}
		if colorantHalftone.HalftoneType() == 5 {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("colorant halftone for %q cannot be Type 5", keyStr),
			}
		}
		// Type assertion to ensure it matches the expected interface
		if embedder, ok := colorantHalftone.(interface {
			pdf.Embedder[pdf.Unused]
			graphics.Halftone
		}); ok {
			// Initialize Colorants map only if we have colorants to add
			if h.Colorants == nil {
				h.Colorants = make(map[string]interface {
					pdf.Embedder[pdf.Unused]
					graphics.Halftone
				})
			}
			h.Colorants[keyStr] = embedder
		}
	}

	return h, nil
}
