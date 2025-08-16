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

// Decode reads an annotation from a PDF file.
func Decode(r pdf.Getter, obj pdf.Object) (Annotation, error) {
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
		return decodeText(r, dict)
	case "Link":
		return decodeLink(r, dict)
	case "FreeText":
		return decodeFreeText(r, dict)
	case "Line":
		return decodeLine(r, dict)
	case "Square":
		return decodeSquare(r, dict)
	case "Circle":
		return decodeCircle(r, dict)
	case "Polygon":
		return decodePolygon(r, dict)
	case "PolyLine":
		return decodePolyline(r, dict)
	case "Highlight":
		return decodeHighlight(r, dict)
	case "Underline":
		return decodeUnderline(r, dict)
	case "Squiggly":
		return decodeSquiggly(r, dict)
	case "StrikeOut":
		return decodeStrikeOut(r, dict)
	case "Caret":
		return decodeCaret(r, dict)
	case "Stamp":
		return decodeStamp(r, dict)
	case "Ink":
		return decodeInk(r, dict)
	case "Popup":
		return decodePopup(r, dict)
	case "FileAttachment":
		return decodeFileAttachment(r, dict)
	case "Sound":
		return decodeSound(r, dict)
	case "Movie":
		return decodeMovie(r, dict)
	case "Screen":
		return decodeScreen(r, dict)
	case "Widget":
		return decodeWidget(r, dict)
	case "PrinterMark":
		return decodePrinterMark(r, dict)
	case "TrapNet":
		return decodeTrapNet(r, dict)
	case "Watermark":
		return decodeWatermark(r, dict)
	case "3D":
		return decodeAnnot3D(r, dict)
	case "Redact":
		return decodeRedact(r, dict)
	case "Projection":
		return decodeProjection(r, dict)
	case "RichMedia":
		return decodeRichMedia(r, dict)
	default:
		return decodeCustom(r, dict)
	}
}
