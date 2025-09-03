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
func Decode(x *pdf.Extractor, obj pdf.Object) (Annotation, error) {
	dict, err := pdf.GetDictTyped(x.R, obj, "Annot")
	if err != nil {
		return nil, err
	}

	subtype, err := pdf.GetName(x.R, dict["Subtype"])
	if err != nil {
		return nil, err
	}

	switch subtype {
	case "Text":
		return decodeText(x, dict)
	case "Link":
		return decodeLink(x, dict)
	case "FreeText":
		return decodeFreeText(x, dict)
	case "Line":
		return decodeLine(x, dict)
	case "Square":
		return decodeSquare(x, dict)
	case "Circle":
		return decodeCircle(x, dict)
	case "Polygon":
		return decodePolygon(x, dict)
	case "PolyLine":
		return decodePolyline(x, dict)
	case "Highlight", "Underline", "Squiggly", "StrikeOut":
		return decodeTextMarkup(x, dict, subtype)
	case "Caret":
		return decodeCaret(x, dict)
	case "Stamp":
		return decodeStamp(x, dict)
	case "Ink":
		return decodeInk(x, dict)
	case "Popup":
		return decodePopup(x, dict)
	case "FileAttachment":
		return decodeFileAttachment(x, dict)
	case "Sound":
		return decodeSound(x, dict)
	case "Movie":
		return decodeMovie(x, dict)
	case "Screen":
		return decodeScreen(x, dict)
	case "Widget":
		return decodeWidget(x, dict)
	case "PrinterMark":
		return decodePrinterMark(x, dict)
	case "TrapNet":
		return decodeTrapNet(x, dict)
	case "Watermark":
		return decodeWatermark(x, dict)
	case "3D":
		return decodeAnnot3D(x, dict)
	case "Redact":
		return decodeRedact(x, dict)
	case "Projection":
		return decodeProjection(x, dict)
	case "RichMedia":
		return decodeRichMedia(x, dict)
	default:
		return decodeCustom(x, dict)
	}
}
