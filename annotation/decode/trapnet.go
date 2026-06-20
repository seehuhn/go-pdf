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

package decode

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
)

func decodeTrapNet(c pdf.Cursor, dict pdf.Dict) (*annotation.TrapNet, error) {
	trapNet := &annotation.TrapNet{}

	// Extract common annotation fields
	if err := decodeCommon(c, &trapNet.Common, dict); err != nil {
		return nil, err
	}

	// Extract trap network-specific fields
	if lastModified, err := pdf.Optional(c.TextString(dict["LastModified"])); err != nil {
		return nil, err
	} else if lastModified != "" {
		trapNet.LastModified = string(lastModified)
	}

	if version, err := pdf.Optional(c.Array(dict["Version"])); err != nil {
		return nil, err
	} else if len(version) > 0 {
		refs := make([]pdf.Reference, 0, len(version))
		for _, obj := range version {
			if ref, ok := obj.(pdf.Reference); ok {
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			trapNet.Version = refs
		}
	}

	if annotStates, err := pdf.Optional(c.Array(dict["AnnotStates"])); err != nil {
		return nil, err
	} else if len(annotStates) > 0 {
		states := make([]pdf.Name, len(annotStates))
		for i, obj := range annotStates {
			if name, ok := obj.(pdf.Name); ok {
				states[i] = name
			} else if obj == nil {
				states[i] = "" // null entry
			}
		}
		trapNet.AnnotStates = states
	}

	if fontFauxing, err := pdf.Optional(c.Array(dict["FontFauxing"])); err != nil {
		return nil, err
	} else if len(fontFauxing) > 0 {
		refs := make([]pdf.Reference, 0, len(fontFauxing))
		for _, obj := range fontFauxing {
			if ref, ok := obj.(pdf.Reference); ok {
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			trapNet.FontFauxing = refs
		}
	}

	// repair field combinations
	hasLM := trapNet.LastModified != ""
	hasVer := len(trapNet.Version) > 0
	hasAS := len(trapNet.AnnotStates) > 0
	switch {
	case hasLM && !hasVer && !hasAS:
		// valid: LastModified only
	case !hasLM && hasVer && hasAS:
		// valid: Version + AnnotStates
	case hasLM && hasVer && hasAS:
		// all present: prefer Version+AnnotStates
		trapNet.LastModified = ""
	case hasLM:
		// LastModified + incomplete pair: keep LastModified, drop the rest
		trapNet.Version = nil
		trapNet.AnnotStates = nil
	default:
		// no valid combination: set LastModified, drop incomplete pair
		trapNet.LastModified = "D:19700101000000Z"
		trapNet.Version = nil
		trapNet.AnnotStates = nil
	}

	return trapNet, nil
}
