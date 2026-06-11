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
// Always invoke this via [pdf.ExtractorGet] so that indirect references are
// resolved and cycle detection covers self- and back-references.
func Annotation(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (annotation.Annotation, error) {
	dict, err := x.GetDictTyped(path, obj, "Annot")
	if err != nil {
		return nil, err
	}

	// a field merged with its single widget is one object that is both a Widget
	// annotation and a form field; decode it as a linked field+widget pair and
	// return the widget half, so the page's /Annots and the field tree's /Kids
	// share one object
	if path != nil && isMergedFieldDict(dict) {
		_, w, err := decodeMergedField(x, path, path.Ref, dict)
		return w, err
	}

	subtype, err := x.GetName(path, dict["Subtype"])
	if err != nil {
		return nil, err
	}

	switch subtype {
	case "Text":
		return decodeText(x, path, dict)
	case "Link":
		return decodeLink(x, path, dict)
	case "FreeText":
		return decodeFreeText(x, path, dict)
	case "Line":
		return decodeLine(x, path, dict)
	case "Square":
		return decodeSquare(x, path, dict)
	case "Circle":
		return decodeCircle(x, path, dict)
	case "Polygon":
		return decodePolygon(x, path, dict)
	case "PolyLine":
		return decodePolyline(x, path, dict)
	case "Highlight", "Underline", "Squiggly", "StrikeOut":
		return decodeTextMarkup(x, path, dict, subtype)
	case "Caret":
		return decodeCaret(x, path, dict)
	case "Stamp":
		return decodeStamp(x, path, dict)
	case "Ink":
		return decodeInk(x, path, dict)
	case "Popup":
		return decodePopup(x, path, dict)
	case "FileAttachment":
		return decodeFileAttachment(x, path, dict)
	case "Sound":
		return decodeSound(x, path, dict)
	case "Movie":
		return decodeMovie(x, path, dict)
	case "Screen":
		return decodeScreen(x, path, dict)
	case "Widget":
		return decodeWidgetBody(x, path, dict)
	case "PrinterMark":
		return decodePrinterMark(x, path, dict)
	case "TrapNet":
		return decodeTrapNet(x, path, dict)
	case "Watermark":
		return decodeWatermark(x, path, dict)
	case "3D":
		return decodeAnnot3D(x, path, dict)
	case "Redact":
		return decodeRedact(x, path, dict)
	case "Projection":
		return decodeProjection(x, path, dict)
	case "RichMedia":
		return decodeRichMedia(x, path, dict)
	default:
		return decodeCustom(x, path, dict)
	}
}
