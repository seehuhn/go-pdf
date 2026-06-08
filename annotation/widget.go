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

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/action"
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
	// widget is not part of an interactive form. It is the back-edge of the field/widget
	// hierarchy: the field's [FieldCommon.Kids] holds this widget, and this
	// widget's Parent holds that field. It is set when the field tree is decoded
	// (see [DecodeInteractiveForm], which page decoding triggers automatically)
	// and is used on encode to write the widget's /Parent entry and, for a
	// single-widget field merged into this widget, to fold in the field's own
	// dictionary entries.
	//
	// Because it forms a cycle with [FieldCommon.Kids], round-trip comparisons
	// must ignore it; the field tree is the authoritative representation.
	Parent Field
}

var _ Annotation = (*Widget)(nil)

// AnnotationType returns "Widget".
// This implements the [Annotation] interface.
func (w *Widget) AnnotationType() pdf.Name {
	return "Widget"
}

// IsFieldNode marks a widget annotation as a possible child in an AcroForm
// field hierarchy. It lets a widget be used as a child node of a form field
// without the annotation package depending on the form package.
func (w *Widget) IsFieldNode() {}

// decodeWidget decodes a widget annotation. A merged field/widget dictionary
// (a Widget that also carries field entries) is decoded as a linked field+widget
// pair by [decodeMergedField]; this returns the widget half so that the page's
// /Annots and the field tree's /Kids share one object.
func decodeWidget(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*Widget, error) {
	if path != nil && isMergedFieldDict(dict) {
		_, w, err := decodeMergedField(x, path, path.Ref, dict)
		return w, err
	}
	return decodeWidgetBody(x, path, dict)
}

// decodeWidgetBody decodes the annotation-level half of a widget dictionary (the
// entries that pertain to a widget annotation, not to a form field). It never
// sets Parent: the field tree owns that linkage.
func decodeWidgetBody(x *pdf.Extractor, path *pdf.CycleCheck, dict pdf.Dict) (*Widget, error) {
	r := x.R
	widget := &Widget{}

	// Extract common annotation fields
	if err := decodeCommon(x, path, &widget.Common, dict); err != nil {
		return nil, err
	}

	// Extract widget-specific fields
	// H (optional) - default to "I" if not specified
	if h, err := pdf.GetName(r, dict["H"]); err == nil && h != "" {
		widget.Highlight = h
	} else {
		widget.Highlight = "I" // PDF default value
	}

	// MK (optional)
	if mk, err := pdf.ExtractorGetOptional(x, path, dict["MK"], appearance.ExtractCharacteristics); err != nil {
		return nil, err
	} else {
		widget.MK = mk
	}

	// A (optional)
	if a, err := pdf.ExtractorGetOptional(x, path, dict["A"], action.Decode); err != nil {
		return nil, err
	} else {
		widget.Action = a
	}

	// AA (optional)
	if aa, err := pdf.ExtractorGetOptional(x, path, dict["AA"], triggers.DecodeAnnotation); err != nil {
		return nil, err
	} else {
		widget.AA = aa
	}

	// BS (optional)
	if bs, err := pdf.ExtractorGetOptional(x, path, dict["BS"], ExtractBorderStyle); err != nil {
		return nil, err
	} else {
		widget.BorderStyle = bs
		if bs != nil {
			// per PDF spec, Border is ignored when BS is present
			widget.Common.Border = nil
		}
	}

	// /Parent is intentionally not read here: the field tree owns the linkage
	// and sets Widget.Parent when the form is decoded.

	// the widget half of a shared /AA keeps only the annotation-level triggers;
	// drop an empty remnant left by the field/widget split
	if widget.AA != nil && widget.AA.IsEmpty() {
		widget.AA = nil
	}

	return widget, nil
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
