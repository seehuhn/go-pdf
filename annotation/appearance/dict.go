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

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/form"
)

// Dict represents an annotation appearance dictionary.
type Dict struct {
	// Normal is the annotation's normal appearance.
	// This is mutually exclusive with NormalMap.
	Normal *form.Form

	// NormalMap give the annotation's normal appearance for each state.
	// This is mutually exclusive with Normal.
	NormalMap map[pdf.Name]*form.Form

	// RollOver is the annotation's rollover appearance.
	// This is mutually exclusive with RollOverMap.
	RollOver *form.Form

	// RollOverMap gives the annotation's rollover appearance for each state.
	// This is mutually exclusive with RollOver.
	RollOverMap map[pdf.Name]*form.Form

	// Down is the annotation's down appearance.
	// This is mutually exclusive with DownMap.
	Down *form.Form

	// DownMap gives the annotation's down appearance for each state.
	// This is mutually exclusive with Down.
	DownMap map[pdf.Name]*form.Form

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder[pdf.Unused] = (*Dict)(nil)

func Extract(x *pdf.Extractor, obj pdf.Object) (*Dict, error) {
	_, isIndirect := obj.(pdf.Reference)

	res := &Dict{
		SingleUse: !isIndirect,
	}

	dict, err := pdf.GetDict(x.R, obj)
	if err != nil {
		return nil, err
	}

	N, err := pdf.Resolve(x.R, dict["N"])
	if err != nil {
		return nil, err
	}
	switch N := N.(type) {
	case pdf.Dict:
		res.NormalMap = make(map[pdf.Name]*form.Form)
		for key, obj := range N {
			state := pdf.Name(key)
			formObj, err := form.Extract(x, obj)
			if err != nil {
				return nil, err
			}
			res.NormalMap[state] = formObj
		}
	case *pdf.Stream:
		formObj, err := form.Extract(x, N)
		if err != nil {
			return nil, err
		}
		res.Normal = formObj
	default:
		return nil, pdf.Errorf("invalid appearance dict entry: N %T", N)
	}

	R, _ := pdf.Resolve(x.R, dict["R"])
	if R == nil {
		R = N
	}
	switch R := R.(type) {
	case pdf.Dict:
		res.RollOverMap = make(map[pdf.Name]*form.Form)
		for key, obj := range R {
			state := pdf.Name(key)
			formObj, err := form.Extract(x, obj)
			if err != nil {
				return nil, err
			}
			res.RollOverMap[state] = formObj
		}
	case *pdf.Stream:
		formObj, err := form.Extract(x, R)
		if err != nil {
			return nil, err
		}
		res.RollOver = formObj
	}

	D, _ := pdf.Resolve(x.R, dict["D"])
	if D == nil {
		D = N
	}
	switch D := D.(type) {
	case pdf.Dict:
		res.DownMap = make(map[pdf.Name]*form.Form)
		for key, obj := range D {
			state := pdf.Name(key)
			formObj, err := form.Extract(x, obj)
			if err != nil {
				return nil, err
			}
			res.DownMap[state] = formObj
		}
	case *pdf.Stream:
		formObj, err := form.Extract(x, D)
		if err != nil {
			return nil, err
		}
		res.Down = formObj
	}

	return res, nil
}

func (d *Dict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "appearance streams", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}

	// Embed Normal appearance
	if d.Normal != nil {
		nRef, _, err := pdf.ResourceManagerEmbed(rm, d.Normal)
		if err != nil {
			return nil, zero, err
		}
		dict["N"] = nRef
	} else if d.NormalMap != nil {
		nDict := pdf.Dict{}
		for state, form := range d.NormalMap {
			formRef, _, err := pdf.ResourceManagerEmbed(rm, form)
			if err != nil {
				return nil, zero, err
			}
			nDict[state] = formRef
		}
		dict["N"] = nDict
	}

	// Embed RollOver appearance
	if d.RollOver != nil {
		rRef, _, err := pdf.ResourceManagerEmbed(rm, d.RollOver)
		if err != nil {
			return nil, zero, err
		}
		dict["R"] = rRef
	} else if d.RollOverMap != nil {
		rDict := pdf.Dict{}
		for state, form := range d.RollOverMap {
			formRef, _, err := pdf.ResourceManagerEmbed(rm, form)
			if err != nil {
				return nil, zero, err
			}
			rDict[state] = formRef
		}
		dict["R"] = rDict
	}

	// Embed Down appearance
	if d.Down != nil {
		dRef, _, err := pdf.ResourceManagerEmbed(rm, d.Down)
		if err != nil {
			return nil, zero, err
		}
		dict["D"] = dRef
	} else if d.DownMap != nil {
		dDict := pdf.Dict{}
		for state, form := range d.DownMap {
			formRef, _, err := pdf.ResourceManagerEmbed(rm, form)
			if err != nil {
				return nil, zero, err
			}
			dDict[state] = formRef
		}
		dict["D"] = dDict
	}

	if d.SingleUse {
		return dict, zero, nil
	}

	ref := rm.Out.Alloc()
	err := rm.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

func (d *Dict) HasDicts() bool {
	if d == nil {
		return false
	}

	return d.NormalMap != nil ||
		d.RollOverMap != nil ||
		d.DownMap != nil
}
