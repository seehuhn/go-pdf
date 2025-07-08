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

// AppearanceDict represents an annotation appearance dictionary.
type AppearanceDict struct {
	// Normal is the annotation's normal appearance.
	Normal Appearance

	// RollOver (optional) is the annotation's rollover appearance.
	// Default: the value of Normal.
	RollOver Appearance

	// Down (optional) is the annotation's down appearance.
	// Default: the value of Normal.
	Down Appearance
}

var _ pdf.Embedder[pdf.Unused] = (*AppearanceDict)(nil)

func (d *AppearanceDict) hasDicts() bool {
	if d == nil {
		return false
	}

	if _, ok := d.Normal.(AppearanceStates); ok {
		return true
	}
	if _, ok := d.RollOver.(AppearanceStates); ok {
		return true
	}
	if _, ok := d.Down.(AppearanceStates); ok {
		return true
	}
	return false
}

func (d *AppearanceDict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "appearance streams", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}

	dict["N"] = d.Normal
	if d.RollOver != nil {
		dict["R"] = d.RollOver
	}
	if d.Down != nil {
		dict["D"] = d.Down
	}

	return dict, zero, nil
}

// Appearance represents either a single appearance stream
// or a subdictionary of appearance states.
//
// This must be one of [AppearanceStream] or [AppearanceStates].
type Appearance interface {
	isAppearanceValue()

	pdf.Object

	// Get returns the appearance stream reference for the given state.
	// For SingleStream, returns the stream reference regardless of state.
	// For StatesDictionary, looks up the state and returns the reference,
	// or zero value if not found.
	Get(state pdf.Name) pdf.Reference
}

// AppearanceStream represents a single appearance stream reference.
type AppearanceStream pdf.Reference

func (s AppearanceStream) isAppearanceValue() {}

func (s AppearanceStream) AsPDF(pdf.OutputOptions) pdf.Native {
	return pdf.Reference(s)
}

func (s AppearanceStream) Get(state pdf.Name) pdf.Reference {
	return pdf.Reference(s)
}

// AppearanceStates represents a mapping of state names to stream references.
type AppearanceStates map[pdf.Name]pdf.Reference

func (s AppearanceStates) isAppearanceValue() {}

func (s AppearanceStates) AsPDF(pdf.OutputOptions) pdf.Native {
	statesDict := pdf.Dict{}
	for state, ref := range s {
		statesDict[state] = ref
	}
	return statesDict
}

func (s AppearanceStates) Get(state pdf.Name) pdf.Reference {
	return s[state]
}
