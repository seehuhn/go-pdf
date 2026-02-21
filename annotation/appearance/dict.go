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

// Package appearance handles annotation appearance dictionaries.
package appearance

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/extract"
	"seehuhn.de/go/pdf/graphics/form"
)

// PDF 2.0 sections: 12.5.5

// Dict represents an annotation appearance dictionary.
type Dict struct {
	// Normal is the annotation's normal appearance.
	// This is mutually exclusive with NormalMap.
	Normal *form.Form

	// NormalMap gives the annotation's normal appearance for each state.
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

var _ pdf.Embedder = (*Dict)(nil)

// Extract reads an annotation appearance dictionary from the PDF object obj.
func Extract(x *pdf.Extractor, obj pdf.Object) (*Dict, error) {
	singleUse := !x.IsIndirect // capture before other x method calls

	res := &Dict{
		SingleUse: singleUse,
	}

	dict, err := x.GetDict(obj)
	if err != nil {
		return nil, err
	}

	N, err := x.Resolve(dict["N"])
	if err != nil {
		return nil, err
	}
	switch N := N.(type) {
	case pdf.Dict:
		res.NormalMap = make(map[pdf.Name]*form.Form)
		for key, obj := range N {
			state := key
			formObj, err := pdf.ExtractorGet(x, obj, extract.Form)
			if err != nil {
				return nil, err
			}
			res.NormalMap[state] = formObj
		}
	case *pdf.Stream:
		formObj, err := pdf.ExtractorGet(x, N, extract.Form)
		if err != nil {
			return nil, err
		}
		res.Normal = formObj
	default:
		return nil, pdf.Errorf("invalid appearance dict entry: N %T", N)
	}

	R, _ := x.Resolve(dict["R"])
	if R == nil {
		R = N
	}
	switch R := R.(type) {
	case pdf.Dict:
		res.RollOverMap = make(map[pdf.Name]*form.Form)
		for key, obj := range R {
			state := key
			formObj, err := pdf.ExtractorGet(x, obj, extract.Form)
			if err != nil {
				return nil, err
			}
			res.RollOverMap[state] = formObj
		}
	case *pdf.Stream:
		formObj, err := pdf.ExtractorGet(x, R, extract.Form)
		if err != nil {
			return nil, err
		}
		res.RollOver = formObj
	}

	D, _ := x.Resolve(dict["D"])
	if D == nil {
		D = N
	}
	switch D := D.(type) {
	case pdf.Dict:
		res.DownMap = make(map[pdf.Name]*form.Form)
		for key, obj := range D {
			state := key
			formObj, err := pdf.ExtractorGet(x, obj, extract.Form)
			if err != nil {
				return nil, err
			}
			res.DownMap[state] = formObj
		}
	case *pdf.Stream:
		formObj, err := pdf.ExtractorGet(x, D, extract.Form)
		if err != nil {
			return nil, err
		}
		res.Down = formObj
	}

	return res, nil
}

func (d *Dict) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(e.Out(), "appearance streams", pdf.V1_2); err != nil {
		return nil, err
	}

	if d.Normal != nil && d.NormalMap != nil {
		return nil, errors.New("Normal and NormalMap are mutually exclusive")
	}
	if d.Normal == nil && d.NormalMap == nil {
		return nil, errors.New("normal appearance is required")
	}
	if d.RollOver != nil && d.RollOverMap != nil {
		return nil, errors.New("RollOver and RollOverMap are mutually exclusive")
	}
	if d.Down != nil && d.DownMap != nil {
		return nil, errors.New("Down and DownMap are mutually exclusive")
	}

	dict := pdf.Dict{}

	// normal appearance
	if d.Normal != nil {
		nRef, err := e.Embed(d.Normal)
		if err != nil {
			return nil, err
		}
		dict["N"] = nRef
	} else {
		nDict := pdf.Dict{}
		for state, form := range d.NormalMap {
			formRef, err := e.Embed(form)
			if err != nil {
				return nil, err
			}
			nDict[state] = formRef
		}
		dict["N"] = nDict
	}

	// rollover appearance
	if d.RollOver != nil {
		rRef, err := e.Embed(d.RollOver)
		if err != nil {
			return nil, err
		}
		dict["R"] = rRef
	} else if d.RollOverMap != nil {
		rDict := pdf.Dict{}
		for state, form := range d.RollOverMap {
			formRef, err := e.Embed(form)
			if err != nil {
				return nil, err
			}
			rDict[state] = formRef
		}
		dict["R"] = rDict
	}

	// down appearance
	if d.Down != nil {
		dRef, err := e.Embed(d.Down)
		if err != nil {
			return nil, err
		}
		dict["D"] = dRef
	} else if d.DownMap != nil {
		dDict := pdf.Dict{}
		for state, form := range d.DownMap {
			formRef, err := e.Embed(form)
			if err != nil {
				return nil, err
			}
			dDict[state] = formRef
		}
		dict["D"] = dDict
	}

	if d.SingleUse {
		return dict, nil
	}

	ref := e.Alloc()
	err := e.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// HasDicts reports whether any appearance uses a state-dependent map.
func (d *Dict) HasDicts() bool {
	if d == nil {
		return false
	}

	return d.NormalMap != nil ||
		d.RollOverMap != nil ||
		d.DownMap != nil
}
