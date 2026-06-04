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

package triggers

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
)

// PDF 2.0 sections: 12.6.3

// Form represents a form field's additional-actions dictionary.
// This corresponds to the AA entry in a form field dictionary.
//
// PDF 1.3, Table 199.
type Form struct {
	// Keystroke is an ECMAScript action performed when the user modifies a
	// character in a text field or combo box or modifies the selection in a
	// scrollable list box. This action may check the added text for validity
	// and reject or modify it.
	Keystroke *action.JavaScript

	// Format is an ECMAScript action performed before the field is formatted
	// to display its value. This action may modify the field's value before
	// formatting.
	Format *action.JavaScript

	// Validate is an ECMAScript action performed when the field's value is
	// changed. This action may check the new value for validity.
	Validate *action.JavaScript

	// Calculate is an ECMAScript action performed to recalculate the value of
	// this field when that of another field changes.
	Calculate *action.JavaScript
}

var _ pdf.Encoder = (*Form)(nil)

// IsEmpty reports whether the additional-actions dictionary defines no actions.
func (f *Form) IsEmpty() bool {
	return f.Keystroke == nil && f.Format == nil && f.Validate == nil && f.Calculate == nil
}

// Encode converts the Form to a PDF dictionary.
func (f *Form) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	dict := pdf.Dict{}

	if f.Keystroke != nil {
		if err := pdf.CheckVersion(rm.Out, "form AA K entry", pdf.V1_3); err != nil {
			return nil, err
		}
		obj, err := f.Keystroke.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["K"] = obj
	}

	if f.Format != nil {
		if err := pdf.CheckVersion(rm.Out, "form AA F entry", pdf.V1_3); err != nil {
			return nil, err
		}
		obj, err := f.Format.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["F"] = obj
	}

	if f.Validate != nil {
		if err := pdf.CheckVersion(rm.Out, "form AA V entry", pdf.V1_3); err != nil {
			return nil, err
		}
		obj, err := f.Validate.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["V"] = obj
	}

	if f.Calculate != nil {
		if err := pdf.CheckVersion(rm.Out, "form AA C entry", pdf.V1_3); err != nil {
			return nil, err
		}
		obj, err := f.Calculate.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["C"] = obj
	}

	return dict, nil
}

// DecodeForm reads a form field's additional-actions dictionary from
// a PDF object.
func DecodeForm(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*Form, error) {
	dict, err := x.GetDict(path, obj)
	if err != nil {
		return nil, err
	}
	if dict == nil {
		return nil, nil
	}

	f := &Form{}

	if act, err := pdf.ExtractorGetOptional(x, path, dict["K"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		f.Keystroke = js
	}

	if act, err := pdf.ExtractorGetOptional(x, path, dict["F"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		f.Format = js
	}

	if act, err := pdf.ExtractorGetOptional(x, path, dict["V"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		f.Validate = js
	}

	if act, err := pdf.ExtractorGetOptional(x, path, dict["C"], action.Decode); err != nil {
		return nil, err
	} else if js, ok := act.(*action.JavaScript); ok {
		f.Calculate = js
	}

	return f, nil
}
