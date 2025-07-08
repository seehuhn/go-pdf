package annotation

import "seehuhn.de/go/pdf"

// Polygon represents a polygon annotation that displays closed polygons on the page.
// Polygons may have many vertices connected by straight lines, with the first and
// last vertex implicitly connected to close the shape.
type Polygon struct {
	Common
	Markup

	// Vertices (required unless Path is present) is an array of numbers specifying
	// the alternating horizontal and vertical coordinates of each vertex in default
	// user space.
	Vertices []float64

	// BS (optional) is a border style dictionary specifying the width and dash
	// pattern used in drawing the polygon.
	BS pdf.Reference

	// IC (optional) is an array of numbers in the range 0.0 to 1.0 specifying
	// the interior color with which to fill the entire polygon shape.
	// The number of array elements determines the colour space:
	// 0 - No colour; transparent
	// 1 - DeviceGray
	// 3 - DeviceRGB
	// 4 - DeviceCMYK
	IC []float64

	// BE (optional) is a border effect dictionary describing an effect applied
	// to the border described by the BS entry.
	BE pdf.Reference

	// Measure (optional; PDF 1.7) is a measure dictionary that specifies the
	// scale and units that apply to the annotation.
	Measure pdf.Reference

	// Path (optional; PDF 2.0) is an array of n arrays, each supplying operands
	// for path building operators (m, l, or c). If present, Vertices shall not be present.
	// Each array contains pairs of values specifying points for path drawing operations.
	Path [][]float64
}

var _ pdf.Annotation = (*Polygon)(nil)

// AnnotationType returns "Polygon".
// This implements the [pdf.Annotation] interface.
func (p *Polygon) AnnotationType() string {
	return "Polygon"
}

func extractPolygon(r pdf.Getter, dict pdf.Dict) (*Polygon, error) {
	polygon := &Polygon{}

	// Extract common annotation fields
	if err := extractCommon(r, dict, &polygon.Common); err != nil {
		return nil, err
	}

	// Extract markup annotation fields
	if err := extractMarkup(r, dict, &polygon.Markup); err != nil {
		return nil, err
	}

	// Extract polygon-specific fields
	// Vertices (required unless Path is present)
	if vertices, err := pdf.GetArray(r, dict["Vertices"]); err == nil && len(vertices) > 0 {
		coords := make([]float64, len(vertices))
		for i, vertex := range vertices {
			if num, err := pdf.GetNumber(r, vertex); err == nil {
				coords[i] = float64(num)
			}
		}
		polygon.Vertices = coords
	}

	// BS (optional)
	if bs, ok := dict["BS"].(pdf.Reference); ok {
		polygon.BS = bs
	}

	// IC (optional)
	if ic, err := pdf.GetArray(r, dict["IC"]); err == nil && len(ic) > 0 {
		colors := make([]float64, len(ic))
		for i, color := range ic {
			if num, err := pdf.GetNumber(r, color); err == nil {
				colors[i] = float64(num)
			}
		}
		polygon.IC = colors
	}

	// BE (optional)
	if be, ok := dict["BE"].(pdf.Reference); ok {
		polygon.BE = be
	}

	// Measure (optional)
	if measure, ok := dict["Measure"].(pdf.Reference); ok {
		polygon.Measure = measure
	}

	// Path (optional; PDF 2.0)
	if path, err := pdf.GetArray(r, dict["Path"]); err == nil && len(path) > 0 {
		pathArrays := make([][]float64, len(path))
		for i, pathEntry := range path {
			if pathArray, err := pdf.GetArray(r, pathEntry); err == nil {
				coords := make([]float64, len(pathArray))
				for j, coord := range pathArray {
					if num, err := pdf.GetNumber(r, coord); err == nil {
						coords[j] = float64(num)
					}
				}
				pathArrays[i] = coords
			}
		}
		polygon.Path = pathArrays
	}

	return polygon, nil
}

func (p *Polygon) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	dict := pdf.Dict{
		"Type":    pdf.Name("Annot"),
		"Subtype": pdf.Name("Polygon"),
	}

	// Add common annotation fields
	if err := p.Common.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add markup annotation fields
	if err := p.Markup.fillDict(rm, dict); err != nil {
		return nil, zero, err
	}

	// Add polygon-specific fields
	// Vertices (required unless Path is present)
	if p.Path == nil && p.Vertices != nil && len(p.Vertices) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polygon annotation", pdf.V1_5); err != nil {
			return nil, zero, err
		}
		verticesArray := make(pdf.Array, len(p.Vertices))
		for i, vertex := range p.Vertices {
			verticesArray[i] = pdf.Number(vertex)
		}
		dict["Vertices"] = verticesArray
	}

	// BS (optional)
	if p.BS != 0 {
		dict["BS"] = p.BS
	}

	// IC (optional)
	if p.IC != nil {
		icArray := make(pdf.Array, len(p.IC))
		for i, color := range p.IC {
			icArray[i] = pdf.Number(color)
		}
		dict["IC"] = icArray
	}

	// BE (optional)
	if p.BE != 0 {
		dict["BE"] = p.BE
	}

	// Measure (optional)
	if p.Measure != 0 {
		if err := pdf.CheckVersion(rm.Out, "polygon annotation Measure entry", pdf.V1_7); err != nil {
			return nil, zero, err
		}
		dict["Measure"] = p.Measure
	}

	// Path (optional; PDF 2.0)
	if p.Path != nil && len(p.Path) > 0 {
		if err := pdf.CheckVersion(rm.Out, "polygon annotation Path entry", pdf.V2_0); err != nil {
			return nil, zero, err
		}
		pathArray := make(pdf.Array, len(p.Path))
		for i, pathEntry := range p.Path {
			entryArray := make(pdf.Array, len(pathEntry))
			for j, coord := range pathEntry {
				entryArray[j] = pdf.Number(coord)
			}
			pathArray[i] = entryArray
		}
		dict["Path"] = pathArray
	}

	return dict, zero, nil
}