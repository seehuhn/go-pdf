// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package fallback

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
)

// Batch generates fallback appearances like [Generator.AddAppearance], for
// many annotations in one go.
//
// Finding the on-state name of a check box or radio button widget takes time
// linear in the number of the field's buttons.  A Batch does this work once
// per field, rather than once per widget, so that generating appearances for
// all widgets of a field takes linear time.
//
// The form fields of the annotations, and the widgets in them, must not
// change while the Batch is in use, except through the Batch itself.  A Batch
// is not safe for concurrent use.
type Batch struct {
	g *Generator

	// onStates holds, for each field seen, the on-state name of each widget
	onStates map[*acroform.ButtonField]map[acroform.Widget]pdf.Name
}

var _ annotation.AppearanceGenerator = (*Batch)(nil)

// NewBatch returns a [Batch] which uses g to draw the appearances.
//
// This implements the [annotation.AppearanceBatcher] interface.
func (g *Generator) NewBatch() annotation.AppearanceGenerator {
	return &Batch{
		g:        g,
		onStates: make(map[*acroform.ButtonField]map[acroform.Widget]pdf.Name),
	}
}

// AddAppearance generates a normal appearance stream for the annotation, as
// [Generator.AddAppearance] does.
func (b *Batch) AddAppearance(a annotation.Annotation) error {
	return b.g.addAppearance(a, b.onState)
}

// onState returns the on-state name of w, a widget of btn.  The names of all
// the field's widgets are found the first time one of them is asked for.
// Giving a widget its appearance does not change any of the names, so they
// stay valid for the lifetime of the batch.
func (b *Batch) onState(btn *acroform.ButtonField, w *annotation.Widget) pdf.Name {
	names, ok := b.onStates[btn]
	if !ok {
		states := btn.OnStates()
		names = make(map[acroform.Widget]pdf.Name, len(btn.Widgets))
		for i, kw := range btn.Widgets {
			if _, seen := names[kw]; !seen {
				names[kw] = states[i]
			}
		}
		b.onStates[btn] = names
	}
	if name, ok := names[w]; ok {
		return name
	}
	// a widget missing from the field's widget list is treated as its first
	return btn.OnState(0)
}
