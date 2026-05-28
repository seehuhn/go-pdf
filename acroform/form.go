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

// Package acroform implements PDF interactive forms (AcroForms).
//
// A PDF document has at most one interactive form, referenced from the
// AcroForm entry of the document catalog. The form is a collection of fields,
// gathering information interactively from the user, that may appear on any
// combination of pages.
package acroform

import (
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/extract"
)

// PDF 2.0 sections: 12.7.3

// InteractiveForm represents a document's interactive form, referenced from
// the AcroForm entry in the document catalog.
type InteractiveForm struct {
	// Fields are the document's root fields, those with no ancestors in the
	// field hierarchy. Each entry is a reference to a field dictionary.
	Fields []pdf.Reference

	// NeedAppearances indicates that the viewer must construct appearance
	// streams and appearance dictionaries for all widget annotations in the
	// document.
	//
	// This entry is deprecated in PDF 2.0, where appearance streams are
	// required.
	NeedAppearances bool

	// SigFlags is a set of flags describing document-level characteristics
	// related to signature fields.
	SigFlags SignatureFlags

	// CalculationOrder (optional) lists field dictionaries with calculation
	// actions, in the order their values are recalculated when the value of
	// any field changes. Each entry is a reference to a field dictionary.
	//
	// This corresponds to the /CO entry in the interactive form dictionary.
	CalculationOrder []pdf.Reference

	// DefaultResources (optional) contains resources, such as fonts, that are
	// used by form field appearance streams.
	//
	// This corresponds to the /DR entry in the interactive form dictionary.
	DefaultResources *content.Resources

	// DefaultAppearance (optional) is the document-wide default appearance
	// string for variable text fields. An empty value indicates that no
	// document-wide default is set.
	//
	// This corresponds to the /DA entry in the interactive form dictionary.
	DefaultAppearance string

	// Align specifies the document-wide default justification for variable
	// text fields. The zero value is [pdf.TextAlignLeft].
	//
	// This corresponds to the /Q entry in the interactive form dictionary.
	Align pdf.TextAlign

	// XFA (optional) holds an XFA resource, as a stream or an array. The
	// library treats this value as opaque.
	//
	// This entry is deprecated in PDF 2.0.
	XFA pdf.Object
}

// SignatureFlags is a set of document-level flags related to signature fields.
type SignatureFlags uint32

const (
	// SignaturesExist indicates that the document contains at least one
	// signature field.
	SignaturesExist SignatureFlags = 1 << 0

	// AppendOnly indicates that the document contains signatures that may be
	// invalidated if the file is saved in a way that alters its previous
	// contents, rather than by an incremental update.
	AppendOnly SignatureFlags = 1 << 1
)

var _ pdf.Encoder = (*InteractiveForm)(nil)

// DecodeInteractiveForm reads an interactive form dictionary from a PDF file.
// The obj argument should be the value of the AcroForm entry in the document
// catalog. It returns nil if obj is nil.
//
// Always invoke this via [pdf.ExtractorGet] so that the form dictionary's
// reference is resolved and cached.
func DecodeInteractiveForm(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*InteractiveForm, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, nil
	}

	form := &InteractiveForm{}

	// Fields (required)
	if fields, err := pdf.Optional(x.GetArray(path, dict["Fields"])); err != nil {
		return nil, err
	} else {
		for _, el := range fields {
			if ref, ok := el.(pdf.Reference); ok {
				form.Fields = append(form.Fields, ref)
			}
		}
	}

	// NeedAppearances (optional)
	if na, err := pdf.Optional(x.GetBoolean(path, dict["NeedAppearances"])); err != nil {
		return nil, err
	} else {
		form.NeedAppearances = bool(na)
	}

	// SigFlags (optional)
	if sf, err := pdf.Optional(x.GetInteger(path, dict["SigFlags"])); err != nil {
		return nil, err
	} else {
		form.SigFlags = SignatureFlags(sf)
	}

	// CO (optional)
	if co, err := pdf.Optional(x.GetArray(path, dict["CO"])); err != nil {
		return nil, err
	} else {
		for _, el := range co {
			if ref, ok := el.(pdf.Reference); ok {
				form.CalculationOrder = append(form.CalculationOrder, ref)
			}
		}
	}

	// DR (optional)
	if drObj := dict["DR"]; drObj != nil {
		if dr, err := pdf.Optional(pdf.ExtractorGet(x, path, drObj, extract.Resources)); err != nil {
			return nil, err
		} else {
			form.DefaultResources = dr
		}
	}

	// DA (optional)
	if da, err := pdf.Optional(x.GetString(path, dict["DA"])); err != nil {
		return nil, err
	} else {
		form.DefaultAppearance = string(da)
	}

	// Q (optional)
	if q, err := pdf.Optional(x.GetInteger(path, dict["Q"])); err != nil {
		return nil, err
	} else if q >= 0 && q <= 2 {
		form.Align = pdf.TextAlign(q)
	}

	// XFA (optional)
	form.XFA = dict["XFA"]

	return form, nil
}

// Encode writes the interactive form to a PDF file and returns a reference to
// the form dictionary, suitable for use as the AcroForm entry in the document
// catalog.
//
// This implements the [pdf.Encoder] interface.
func (f *InteractiveForm) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "interactive form", pdf.V1_2); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	// Fields (required)
	fields := make(pdf.Array, len(f.Fields))
	for i, ref := range f.Fields {
		fields[i] = ref
	}
	dict["Fields"] = fields

	// NeedAppearances (optional)
	if f.NeedAppearances {
		dict["NeedAppearances"] = pdf.Boolean(true)
	}

	// SigFlags (optional)
	if f.SigFlags != 0 {
		if err := pdf.CheckVersion(rm.Out, "interactive form SigFlags entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["SigFlags"] = pdf.Integer(f.SigFlags)
	}

	// CO (optional)
	if len(f.CalculationOrder) > 0 {
		if err := pdf.CheckVersion(rm.Out, "interactive form CO entry", pdf.V1_3); err != nil {
			return nil, err
		}
		co := make(pdf.Array, len(f.CalculationOrder))
		for i, ref := range f.CalculationOrder {
			co[i] = ref
		}
		dict["CO"] = co
	}

	// DR (optional)
	if f.DefaultResources != nil {
		dr, err := rm.Embed(f.DefaultResources)
		if err != nil {
			return nil, err
		}
		dict["DR"] = dr
	}

	// DA (optional)
	if f.DefaultAppearance != "" {
		dict["DA"] = pdf.String(f.DefaultAppearance)
	}

	// Q (optional)
	if f.Align != pdf.TextAlignLeft {
		if f.Align < pdf.TextAlignLeft || f.Align > pdf.TextAlignRight {
			return nil, fmt.Errorf("invalid text alignment %d", f.Align)
		}
		dict["Q"] = pdf.Integer(f.Align)
	}

	// XFA (optional)
	if f.XFA != nil {
		// the stream form dates from PDF 1.5, the array form from PDF 1.6
		// TODO(voss): an indirect reference resolving to an array is gated
		// at 1.5 instead of 1.6 here, since we only inspect the direct value.
		xfaVersion := pdf.V1_5
		if _, ok := f.XFA.(pdf.Array); ok {
			xfaVersion = pdf.V1_6
		}
		if err := pdf.CheckVersion(rm.Out, "interactive form XFA entry", xfaVersion); err != nil {
			return nil, err
		}
		dict["XFA"] = f.XFA
	}

	ref := rm.Out.Alloc()
	if err := rm.Out.Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}
