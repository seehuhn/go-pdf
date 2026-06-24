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

import (
	"errors"
	"strings"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 12.7.4.1 12.7.4.2

// Node represents any node, internal or leaf, in the field tree of an
// interactive form. This is either a [*Group] or one of the four [Field]
// types.
type Node interface {
	// PartialName returns the node's partial field name (the /T entry). An
	// empty value means the node contributes no component to the fully
	// qualified names of its descendants.
	PartialName() string
}

// Field is a field in a PDF interactive form. This is implemented by four
// concrete types:
//   - [TextField] (text),
//   - [ButtonField] (button),
//   - [ChoiceField] (choice) and
//   - [SignatureField] (signature).
//
// A field is rendered on a page by one or more
// [seehuhn.de/go/pdf/annotation.Widget] annotations. Use
// [seehuhn.de/go/pdf/annotation.AddWidget] to add a widget to a field.
//
// The types implementing Field are the leaf nodes in the field tree of an
// interactive form.
//
// Use [InteractiveForm.Encode] to encode all fields of the form as a PDF field
// tree.
type Field interface {
	Node

	// GetCommon returns the entries common to all field types.
	GetCommon() *Common

	// FieldType returns the PDF field type, one of "Btn", "Tx", "Ch", or "Sig".
	FieldType() pdf.Name

	fillDict(rm *pdf.ResourceManager, dict pdf.Dict) error
}

// Widget represents the visual representation of a [Field] on a page. The only
// implementation is "seehuhn.de/go/pdf/annotation".Widget; the interface
// exists only to avoid dependency cycles.
type Widget interface {
	pdf.Encoder

	// ParentField returns the field this widget belongs to, or nil.
	ParentField() Field
}

// terminalEntries builds the dictionary entries of a terminal field — its
// flattened own entries (FT, T, TU, TM, Ff, AA, and the type-specific entries),
// excluding /Parent and /Kids. The factoring pass may later remove inheritable
// entries that are hoisted into an ancestor.
func terminalEntries(rm *pdf.ResourceManager, f Field) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "interactive form field", pdf.V1_2); err != nil {
		return nil, err
	}

	b := f.GetCommon()
	dict := pdf.Dict{
		"FT": f.FieldType(),
	}

	if b.Name != "" {
		if strings.Contains(b.Name, ".") {
			return nil, errors.New("field partial name must not contain a period")
		}
		dict["T"] = pdf.TextString(b.Name)
	}
	if b.TU != "" {
		if err := pdf.CheckVersion(rm.Out, "field TU entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["TU"] = pdf.TextString(b.TU)
	}
	if b.TM != "" {
		if err := pdf.CheckVersion(rm.Out, "field TM entry", pdf.V1_3); err != nil {
			return nil, err
		}
		dict["TM"] = pdf.TextString(b.TM)
	}
	if b.Flags != 0 {
		if err := checkFlagVersions(rm.Out, b.Flags); err != nil {
			return nil, err
		}
		dict["Ff"] = pdf.Integer(uint32(b.Flags))
	}
	if b.AA != nil {
		if err := pdf.CheckVersion(rm.Out, "field AA entry", pdf.V1_3); err != nil {
			return nil, err
		}
		aa, err := b.AA.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["AA"] = aa
	}

	// type-specific entries (V, DV, DA, Q, MaxLen, Opt, …)
	if err := f.fillDict(rm, dict); err != nil {
		return nil, err
	}

	return dict, nil
}

// mergedDetectionKeys are the entries whose presence marks a Widget-subtype
// dictionary as a field merged with its single widget (see isMergedFieldDict
// in the annotation/decode package). The factoring pass must leave at least one
// of these on every merged terminal so it stays recognisable as a field.
var mergedDetectionKeys = []pdf.Name{
	"FT", "T", "TU", "TM", "Ff", "V", "DV", "DA", "Q", "MaxLen", "Opt", "Lock", "SV",
}

// hasMergedDetectionKey reports whether dict carries any entry that marks it as
// a merged field/widget dictionary.
func hasMergedDetectionKey(dict pdf.Dict) bool {
	for _, key := range mergedDetectionKeys {
		if _, ok := dict[key]; ok {
			return true
		}
	}
	return false
}
