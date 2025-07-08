package annotation

import "seehuhn.de/go/pdf"

// Squiggly represents a squiggly annotation that appears as jagged underlined text.
// When opened, it displays a popup window containing the text of the associated note.
type Squiggly struct {
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

var _ pdf.Annotation = (*Squiggly)(nil)

// AnnotationType returns "Squiggly".
// This implements the [pdf.Annotation] interface.
func (s *Squiggly) AnnotationType() string {
	return "Squiggly"
}

func extractSquiggly(r pdf.Getter, dict pdf.Dict) (*Squiggly, error) {
	squiggly := &Squiggly{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &squiggly.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &squiggly.Markup); err != nil {
		return nil, err
	}

	// Extract squiggly-specific fields
	// QuadPoints (required)
	if quadPoints, err := pdf.GetArray(r, dict["QuadPoints"]); err == nil && len(quadPoints) > 0 {
		coords := make([]float64, len(quadPoints))
		for i, point := range quadPoints {
			if num, err := pdf.GetNumber(r, point); err == nil {
				coords[i] = float64(num)
			}
		}
		squiggly.QuadPoints = coords
	}

	return squiggly, nil
}

func (s *Squiggly) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "squiggly annotation", pdf.V1_4); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Squiggly"),
	}

	// Add common annotation fields
	if err := s.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := s.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add squiggly-specific fields
	// QuadPoints (required)
	if s.QuadPoints != nil && len(s.QuadPoints) > 0 {
		quadArray := make(pdf.Array, len(s.QuadPoints))
		for i, point := range s.QuadPoints {
			quadArray[i] = pdf.Number(point)
		}
		dict["QuadPoints"] = quadArray
	}

	return dict, zero, nil
}