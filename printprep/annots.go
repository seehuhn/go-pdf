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

package printprep

import (
	"bytes"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/annotation/appearance"
	"seehuhn.de/go/pdf/annotation/decode"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

// coordDigits is the precision to which flattened-annotation transform matrices
// are rounded before being written to content.
const coordDigits = 4

// flattenAnnots renders the printable annotations of a page into content that
// is appended after the page's own marks.  It returns the overlay content and
// a map of the form XObjects it references, to be merged into the page's
// resources.  reserved names the /XObject keys the page already uses, which the
// overlay form names avoid.  Interactive, media and hidden annotations are
// dropped; the rest are drawn from their normal appearance, synthesizing one
// via the configured generator when the annotation has none.
func (c *converter) flattenAnnots(annotsObj pdf.Object, reserved map[pdf.Name]bool) ([]byte, map[pdf.Name]pdf.Reference, error) {
	annots, err := pdf.CursorAt(c.x, nil).Array(annotsObj)
	if err != nil || len(annots) == 0 {
		return nil, nil, err
	}

	var buf bytes.Buffer
	forms := make(map[pdf.Name]pdf.Reference)

	// assign each overlay form a name the page's own XObject resources do not
	// use, so flattening cannot shadow existing page content
	counter := 0
	freshName := func() pdf.Name {
		for {
			nm := pdf.Name(fmt.Sprintf("PPAnnot%d", counter))
			counter++
			if !reserved[nm] {
				return nm
			}
		}
	}

	for _, item := range annots {
		ai, err := decode.Annotation(pdf.CursorAt(c.x, nil), item, false)
		if err != nil || ai == nil {
			continue
		}
		if c.annotDropped(ai) {
			continue
		}

		common := ai.GetCommon()
		synthesized := false
		if !annotation.HasAppearance(ai) && annotation.ShouldSynthesizeFallback(ai) {
			// a synthesized appearance is content the file never had; on failure
			// the annotation is simply left undrawn (ap stays nil below)
			if err := c.gen.AddAppearance(ai); err == nil {
				synthesized = true
			}
		}
		ap := annotation.Resolve(common, appearance.Normal)
		if ap == nil || ap.Content == nil {
			continue
		}
		m, ok := appearance.AppearanceToRect(ap, common.Rect)
		if !ok {
			continue
		}

		ref, err := c.embedAppearance(item, common.AppearanceState, ap, synthesized)
		if err != nil {
			return nil, nil, err
		}
		if ref == 0 {
			continue
		}

		name := freshName()
		forms[name] = ref

		ops := []content.Operator{
			{Name: content.OpPushGraphicsState},
			{Name: content.OpTransform, Args: []pdf.Object{
				pdf.Number(pdf.Round(m[0], coordDigits)), pdf.Number(pdf.Round(m[1], coordDigits)),
				pdf.Number(pdf.Round(m[2], coordDigits)), pdf.Number(pdf.Round(m[3], coordDigits)),
				pdf.Number(pdf.Round(m[4], coordDigits)), pdf.Number(pdf.Round(m[5], coordDigits)),
			}},
			{Name: content.OpXObject, Args: []pdf.Object{name}},
			{Name: content.OpPopGraphicsState},
		}
		for _, op := range ops {
			if err := op.Format(&buf); err != nil {
				return nil, nil, err
			}
			buf.WriteByte('\n')
		}
	}

	return buf.Bytes(), forms, nil
}

// embedAppearance writes the annotation's normal appearance to the output as a
// form XObject and returns its reference, or 0 if it could not be embedded.
//
// A source appearance is routed through convertXObject, the same normalization
// as page and form-XObject content (glyf fonts converted to Identity-H, off
// optional content removed).  A synthesized fallback is content the source
// never held; it uses standard fonts and carries no optional content, so it is
// embedded as built.
func (c *converter) embedAppearance(item pdf.Object, state pdf.Name, ap *form.Form, synthesized bool) (pdf.Reference, error) {
	if !synthesized {
		if rawAP := c.rawNormalAppearance(item, state); rawAP != nil {
			obj, err := c.convertXObject(rawAP, 0)
			if err != nil {
				return 0, err
			}
			ref, _ := obj.(pdf.Reference)
			return ref, nil
		}
	}
	obj, err := c.rm.Embed(ap)
	if err != nil {
		return 0, nil // best-effort: leave the annotation undrawn
	}
	ref, _ := obj.(pdf.Reference)
	return ref, nil
}

// rawNormalAppearance returns the source /AP /N form-XObject object of an
// annotation, honouring the appearance state /AS, or nil if there is none.
// The result is the raw object (typically a reference), suitable for
// convertXObject.
func (c *converter) rawNormalAppearance(item pdf.Object, state pdf.Name) pdf.Object {
	cur := pdf.CursorAt(c.x, nil)
	annotDict, err := cur.Dict(item)
	if err != nil || annotDict == nil {
		return nil
	}
	apDict, err := cur.Dict(annotDict["AP"])
	if err != nil || apDict == nil {
		return nil
	}
	n := apDict["N"]
	// /N is either a form-XObject stream or a subdictionary keyed by state
	if stm, _ := cur.Stream(n); stm != nil {
		return n
	}
	if state != "" {
		if sub, _ := cur.Dict(n); sub != nil {
			return sub[state]
		}
	}
	return nil
}

// reservedXObjectNames returns the set of /XObject resource keys the source
// page already uses, so that overlay form names can avoid shadowing them.
func (c *converter) reservedXObjectNames(srcRes pdf.Dict) map[pdf.Name]bool {
	if srcRes == nil {
		return nil
	}
	xobjs, _ := pdf.CursorAt(c.x, nil).Dict(srcRes["XObject"])
	if len(xobjs) == 0 {
		return nil
	}
	names := make(map[pdf.Name]bool, len(xobjs))
	for name := range xobjs {
		names[name] = true
	}
	return names
}

// annotDropped reports whether an annotation must not be flattened into print
// content: interactive, media and link annotations, and any annotation that
// the print visibility rules suppress.
func (c *converter) annotDropped(ai annotation.Annotation) bool {
	switch ai.(type) {
	case *annotation.Link, *annotation.Popup,
		*annotation.Screen, *annotation.Movie, *annotation.Sound,
		*annotation.RichMedia, *annotation.Annot3D:
		return true
	}
	return annotation.AnnotSuppressed(ai, true, false, c.hideMkp, c.ocState)
}
