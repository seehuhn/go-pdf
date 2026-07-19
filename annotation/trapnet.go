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
	"fmt"
	"time"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.5.2 12.5.6.21

// TrapNet represents a trap network annotation that defines the trapping
// characteristics for a page of a PDF document. Trapping is the process of
// adding marks to a page along color boundaries to avoid unwanted visual
// artifacts resulting from misregistration of colorants when the page is printed.
//
// NOTE: TrapNet annotations are deprecated in PDF 2.0.
//
// A page has no more than one trap network annotation, which is
// always be the last element in the page object's Annots array. The AP
// (appearances), AS (appearance state), and F (flags) entries are present,
// with the Print and ReadOnly flags set and all others clear.
type TrapNet struct {
	Common

	// LastModified (required if Version and AnnotStates are absent; is
	// absent if Version and AnnotStates are present; PDF 1.4) is the date and
	// time when the trap network was most recently modified.
	//
	// This is the LastModified entry of the trap network annotation.  It
	// shadows the promoted [Common.LastModified] field, which holds the
	// modification date of the annotation itself (the M entry).
	LastModified time.Time

	// Version (required if AnnotStates is present; is absent if
	// LastModified is present) is an unordered array of all objects present
	// in the page description at the time the trap networks were generated
	// and that, if changed, could affect the appearance of the page.
	Version []pdf.Reference

	// AnnotStates (required if Version is present; is absent if
	// LastModified is present) is an array of name objects representing the
	// appearance states (value of the AS entry) for annotations associated
	// with the page. The appearance states are listed in the same order
	// as the annotations in the page's Annots array.
	AnnotStates []pdf.Name

	// FontFauxing (optional) is an array of font dictionaries representing
	// fonts that were fauxed (replaced by substitute fonts) during the
	// generation of trap networks for the page.
	FontFauxing []pdf.Reference
}

var _ Annotation = (*TrapNet)(nil)

// AnnotationType returns "TrapNet".
// This implements the [Annotation] interface.
func (t *TrapNet) AnnotationType() pdf.Name {
	return "TrapNet"
}

func (t *TrapNet) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "trap network annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	if t.Flags != FlagPrint|FlagReadOnly {
		return nil, errors.New("trap network needs Print and ReadOnly flags only")
	}

	// The AS entry is required only where the normal appearance holds several
	// trap networks to choose between, which is the general rule for appearance
	// states and is checked in fillDict.  Taken literally the specification
	// demands AS on every trap network annotation, but the same subclause
	// describes AS as designating one of several alternate networks, and the
	// parallel wording for printer's marks makes the requirement conditional.
	// A state name is meaningless where there is nothing to select, so
	// requiring one would mean inventing a name and wrapping the sole
	// appearance in a subdictionary that the file never had.

	// The normal appearance of a trap network annotation is the trap network
	// itself, so it must carry the trap network entries.  An annotation
	// without an appearance has nothing to check.
	if ap := t.Appearance; ap != nil {
		if ap.Normal != nil && ap.Normal.TrapNet == nil {
			return nil, errors.New("normal appearance is not a trap network")
		}
		for name, f := range ap.NormalMap {
			if f != nil && f.TrapNet == nil {
				return nil, fmt.Errorf("normal appearance %q is not a trap network", name)
			}
		}
	}

	// validate field combinations
	hasLM := !t.LastModified.IsZero()
	hasVer := len(t.Version) > 0
	hasAS := len(t.AnnotStates) > 0
	switch {
	case hasLM && !hasVer && !hasAS:
		if err := pdf.CheckVersion(rm.Out, "trap network annotation LastModified entry", pdf.V1_4); err != nil {
			return nil, err
		}
	case !hasLM && hasVer && hasAS:
		// ok: Version + AnnotStates
	default:
		return nil, errors.New("trap network annotation requires either LastModified or Version+AnnotStates")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("TrapNet"),
	}

	// Add common annotation fields
	if err := t.Common.fillDict(rm, dict, isMarkup(t), false); err != nil {
		return nil, err
	}

	// Add trap network-specific fields
	if !t.LastModified.IsZero() {
		dict["LastModified"] = pdf.Date(t.LastModified)
	}

	// Version (conditional)
	if len(t.Version) > 0 {
		versionArray := make(pdf.Array, len(t.Version))
		for i, ref := range t.Version {
			versionArray[i] = ref
		}
		dict["Version"] = versionArray
	}

	// AnnotStates (conditional)
	if len(t.AnnotStates) > 0 {
		statesArray := make(pdf.Array, len(t.AnnotStates))
		for i, state := range t.AnnotStates {
			if state != "" {
				statesArray[i] = state
			} else {
				statesArray[i] = nil // null entry
			}
		}
		dict["AnnotStates"] = statesArray
	}

	// FontFauxing (optional)
	if len(t.FontFauxing) > 0 {
		fauxingArray := make(pdf.Array, len(t.FontFauxing))
		for i, ref := range t.FontFauxing {
			fauxingArray[i] = ref
		}
		dict["FontFauxing"] = fauxingArray
	}

	return dict, nil
}
