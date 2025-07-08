package annotation

import "seehuhn.de/go/pdf"

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
	default:
		return extractUnknown(r, dict)
	}
}
