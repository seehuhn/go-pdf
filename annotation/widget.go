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

import (
	"errors"
	"fmt"
	"maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation/appearance"
)

// PDF 2.0 sections: 12.5.2 12.5.6.19

// Widget represents a widget annotation used by interactive forms to represent
// the appearance of fields and to manage user interactions. When a field has
// only a single associated widget annotation, the contents of the field
// dictionary and the annotation dictionary may be merged into a single dictionary.
type Widget struct {
	Common

	// Highlight (optional) is the annotation's highlighting mode, the visual effect
	// that is used when the mouse button is pressed or held down inside
	// its active area. Valid values:
	// - "N" (None): No highlighting
	// - "I" (Invert): Invert the colors used to display the contents
	// - "O" (Outline): Stroke the colors used to display the annotation border
	// - "P" (Push): Display the annotation's down appearance
	// - "T" (Toggle): Same as P (which is preferred)
	// An empty value selects the Invert mode ("I").
	Highlight pdf.Name

	// MK (optional) is an appearance characteristics dictionary that is
	// used in constructing a dynamic appearance stream specifying the
	// annotation's visual presentation on the page.
	MK *appearance.Characteristics

	// Action (optional; PDF 1.1) is an action that is performed when the
	// annotation is activated.
	Action pdf.Action

	// AA (optional; PDF 1.2) is an additional-actions dictionary defining
	// the annotation's behaviour in response to various trigger events.
	//
	// This corresponds to the /AA entry in the PDF annotation dictionary.
	AA *triggers.Annotation

	// BorderStyle (optional; PDF 1.2) is a border style dictionary specifying
	// the width and dash pattern that is used in drawing the annotation's
	// border.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// Parent (optional) is the form field this widget belongs to, or nil if the
	// widget is not part of an interactive form. It is the back-edge of the
	// field/widget hierarchy: the field's [acroform.FieldCommon.Kids] holds this
	// widget, and this widget's Parent holds that field. It is set when the field
	// tree is decoded (page decoding triggers this automatically) and is used on
	// encode to write the widget's /Parent entry and, for a single-widget field
	// merged into this widget, to fold in the field's own dictionary entries.
	//
	// Because it forms a cycle with [acroform.FieldCommon.Kids], round-trip
	// comparisons must ignore it; the field tree is the authoritative
	// representation.
	Parent acroform.Field
}

var (
	_ Annotation    = (*Widget)(nil)
	_ acroform.Node = (*Widget)(nil)
)

// AnnotationType returns "Widget".
// This implements the [Annotation] interface.
func (w *Widget) AnnotationType() pdf.Name {
	return "Widget"
}

// IsFieldNode marks a widget annotation as a possible child in an AcroForm
// field hierarchy, satisfying the acroform.Node interface.
func (w *Widget) IsFieldNode() {}

// SetFieldParent links this widget to the form field f as its parent. It is the
// back-edge of the field/widget hierarchy (see [Widget.Parent]) and is set by
// the field tree on encode and decode.
func (w *Widget) SetFieldParent(f acroform.Field) { w.Parent = f }

// AddWidget adds a widget annotation for the terminal field f at the given
// rectangle and returns it. A terminal field may have several widgets, one per
// place it appears. The caller must add the returned widget to the annotation
// list of the page it appears on.
func AddWidget(f acroform.Field, rect pdf.Rectangle) *Widget {
	w := &Widget{
		Common:    Common{Rect: rect},
		Highlight: "I",
	}
	w.Parent = f
	c := f.GetFieldCommon()
	c.Kids = append(c.Kids, w)
	return w
}

func (w *Widget) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out, "widget annotation", pdf.V1_2); err != nil {
		return nil, err
	}

	if w.BorderStyle != nil && w.Common.Border != nil {
		return nil, errors.New("Border and BorderStyle are mutually exclusive")
	}

	dict := pdf.Dict{
		"Subtype": pdf.Name("Widget"),
	}

	// Add common annotation fields
	if err := w.Common.fillDict(rm, dict, isMarkup(w), w.BorderStyle != nil); err != nil {
		return nil, err
	}

	// Add widget-specific fields
	// H (optional) - only write if not the default value "I"
	if w.Highlight != "" && w.Highlight != "I" {
		dict["H"] = w.Highlight
	}

	// MK (optional)
	if w.MK != nil {
		mk, err := rm.Embed(w.MK)
		if err != nil {
			return nil, err
		}
		dict["MK"] = mk
	}

	// A (optional)
	if w.Action != nil {
		if err := pdf.CheckVersion(rm.Out, "widget annotation A entry", pdf.V1_1); err != nil {
			return nil, err
		}
		encoded, err := w.Action.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["A"] = encoded
	}

	// AA (optional)
	if w.AA != nil {
		aa, err := w.AA.Encode(rm)
		if err != nil {
			return nil, err
		}
		dict["AA"] = aa
	}

	// BS (optional)
	if w.BorderStyle != nil {
		bs, err := rm.Embed(w.BorderStyle)
		if err != nil {
			return nil, err
		}
		dict["BS"] = bs
	}

	// tie the widget to its form field: for a field whose single widget this is,
	// fold the field's own entries in here (one merged field/widget dictionary);
	// otherwise write /Parent pointing at the field's object.
	if w.Parent != nil {
		return foldFieldIntoWidget(rm, w, dict)
	}

	return dict, nil
}

// foldFieldIntoWidget ties an encoded widget dictionary to its form field
// (w.Parent). If the widget is the single widget of a terminal field, the
// field's own entries are folded into the widget dictionary, producing one
// merged field/widget object, and /Parent names the field's own parent (absent
// for a root field). Otherwise the widget gets a /Parent pointing at the field's
// own object. It is called from [Widget.Encode] and keeps the merge rules here.
//
// Field entries take precedence over the widget's, except for the shared /AA,
// whose field half (K/F/V/C) and widget half (E/X/Fo/Bl/…) are combined.
func foldFieldIntoWidget(rm *pdf.ResourceManager, w *Widget, dict pdf.Dict) (pdf.Native, error) {
	f := w.Parent

	// w's entries are folded into the field only when w is the sole widget of a
	// terminal field: no sub-fields and exactly one widget child, which is w
	var theWidget acroform.Node
	widgetCount := 0
	hasSubfield := false
	for _, kid := range f.GetFieldCommon().Kids {
		if _, ok := kid.(acroform.Field); ok {
			hasSubfield = true
		} else {
			widgetCount++
			theWidget = kid
		}
	}
	if hasSubfield || widgetCount != 1 || theWidget != acroform.Node(w) {
		// a non-merged widget kid of a multi-widget field
		dict["Parent"] = rm.GetReference(f)
		return dict, nil
	}

	// the single widget of a terminal field: fold the field's entries in
	entries, err := acroform.FieldEntries(rm, f)
	if err != nil {
		return nil, err
	}
	for k, v := range entries {
		if existing, exists := dict[k]; exists && k == "AA" {
			merged, err := mergeAADicts(v, existing)
			if err != nil {
				return nil, err
			}
			dict[k] = merged
			continue
		}
		dict[k] = v
	}
	if p := acroform.ParentOf(f); p != nil {
		dict["Parent"] = rm.GetReference(p)
	}
	return dict, nil
}

// mergeAADicts combines the field-level (K/F/V/C) and annotation-level
// (E/X/Fo/Bl/…) halves of a merged field's shared /AA dictionary. The two key
// sets are disjoint by construction, so an overlap signals a bug.
func mergeAADicts(field, widget pdf.Object) (pdf.Dict, error) {
	fd, ok := field.(pdf.Dict)
	if !ok {
		return nil, errors.New("field AA did not encode to a dictionary")
	}
	wd, ok := widget.(pdf.Dict)
	if !ok {
		return nil, errors.New("widget AA did not encode to a dictionary")
	}
	merged := maps.Clone(fd)
	for k, v := range wd {
		if _, exists := merged[k]; exists {
			return nil, fmt.Errorf("conflicting AA entry %q", k)
		}
		merged[k] = v
	}
	return merged, nil
}
