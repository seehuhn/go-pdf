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
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/action/triggers"
	"seehuhn.de/go/pdf/annotation/appearance"
)

// PDF 2.0 sections: 12.5.2 12.5.6.19

// Widget is the visual representation of a document form field on a page.
// Usually it corresponds to an [acroform.Field], but it can also be used as a
// purely visual element, to trigger an action when the user clicks on it.
type Widget struct {
	Common

	// Highlight is the annotation's highlighting mode.
	// A highlighting mode other than [HighlightPush] overrides any down
	// appearance defined for the annotation.
	//
	// When writing annotations, an empty name can be used as a shorthand
	// for [HighlightInvert].
	//
	// This corresponds to the /H entry in the PDF annotation dictionary.
	Highlight Highlight

	// Style (optional) specifies the annotation's visual presentation on the
	// page when the viewer constructs a dynamic appearance stream for the
	// annotation.
	//
	// This corresponds to the /MK entry in the PDF annotation dictionary.
	Style *appearance.Characteristics

	// Action (optional) is an action that is performed when the annotation is
	// activated.
	//
	// This corresponds to the /A entry in the PDF annotation dictionary.
	Action pdf.Action

	// AA (optional) specifies the annotation's behaviour in response to
	// various trigger events.
	AA *triggers.Annotation

	// BorderStyle (optional) is a border style dictionary specifying
	// the width and dash pattern that is used in drawing the annotation's
	// border.
	//
	// If this field is set, the Common.Border field is ignored.
	//
	// This corresponds to the /BS entry in the PDF annotation dictionary.
	BorderStyle *BorderStyle

	// Field (optional) is the form field this widget belongs to, or nil if
	// the widget is not part of an interactive form.
	Field acroform.Field
}

var (
	_ Annotation      = (*Widget)(nil)
	_ acroform.Widget = (*Widget)(nil)
)

// AnnotationType returns "Widget".
// This implements the [Annotation] interface.
func (w *Widget) AnnotationType() pdf.Name {
	return "Widget"
}

// ParentField returns [Widget.Field].
//
// This implements the acroform.Widget interface.
func (w *Widget) ParentField() acroform.Field { return w.Field }

// AddWidget adds a widget annotation for the terminal field f at the given
// rectangle and returns it. A field may correspond to several widgets, one per
// place it appears.
//
// The caller must add the returned widget to the annotation list of the page
// it appears on.
func AddWidget(f acroform.Field, rect pdf.Rectangle) *Widget {
	w := &Widget{
		Common:    Common{Rect: rect},
		Highlight: HighlightInvert,
	}
	w.Field = f
	c := f.GetCommon()
	c.Widgets = append(c.Widgets, w)
	return w
}

func (w *Widget) Encode(rm *pdf.ResourceManager) (pdf.Native, error) {
	if w.Field != nil {
		// The widget belongs to a form field. The field's own entries are only
		// known when the form is encoded, so the form writes this widget then
		// (merged with the field, or with a /Parent link), pulling the widget's
		// own entries via [formhooks.EncodeWidgetEntries]. Reserve the reference
		// and defer the write.
		if !fieldHasWidget(w.Field, w) {
			return nil, errors.New("widget's field does not list this widget")
		}
		rm.GetReference(w)
		return nil, nil
	}
	dict, err := w.encodeOwnEntries(rm)
	if err != nil {
		return nil, err
	}
	return dict, nil
}

// encodeOwnEntries builds the widget's own dictionary entries, excluding the
// form-field linkage (/Parent and any folded-in field entries). It is used
// directly for a standalone widget, and by the acroform package — via
// [formhooks.EncodeWidgetEntries] — when it folds a widget into its field.
func (w *Widget) encodeOwnEntries(rm *pdf.ResourceManager) (pdf.Dict, error) {
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

	// H (optional)
	if err := w.Highlight.encodeEntry(rm, dict, "widget annotation H entry"); err != nil {
		return nil, err
	}

	// MK (optional)
	if w.Style != nil {
		mk, err := rm.Embed(w.Style)
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

	return dict, nil
}

// fieldHasWidget reports whether f lists w among its widget annotations. A
// widget's Field back-reference and the field's Widgets slice must agree; use
// [AddWidget] to keep them in sync.
func fieldHasWidget(f acroform.Field, w *Widget) bool {
	return slices.Contains(f.GetCommon().Widgets, acroform.Widget(w))
}
