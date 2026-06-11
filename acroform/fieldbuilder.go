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

// The builder methods assemble a field hierarchy top-down, wiring each child's
// parent link as it is added. A tree built this way resolves inherited
// attributes (see [ResolvedFT]) and fully qualified names (see
// [FieldCommon.FullyQualifiedName]) correctly straight away, without first being
// encoded or decoded.

// must returns the concrete Field that embeds c. It is set when the field is
// created by a builder or by decoding; calling a builder method on a field
// assembled from a raw struct literal is a programming error and panics.
func (c *FieldCommon) must() Field {
	if c.self == nil {
		panic("acroform: field builder used on a field not created by a builder")
	}
	return c.self
}

// initField stamps the bookkeeping shared by every builder: the self
// back-pointer (so a promoted builder method reaches the concrete field), the
// partial name, and the parent link used for inheritance and name resolution.
func initField(f Field, name string, parent Field) {
	c := f.GetFieldCommon()
	c.self = f
	c.T = name
	c.parent = parent
}

// NewField adds a new non-terminal field as a root of the form and returns it.
func (form *InteractiveForm) NewField(name string) *FieldCommon {
	f := &FieldCommon{}
	initField(f, name, nil)
	form.Fields = append(form.Fields, f)
	return f
}

// NewTextField adds a new text field (field type "Tx") as a root of the form
// and returns it.
func (form *InteractiveForm) NewTextField(name string) *FieldTx {
	f := &FieldTx{}
	initField(f, name, nil)
	form.Fields = append(form.Fields, f)
	return f
}

// NewButtonField adds a new button field (field type "Btn") as a root of the
// form and returns it. Set its flags to select a check box, radio button, or
// push button.
func (form *InteractiveForm) NewButtonField(name string) *FieldBtn {
	f := &FieldBtn{}
	initField(f, name, nil)
	form.Fields = append(form.Fields, f)
	return f
}

// NewChoiceField adds a new choice field (field type "Ch") as a root of the
// form and returns it.
func (form *InteractiveForm) NewChoiceField(name string) *FieldChoice {
	f := &FieldChoice{}
	initField(f, name, nil)
	form.Fields = append(form.Fields, f)
	return f
}

// NewSignatureField adds a new signature field (field type "Sig") as a root of
// the form and returns it.
func (form *InteractiveForm) NewSignatureField(name string) *FieldSig {
	f := &FieldSig{}
	initField(f, name, nil)
	form.Fields = append(form.Fields, f)
	return f
}

// NewField adds a new non-terminal sub-field under this field and returns it.
func (c *FieldCommon) NewField(name string) *FieldCommon {
	parent := c.must()
	f := &FieldCommon{}
	initField(f, name, parent)
	c.Kids = append(c.Kids, f)
	return f
}

// NewTextField adds a new text field (field type "Tx") as a sub-field of this
// field and returns it.
func (c *FieldCommon) NewTextField(name string) *FieldTx {
	parent := c.must()
	f := &FieldTx{}
	initField(f, name, parent)
	c.Kids = append(c.Kids, f)
	return f
}

// NewButtonField adds a new button field (field type "Btn") as a sub-field of
// this field and returns it.
func (c *FieldCommon) NewButtonField(name string) *FieldBtn {
	parent := c.must()
	f := &FieldBtn{}
	initField(f, name, parent)
	c.Kids = append(c.Kids, f)
	return f
}

// NewChoiceField adds a new choice field (field type "Ch") as a sub-field of
// this field and returns it.
func (c *FieldCommon) NewChoiceField(name string) *FieldChoice {
	parent := c.must()
	f := &FieldChoice{}
	initField(f, name, parent)
	c.Kids = append(c.Kids, f)
	return f
}

// NewSignatureField adds a new signature field (field type "Sig") as a sub-field
// of this field and returns it.
func (c *FieldCommon) NewSignatureField(name string) *FieldSig {
	parent := c.must()
	f := &FieldSig{}
	initField(f, name, parent)
	c.Kids = append(c.Kids, f)
	return f
}
