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
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
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
