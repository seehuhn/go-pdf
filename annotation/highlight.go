package annotation

import "seehuhn.de/go/pdf"

// Highlight represents a highlight annotation that appears as highlighted text.
// When opened, it displays a popup window containing the text of the associated note.
type Highlight struct {
	Common
	Markup

	// QuadPoints (required) is an array of 8×n numbers specifying the coordinates
	// of n quadrilaterals in default user space. Each quadrilateral encompasses
	// a word or group of contiguous words in the text underlying the annotation.
	// The coordinates for each quadrilateral are given in the order:
	// x1 y1 x2 y2 x3 y3 x4 y4
	// specifying the quadrilateral's four vertices in counterclockwise order.
	QuadPoints []float64
}

var _ pdf.Annotation = (*Highlight)(nil)

// AnnotationType returns "Highlight".
// This implements the [pdf.Annotation] interface.
func (h *Highlight) AnnotationType() string {
	return "Highlight"
}

func extractHighlight(r pdf.Getter, dict pdf.Dict) (*Highlight, error) {
	highlight := &Highlight{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &highlight.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &highlight.Markup); err != nil {
		return nil, err
	}

	// Extract highlight-specific fields
	// QuadPoints (required)
	if quadPoints, err := pdf.GetArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		coords := make([]float64, len(quadPoints))
		for i, point := range quadPoints {
			if num, err := pdf.GetNumber(r, point); err == nil {
				coords[i] = float64(num)
			}
		}
		highlight.QuadPoints = coords
	}

	return highlight, nil
}

func (h *Highlight) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "highlight annotation", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Highlight"),
	}

	// Add common annotation fields
	if err := h.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := h.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add highlight-specific fields
	// QuadPoints (required)
	if h.QuadPoints != nil && len(h.QuadPoints) > 0 {
		quadArray := make(pdf.Array, len(h.QuadPoints))
		for i, point := range h.QuadPoints {
			quadArray[i] = pdf.Number(point)
		}
		dict["QuadPoints"] = quadArray
	}

	return dict, zero, nil
}