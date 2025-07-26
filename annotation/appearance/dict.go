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

import "seehuhn.de/go/pdf"

// Dict represents an annotation appearance dictionary.
type Dict struct {
	// Normal is the annotation's normal appearance.
	Normal pdf.Object

	// RollOver is the annotation's rollover appearance.
	//
	// When writing appearance dictionaries, a zero value can be used as a
	// shorthand for the same value as Normal.
	RollOver pdf.Object

	// Down is the annotation's down appearance.
	//
	// When writing appearance dictionaries, a zero value can be used as a
	// shorthand for the same value as Normal.
	Down pdf.Object

	// SingleUse determines if Embed returns as dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*Dict)(nil)

func Extract(r pdf.Getter, obj pdf.Object) (*Dict, error) {
	_, singleUse := obj.(pdf.Reference)

	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	}

	N, err := pdf.Resolve(r, dict["N"])
	if err != nil {
		return nil, err
	}

	R, _ := pdf.Resolve(r, dict["R"])
	if R == nil {
		R = N
	}

	D, _ := pdf.Resolve(r, dict["D"])
	if D == nil {
		D = N
	}

	return &Dict{
		Normal:    N,
		RollOver:  R,
		Down:      D,
		SingleUse: singleUse,
	}, nil
}

func (d *Dict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
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

func (d *Dict) HasDicts() bool {
	if d == nil {
		return false
	}

	if _, ok := d.Normal.(pdf.Dict); ok {
		return true
	}
	if _, ok := d.RollOver.(pdf.Dict); ok {
		return true
	}
	if _, ok := d.Down.(pdf.Dict); ok {
		return true
	}
	return false
}
