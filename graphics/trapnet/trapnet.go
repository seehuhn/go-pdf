// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package trapnet implements the form XObject entries specific to trap
// network appearance streams.
//
// A trap network is a form XObject containing the graphics objects which
// paint the traps for a page.  It appears as an appearance stream in the
// N entry of a trap network annotation's appearance dictionary.  Unlike
// group XObjects and reference XObjects, a trap network is not identified
// by an entry in the form dictionary; it is a trap network because of where
// it is referenced from.
//
// The entries represented by [Attributes] are stored directly in the form
// dictionary, alongside the normal form XObject entries.
package trapnet

import (
	"errors"
	"fmt"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/opaque"
)

// PDF 2.0 sections: 14.11.6

// DefaultPCM is used to repair a trap network which does not specify a valid
// process colour model.  Trapping is a process-colour operation and CMYK is
// the usual process colour model.
const DefaultPCM pdf.Name = "DeviceCMYK"

// Attributes holds the form dictionary entries specific to a trap network
// appearance stream.
type Attributes struct {
	// PCM is the process colour model assumed when the trap network was
	// created.  It must be one of DeviceGray, DeviceRGB, DeviceCMYK,
	// DeviceCMY, DeviceRGBK or DeviceN.
	PCM pdf.Name

	// SeparationColorNames (optional) names the colourants assumed when the
	// trap network was created.  Colourants implied by PCM are available
	// automatically and need not be listed.
	SeparationColorNames []pdf.Name

	// TrapRegions (optional; deprecated in PDF 2.0) refers to the TrapRegion
	// objects which define the page's trapping zones and the associated
	// trapping parameters, as described in Adobe Technical Note #5620,
	// Portable Job Ticket Format.
	//
	// The library does not interpret these objects.  They are preserved
	// verbatim, and references inside them are translated when the value is
	// written to a different PDF file.
	TrapRegions []*opaque.Object

	// TrapStyles (optional) describes the trap network to the user.
	TrapStyles string
}

// isValidPCM reports whether name is one of the process colour models
// allowed for the PCM entry.
func isValidPCM(name pdf.Name) bool {
	switch name {
	case "DeviceGray", "DeviceRGB", "DeviceCMYK", "DeviceCMY", "DeviceRGBK", "DeviceN":
		return true
	}
	return false
}

// Equal reports whether two Attributes are equal.
func (a *Attributes) Equal(other *Attributes) bool {
	if a == nil || other == nil {
		return a == other
	}
	if a.PCM != other.PCM || a.TrapStyles != other.TrapStyles {
		return false
	}
	if !slices.Equal(a.SeparationColorNames, other.SeparationColorNames) {
		return false
	}
	if len(a.TrapRegions) != len(other.TrapRegions) {
		return false
	}
	for i, r := range a.TrapRegions {
		if !r.Equal(other.TrapRegions[i]) {
			return false
		}
	}
	return true
}

// FillDict adds the trap network entries to a form XObject dictionary.
//
// The entries are stored directly in the form dictionary, so this cannot
// use the [pdf.Embedder] interface.
//
// PCM must name one of the allowed process colour models, and TrapRegions
// must not contain nil entries.
func (a *Attributes) FillDict(e *pdf.EmbedHelper, dict pdf.Dict) error {
	if err := pdf.CheckVersion(e.Out(), "trap network appearance stream", pdf.V1_3); err != nil {
		return err
	}

	if !isValidPCM(a.PCM) {
		return fmt.Errorf("invalid process colour model %q", a.PCM)
	}
	dict["PCM"] = a.PCM

	if len(a.SeparationColorNames) > 0 {
		names := make(pdf.Array, len(a.SeparationColorNames))
		for i, name := range a.SeparationColorNames {
			names[i] = name
		}
		dict["SeparationColorNames"] = names
	}

	if len(a.TrapRegions) > 0 {
		regions := make(pdf.Array, len(a.TrapRegions))
		for i, region := range a.TrapRegions {
			if region == nil {
				return errors.New("trap network has a nil trap region")
			}
			obj, err := e.Embed(region)
			if err != nil {
				return err
			}
			regions[i] = obj
		}
		dict["TrapRegions"] = regions
	}

	if a.TrapStyles != "" {
		dict["TrapStyles"] = pdf.TextString(a.TrapStyles)
	}

	return nil
}

// ExtractAttributes reads the trap network entries from a form XObject
// dictionary.  It returns nil if the dictionary contains none of them.
//
// A dictionary which uses any of the trap network entries but does not give
// a valid process colour model is repaired by assuming DeviceCMYK.
func ExtractAttributes(c pdf.Cursor, dict pdf.Dict) (*Attributes, error) {
	_, hasPCM := dict["PCM"]
	_, hasNames := dict["SeparationColorNames"]
	_, hasRegions := dict["TrapRegions"]
	_, hasStyles := dict["TrapStyles"]
	if !hasPCM && !hasNames && !hasRegions && !hasStyles {
		return nil, nil
	}

	a := &Attributes{}

	if pcm, err := pdf.Optional(c.Name(dict["PCM"])); err != nil {
		return nil, err
	} else if isValidPCM(pcm) {
		a.PCM = pcm
	} else {
		a.PCM = DefaultPCM
	}

	if names, err := pdf.Optional(c.Array(dict["SeparationColorNames"])); err != nil {
		return nil, err
	} else if len(names) > 0 {
		for _, obj := range names {
			name, err := pdf.Optional(c.Name(obj))
			if err != nil {
				return nil, err
			}
			if name != "" {
				a.SeparationColorNames = append(a.SeparationColorNames, name)
			}
		}
	}

	if regions, err := pdf.Optional(c.Array(dict["TrapRegions"])); err != nil {
		return nil, err
	} else if len(regions) > 0 {
		x := c.Extractor()
		for _, obj := range regions {
			if obj == nil {
				continue
			}
			// The raw object is wrapped, so that indirect references stay
			// indirect when the value is written back.
			a.TrapRegions = append(a.TrapRegions, opaque.Extract(x, obj))
		}
	}

	if styles, err := pdf.Optional(c.TextString(dict["TrapStyles"])); err != nil {
		return nil, err
	} else {
		a.TrapStyles = string(styles)
	}

	return a, nil
}
