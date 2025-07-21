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
	_, singleUse := obj.(pdf.Dict)

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
		return extractText(r, dict, singleUse)
	case "Link":
		return extractLink(r, dict, singleUse)
	case "FreeText":
		return extractFreeText(r, dict, singleUse)
	case "Line":
		return extractLine(r, dict, singleUse)
	case "Square":
		return extractSquare(r, dict, singleUse)
	case "Circle":
		return extractCircle(r, dict, singleUse)
	case "Polygon":
		return extractPolygon(r, dict, singleUse)
	case "PolyLine":
		return extractPolyline(r, dict, singleUse)
	case "Highlight":
		return extractHighlight(r, dict, singleUse)
	case "Underline":
		return extractUnderline(r, dict, singleUse)
	case "Squiggly":
		return extractSquiggly(r, dict, singleUse)
	case "StrikeOut":
		return extractStrikeOut(r, dict, singleUse)
	case "Caret":
		return extractCaret(r, dict, singleUse)
	case "Stamp":
		return extractStamp(r, dict, singleUse)
	case "Ink":
		return extractInk(r, dict, singleUse)
	case "Popup":
		return extractPopup(r, dict, singleUse)
	case "FileAttachment":
		return extractFileAttachment(r, dict, singleUse)
	case "Sound":
		return extractSound(r, dict, singleUse)
	case "Movie":
		return extractMovie(r, dict, singleUse)
	case "Screen":
		return extractScreen(r, dict, singleUse)
	case "Widget":
		return extractWidget(r, dict, singleUse)
	case "PrinterMark":
		return extractPrinterMark(r, dict, singleUse)
	case "TrapNet":
		return extractTrapNet(r, dict, singleUse)
	case "Watermark":
		return extractWatermark(r, dict, singleUse)
	case "3D":
		return extractAnnot3D(r, dict, singleUse)
	case "Redact":
		return extractRedact(r, dict, singleUse)
	case "Projection":
		return extractProjection(r, dict, singleUse)
	case "RichMedia":
		return extractRichMedia(r, dict, singleUse)
	default:
		return extractUnknown(r, dict, singleUse)
	}
}
