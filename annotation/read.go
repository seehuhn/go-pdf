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
	"seehuhn.de/go/pdf"
)

// Extract extracts an annotation from a PDF file.
func Extract(r pdf.Getter, obj pdf.Object) (pdf.Annotation, error) {
	dict, err := pdf.GetDictTyped(r, obj, "Annot")
	if err != nil {
		return nil, err
	}

	subtype, err := pdf.GetName(r, dict["Subtype"])
	if err != nil {
		return nil, err
	}

	switch subtype {
	case "Text":
		return extractText(r, dict)
	case "Link":
		return extractLink(r, dict)
	case "FreeText":
		return extractFreeText(r, dict)
	case "Line":
		return extractLine(r, dict)
	case "Square":
		return extractSquare(r, dict)
	case "Circle":
		return extractCircle(r, dict)
	case "Polygon":
		return extractPolygon(r, dict)
	case "PolyLine":
		return extractPolyline(r, dict)
	case "Highlight":
		return extractHighlight(r, dict)
	case "Underline":
		return extractUnderline(r, dict)
	case "Squiggly":
		return extractSquiggly(r, dict)
	case "StrikeOut":
		return extractStrikeOut(r, dict)
	case "Caret":
		return extractCaret(r, dict)
	case "Stamp":
		return extractStamp(r, dict)
	case "Ink":
		return extractInk(r, dict)
	case "Popup":
		return extractPopup(r, dict)
	case "FileAttachment":
		return extractFileAttachment(r, dict)
	case "Sound":
		return extractSound(r, dict)
	case "Movie":
		return extractMovie(r, dict)
	case "Screen":
		return extractScreen(r, dict)
	case "Widget":
		return extractWidget(r, dict)
	case "PrinterMark":
		return extractPrinterMark(r, dict)
	case "TrapNet":
		return extractTrapNet(r, dict)
	case "Watermark":
		return extractWatermark(r, dict)
	case "3D":
		return extractAnnot3D(r, dict)
	case "Redact":
		return extractRedact(r, dict)
	case "Projection":
		return extractProjection(r, dict)
	case "RichMedia":
		return extractRichMedia(r, dict)
	default:
		return extractUnknown(r, dict)
	}
}
