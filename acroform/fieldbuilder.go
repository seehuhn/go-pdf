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

package acroform

// The builder functions assemble a field hierarchy top-down, wiring each
// child's parent link as it is added. A tree built this way resolves inherited
// attributes (see [ResolvedFT]) and fully qualified names (see
// [FieldCommon.FullyQualifiedName]) correctly straight away, without first
// being encoded or decoded.

// initField stamps the new field's partial name and, for a non-nil parent,
// adds the field to the parent's Kids and sets the parent link used for
// inheritance and name resolution.
func initField(f Field, parent Field, name string) {
	c := f.GetFieldCommon()
	c.T = name
	if parent != nil {
		c.Parent = parent
		p := parent.GetFieldCommon()
		p.Kids = append(p.Kids, f)
	}
}

// NewField returns a new non-terminal field with the given partial name. A
// non-nil parent adds the field as a sub-field of parent; a nil parent leaves
// the field detached, for use as a root field (see [InteractiveForm.Fields]).
func NewField(parent Field, name string) *FieldCommon {
	f := &FieldCommon{}
	initField(f, parent, name)
	return f
}

// NewTextField returns a new text field (field type "Tx") with the given
// partial name. Parent handling is as for [NewField].
func NewTextField(parent Field, name string) *FieldTx {
	f := &FieldTx{}
	initField(f, parent, name)
	return f
}

// NewButtonField returns a new button field (field type "Btn") with the given
// partial name. Set its flags to select a check box, radio button, or push
// button. Parent handling is as for [NewField].
func NewButtonField(parent Field, name string) *FieldBtn {
	f := &FieldBtn{}
	initField(f, parent, name)
	return f
}

// NewChoiceField returns a new choice field (field type "Ch") with the given
// partial name. Parent handling is as for [NewField].
func NewChoiceField(parent Field, name string) *FieldChoice {
	f := &FieldChoice{}
	initField(f, parent, name)
	return f
}

// NewSignatureField returns a new signature field (field type "Sig") with the
// given partial name. Parent handling is as for [NewField].
func NewSignatureField(parent Field, name string) *FieldSig {
	f := &FieldSig{}
	initField(f, parent, name)
	return f
}

// NewField adds a new non-terminal field as a root of the form and returns it.
func (form *InteractiveForm) NewField(name string) *FieldCommon {
	f := NewField(nil, name)
	form.Fields = append(form.Fields, f)
	return f
}

// NewTextField adds a new text field (field type "Tx") as a root of the form
// and returns it.
func (form *InteractiveForm) NewTextField(name string) *FieldTx {
	f := NewTextField(nil, name)
	form.Fields = append(form.Fields, f)
	return f
}

// NewButtonField adds a new button field (field type "Btn") as a root of the
// form and returns it. Set its flags to select a check box, radio button, or
// push button.
func (form *InteractiveForm) NewButtonField(name string) *FieldBtn {
	f := NewButtonField(nil, name)
	form.Fields = append(form.Fields, f)
	return f
}

// NewChoiceField adds a new choice field (field type "Ch") as a root of the
// form and returns it.
func (form *InteractiveForm) NewChoiceField(name string) *FieldChoice {
	f := NewChoiceField(nil, name)
	form.Fields = append(form.Fields, f)
	return f
}

// NewSignatureField adds a new signature field (field type "Sig") as a root of
// the form and returns it.
func (form *InteractiveForm) NewSignatureField(name string) *FieldSig {
	f := NewSignatureField(nil, name)
	form.Fields = append(form.Fields, f)
	return f
}
