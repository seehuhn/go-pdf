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

import "seehuhn.de/go/pdf"

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
	LastModified string

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

func extractTrapNet(r pdf.Getter, dict pdf.Dict, singleUse bool) (*TrapNet, error) {
	trapNet := &TrapNet{}

	// Extract common annotation fields
	if err := extractCommon(r, &trapNet.Common, dict, singleUse); err != nil {
		return nil, err
	}

	// Extract trap network-specific fields
	// LastModified (conditional)
	if lastModified, err := pdf.GetTextString(r, dict["LastModified"]); err == nil && lastModified != "" {
		trapNet.LastModified = string(lastModified)
	}

	// Version (conditional)
	if version, err := pdf.GetArray(r, dict["Version"]); err == nil && len(version) > 0 {
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

	// AnnotStates (conditional)
	if annotStates, err := pdf.GetArray(r, dict["AnnotStates"]); err == nil && len(annotStates) > 0 {
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

	// FontFauxing (optional)
	if fontFauxing, err := pdf.GetArray(r, dict["FontFauxing"]); err == nil && len(fontFauxing) > 0 {
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

	return trapNet, nil
}

func (t *TrapNet) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused
	dict, err := t.asDict(rm)
	if err != nil {
		return nil, zero, err
	}

	if t.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err = rm.Out.Put(ref, dict)
	return ref, zero, err
}

func (t *TrapNet) asDict(rm *pdf.ResourceManager) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "trap network annotation", pdf.V1_3); err != nil {
		return nil, err
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("TrapNet"),
	}

	// Add common annotation fields
	if err := t.Common.fillDict(rm, dict, isMarkup(t)); err != nil {
		return nil, err
	}

	// Add trap network-specific fields
	// LastModified (conditional)
	if t.LastModified != "" {
		if err := pdf.CheckVersion(rm.Out, "trap network annotation LastModified entry", pdf.V1_4); err != nil {
			return nil, err
		}
		dict["LastModified"] = pdf.TextString(t.LastModified)
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
