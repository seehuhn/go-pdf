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
	"seehuhn.de/go/pdf/action/triggers"
)

// PDF 2.0 sections: 12.7.4.1 12.7.4.2

// TreeNode is a node of a field tree.
// This must be either a [*Group] or a [Field].
type TreeNode interface {
	// PartialName returns the node's partial field name (the /T entry). An
	// empty value means the node contributes no component to the fully
	// qualified names of its descendants.
	PartialName() string
}

// Field is a terminal field in a PDF interactive form: a [FieldTx] (text),
// [FieldBtn] (button), [FieldChoice] (choice), or [FieldSig] (signature).
//
// In a PDF file, field attributes may be inherited from ancestors in the field
// tree (12.7.4.1). This package hides that: a decoded field carries fully
// resolved ("flattened") attribute values, and the encoder restores the
// inheritance as a storage optimization, invisibly. There are therefore no
// inheritance helpers; read a field's attributes directly from its fields.
//
// A terminal field is rendered on a page by one or more widget annotations
// ("seehuhn.de/go/pdf/annotation".Widget); see [Field.Widgets]. A field with
// exactly one widget is written as a single combined field/widget dictionary;
// this merging is automatic and transparent.
//
// Fields are not written individually; [InteractiveForm.Encode] writes the
// whole tree when the form is stored.
type Field interface {
	TreeNode

	// FieldType returns the PDF field type, one of "Btn", "Tx", "Ch", or "Sig".
	FieldType() pdf.Name

	// Flags returns the field's flags.
	Flags() FieldFlags

	// Widgets returns the field's widget annotations, one for each place the
	// field appears on a page.
	Widgets() []Widget

	// AddWidget appends a widget annotation to the field. Prefer
	// "seehuhn.de/go/pdf/annotation".AddWidget, which also sets the widget's
	// parent link.
	AddWidget(Widget)

	base() *fieldBase
	fillTypeDict(rm *pdf.ResourceManager, dict pdf.Dict) error
}

// Widget is the interface satisfied by a terminal field's widget annotations.
// Its only implementation is "seehuhn.de/go/pdf/annotation".Widget; the
// interface exists so that the acroform package can refer to widgets without
// importing the annotation package.
type Widget interface {
	pdf.Encoder

	// FieldParent returns the field this widget belongs to, or nil.
	FieldParent() Field
}

// fieldBase holds the attributes shared by all terminal field types. The four
// concrete types embed it. Its exported fields can be set directly; the
// unexported fields carry the widget list and per-encoding state.
type fieldBase struct {
	// Name (optional) is the partial field name. An empty value means the
	// field has no name of its own and does not contribute to fully qualified
	// field names.
	//
	// This corresponds to the /T entry in the PDF field dictionary.
	Name string

	// TU (optional) is an alternative field name used in the user interface
	// and for accessibility.
	TU string

	// TM (optional) is the mapping name used when exporting field data.
	TM string

	// Ff holds the field flags. The zero value means no flags are set.
	//
	// This corresponds to the /Ff entry in the PDF field dictionary.
	Ff FieldFlags

	// AA (optional) is the field's additional-actions dictionary.
	AA *triggers.Form

	widgets []Widget
	enc     *fieldEncState
}

// fieldEncState records, for one encoding pass, how a field's widget
// annotations should tie themselves to the field tree. [InteractiveForm.Encode]
// sets it; the annotation package reads it through the formhooks seam when it
// later writes the widgets.
type fieldEncState struct {
	rm        *pdf.ResourceManager
	parentRef pdf.Reference // the enclosing group's reference, or 0 for a root
	fieldRef  pdf.Reference // the field's own reference, or its single widget's
	entries   pdf.Dict      // the field's own entries; non-nil only when merged
}

// PartialName implements the [TreeNode] interface.
func (b *fieldBase) PartialName() string { return b.Name }

// Flags implements the [Field] interface.
func (b *fieldBase) Flags() FieldFlags { return b.Ff }

// Widgets implements the [Field] interface.
func (b *fieldBase) Widgets() []Widget { return b.widgets }

// AddWidget implements the [Field] interface.
func (b *fieldBase) AddWidget(w Widget) { b.widgets = append(b.widgets, w) }

func (b *fieldBase) base() *fieldBase { return b }

// terminalEntries builds the dictionary entries of a terminal field — its
// flattened own entries (FT, T, TU, TM, Ff, AA, and the type-specific entries),
// excluding /Parent and /Kids. The factoring pass may later remove inheritable
// entries that are hoisted into an ancestor.
func terminalEntries(rm *pdf.ResourceManager, f Field) (pdf.Dict, error) {
	if err := pdf.CheckVersion(rm.Out, "interactive form field", pdf.V1_2); err != nil {
		return nil, err
	}

	b := f.base()
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
	if b.Ff != 0 {
		if err := checkFlagVersions(rm.Out, b.Ff); err != nil {
			return nil, err
		}
		dict["Ff"] = pdf.Integer(uint32(b.Ff))
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
	if err := f.fillTypeDict(rm, dict); err != nil {
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
