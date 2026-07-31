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
// Always invoke this via [pdf.Decode] so that indirect references are
// resolved and cycle detection covers self- and back-references.
func Annotation(c pdf.Cursor, obj pdf.Object, _ bool) (annotation.Annotation, error) {
	dict, err := c.DictTyped(obj, "Annot")
	if err != nil {
		return nil, err
	}

	a, err := decodeAnnotation(c, dict)
	if err != nil {
		return nil, err
	}
	repairMissingAppearance(a, pdf.GetVersion(c.Getter()))
	repairMissingAppearanceState(c, a, dict)
	return a, nil
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
	if common.AppearanceState != "" {
		return
	}
	if w, ok := a.(*annotation.Widget); ok {
		if state, ok := buttonAppearanceState(c, w, dict); ok {
			common.AppearanceState = state
			return
		}
	}
	common.AppearanceState = common.Appearance.AnyState()
}

// buttonAppearanceState returns the appearance state a check box or radio
// button widget shows, taken from the value of its field.  The second return
// value is false if the annotation is not such a widget, or if neither the
// value nor "Off" names one of the widget's normal appearances.
//
// A button field's value and the appearance states of its widgets say the same
// thing, so a widget whose file names no state takes one from the value rather
// than from the appearance dictionary alone: a check box which is on must not
// come out unchecked.  A widget the value does not name is off, which is also
// where a radio button other than the selected one ends up.
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
	if _, ok := ap.NormalMap["Off"]; ok {
		return "Off", true
	}
	return "", false
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
	if p := c.Path(); p != nil && isMergedFieldDict(dict) {
		_, w, err := decodeMergedField(c, p.Ref, dict, inheritedFromChain(c, dict))
		return w, err
	}

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
