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

package decode

import (
	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

// Annotation reads an annotation from a PDF file.
//
// A widget annotation belongs to a form field.  Before a widget is decoded,
// the document's interactive form is read, which links the widget to its
// field ([annotation.Widget.Field]) and makes the form's view of a field
// merged with its widget the one every reader sees.  Errors in the form are
// not reported; a malformed form must not break annotation decoding.
//
// Always invoke this via [pdf.Decode] so that indirect references are
// resolved and cycle detection covers self- and back-references.
func Annotation(c pdf.Cursor, obj pdf.Object, isDirect bool) (annotation.Annotation, error) {
	dict, err := c.DictTyped(obj, "Annot")
	if err != nil {
		return nil, err
	}

	// A direct dictionary has no reference of its own: p.Ref names the
	// enclosing object, which must not be re-decoded as an annotation.
	p := c.Path()
	if p == nil || isDirect || !isWidgetSubtype(c, dict) {
		return annotationBody(c, dict, isDirect)
	}

	// The form's walk reads its widgets through annotationBody, never through
	// this function, so decoding the form here cannot re-enter it.
	// DecodeExclusive single-flights the form, so concurrent decoders share
	// one field tree.
	if m := c.Getter().GetMeta(); m != nil && m.Catalog != nil && m.Catalog.AcroForm != nil {
		_, _ = pdf.DecodeExclusive(pdf.CursorAt(c.Extractor(), nil), m.Catalog.AcroForm, Form)
	}

	// If the form claimed this widget, this is a cache hit and returns the
	// form's widget; otherwise the widget is an orphan and is decoded here.
	// The path is rewound by one step, since p.Ref is the object being decoded.
	return pdf.Decode(pdf.CursorAt(c.Extractor(), p.Parent), p.Ref, annotationBody)
}

// annotationBody decodes an annotation dictionary without first reading the
// interactive form.  It has the same result type as [Annotation], so the two
// share one cache entry per reference.
func annotationBody(c pdf.Cursor, obj pdf.Object, _ bool) (annotation.Annotation, error) {
	dict, err := c.DictTyped(obj, "Annot")
	if err != nil {
		return nil, err
	}
	return decodeAnnotation(c, dict)
}

// repairMissingAppearance supplies an empty appearance for an annotation which
// needs one but has none, so that everything we can read can be written back.
//
// The appearance is empty rather than a generated fallback: the file gives no
// appearance, and inventing one here would fix the annotation's rendering in
// place, taking the choice away from the viewer.
//
// Subtypes whose appearance needs more than a bare form supply it themselves,
// while decoding, and are left alone here.
func repairMissingAppearance(a annotation.Annotation, v pdf.Version) {
	c := a.GetCommon()
	if c.Appearance == nil && annotation.AppearanceRequired(a.AnnotationType(), c.Rect, v) {
		c.Appearance = emptyAppearance(c.Rect)
	}
}

// repairMissingAppearanceState names an appearance state for an annotation
// whose appearance dictionary holds a stream per state but which names none,
// so that everything we can read can be written back.
//
// A file storing appearance streams means them to be seen, so a state is
// picked rather than the annotation left undrawn.  A check box or radio button
// takes its state from the field value; see [buttonAppearanceState].  Anything
// else falls back to [appearance.Dict.AnyState].
func repairMissingAppearanceState(c pdf.Cursor, a annotation.Annotation, dict pdf.Dict) {
	common := a.GetCommon()
	w, isWidget := a.(*annotation.Widget)
	if common.AppearanceState != "" {
		// a widget whose state names no appearance is treated as though it
		// named none; other annotations keep the state they name
		if !isWidget || common.Appearance == nil || len(common.Appearance.NormalMap) == 0 {
			return
		}
		if _, ok := common.Appearance.NormalMap[common.AppearanceState]; ok {
			return
		}
		common.AppearanceState = ""
	}
	if isWidget {
		if state, ok := buttonAppearanceState(c, w, dict); ok {
			common.AppearanceState = state
			return
		}
	}
	common.AppearanceState = common.Appearance.AnyState()
}

// linkWidget attaches a widget to its field.
func linkWidget(f acroform.Field, w *annotation.Widget) {
	w.Field = f
	fc := f.GetCommon()
	fc.Widgets = append(fc.Widgets, w)
}

// reconcileButtonValue makes a check box or radio button's value agree with
// the appearance states of its widgets, once all of them are linked. Where
// the two disagree the appearance state wins, as the specification directs:
// the value becomes the state of a widget which is on, or "Off" when every
// widget names a state and none is on.  Where widgets are on in different
// states, the state the value names wins, else the first; the others are
// switched off, since a field holds one value.
func reconcileButtonValue(f acroform.Field) {
	btn, ok := f.(*acroform.ButtonField)
	if !ok || btn.Variant() == acroform.ButtonPush || len(btn.Widgets) == 0 {
		return
	}
	allNamed := true
	var on pdf.Name
	for _, wi := range btn.Widgets {
		w, ok := wi.(*annotation.Widget)
		if !ok {
			continue
		}
		switch as := w.AppearanceState; as {
		case "":
			allNamed = false
		case "Off":
		default:
			if on == "" || as == btn.V {
				on = as
			}
		}
	}
	switch {
	case on != "":
		btn.V = on
		for _, wi := range btn.Widgets {
			if w, ok := wi.(*annotation.Widget); ok && w.AppearanceState != "" && w.AppearanceState != on {
				w.AppearanceState = "Off"
			}
		}
	case allNamed && btn.V != "" && btn.V != "Off":
		btn.V = "Off"
	}
}

// buttonAppearanceState returns the appearance state a check box or radio
// button widget shows, taken from the value of its field.  The second return
// value is false if the annotation is not such a widget.
//
// A button field's value and the appearance states of its widgets say the same
// thing, so a widget whose file names no state takes one from the value rather
// than from the appearance dictionary alone: a check box which is on must not
// come out unchecked, and one which is off must not come out checked.  A
// widget the value does not name is off, which is also where a radio button
// other than the selected one ends up.
//
// The field of a widget read on its own is not known yet, so the value is
// reconstructed from the /Parent chain the way the field tree does it.  The
// answer therefore does not depend on whether the form was read as well.
func buttonAppearanceState(c pdf.Cursor, w *annotation.Widget, dict pdf.Dict) (pdf.Name, bool) {
	ap := w.Appearance
	if ap == nil || len(ap.NormalMap) == 0 {
		return "", false
	}

	var value pdf.Name
	switch f := w.Field.(type) {
	case *acroform.ButtonField:
		if f.Variant() == acroform.ButtonPush {
			return "", false
		}
		value = f.V
	case nil:
		ctx := applyOwnContext(inheritedFromChain(c, dict), c, dict)
		if ctx.ft != "Btn" || ctx.ff&acroform.FieldPushbutton != 0 {
			return "", false
		}
		value, _ = pdf.Optional(c.Name(ctx.v))
	default:
		return "", false
	}

	if value != "" && value != "Off" {
		if _, ok := ap.NormalMap[value]; ok {
			return value, true
		}
	}
	// the off state needs no appearance stream
	return "Off", true
}

// emptyAppearance builds an appearance dictionary which draws nothing over the
// given rectangle.
//
// The shape mirrors what reading such an appearance back yields: an absent
// Matrix reads as the identity, and absent R and D entries default to N.
// Without this the result would not be a fixed point.
func emptyAppearance(rect pdf.Rectangle) *appearance.Dict {
	empty := &form.Form{
		BBox:   rect,
		Res:    &content.Resources{},
		Matrix: matrix.Identity,
	}
	// The three entries share one form.  Repairs which follow, in particular
	// [repairTrapNetAppearance], copy the form they fix rather than modifying
	// it, so the sharing cannot leak a change from one entry into the others.
	// Anything added here which does modify a form in place has to copy it
	// first, or build a form per entry.
	return &appearance.Dict{
		Normal:    empty,
		RollOver:  empty,
		Down:      empty,
		SingleUse: true,
	}
}

func decodeAnnotation(c pdf.Cursor, dict pdf.Dict) (annotation.Annotation, error) {
	// a field merged with its single widget is one object that is both a Widget
	// annotation and a form field; decode it as a linked field+widget pair and
	// return the widget half, so the page's /Annots and the field tree's /Kids
	// share one object. The field's inheritable attributes are flattened against
	// the context reconstructed from its /Parent chain, matching the field tree.
	if p := c.Path(); p != nil && isMergedFieldDict(c, dict) {
		_, w, err := decodeMergedField(c, p.Ref, dict, inheritedFromChain(c, dict), nil)
		return w, err
	}

	a, err := decodeBySubtype(c, dict)
	if err != nil {
		return nil, err
	}
	repairMissingAppearance(a, pdf.GetVersion(c.Getter()))
	repairMissingAppearanceState(c, a, dict)
	return a, nil
}

// decodeBySubtype decodes an annotation which is not a merged field according
// to its Subtype entry.
func decodeBySubtype(c pdf.Cursor, dict pdf.Dict) (annotation.Annotation, error) {
	subtype, err := c.Name(dict["Subtype"])
	if err != nil {
		return nil, err
	}

	switch subtype {
	case "Text":
		return decodeText(c, dict)
	case "Link":
		return decodeLink(c, dict)
	case "FreeText":
		return decodeFreeText(c, dict)
	case "Line":
		return decodeLine(c, dict)
	case "Square":
		return decodeSquare(c, dict)
	case "Circle":
		return decodeCircle(c, dict)
	case "Polygon":
		return decodePolygon(c, dict)
	case "PolyLine":
		return decodePolyline(c, dict)
	case "Highlight", "Underline", "Squiggly", "StrikeOut":
		return decodeTextMarkup(c, dict, subtype)
	case "Caret":
		return decodeCaret(c, dict)
	case "Stamp":
		return decodeStamp(c, dict)
	case "Ink":
		return decodeInk(c, dict)
	case "Popup":
		return decodePopup(c, dict)
	case "FileAttachment":
		return decodeFileAttachment(c, dict)
	case "Sound":
		return decodeSound(c, dict)
	case "Movie":
		return decodeMovie(c, dict)
	case "Screen":
		return decodeScreen(c, dict)
	case "Widget":
		return decodeWidgetBody(c, dict)
	case "PrinterMark":
		return decodePrinterMark(c, dict)
	case "TrapNet":
		return decodeTrapNet(c, dict)
	case "Watermark":
		return decodeWatermark(c, dict)
	case "3D":
		return decodeAnnot3D(c, dict)
	case "Redact":
		return decodeRedact(c, dict)
	case "Projection":
		return decodeProjection(c, dict)
	case "RichMedia":
		return decodeRichMedia(c, dict)
	default:
		return decodeCustom(c, dict)
	}
}
