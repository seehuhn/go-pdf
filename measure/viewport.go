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

package measure

import (
	"seehuhn.de/go/geom/vec"
	"seehuhn.de/go/pdf"
)

// Viewport represents a rectangular region of a page with measurement information.
type Viewport struct {
	// BBox specifies the location of the viewport on the page.
	BBox pdf.Rectangle

	// Name is a descriptive title of the viewport (optional).
	Name string

	// Measure specifies the scale and units for measurements within the viewport (optional).
	Measure Measure

	// TODO(voss): Add PtData field when point data dictionaries are implemented.
	// PtData specifies extended geospatial data (optional, PDF 2.0).

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractViewport extracts a Viewport from a PDF object.
func ExtractViewport(r pdf.Getter, obj pdf.Object) (*Viewport, error) {
	dict, err := pdf.GetDictTyped(r, obj, "Viewport")
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing viewport dictionary")
	}

	vp := &Viewport{}

	// Extract required BBox field
	bbox, err := pdf.GetRectangle(r, dict["BBox"])
	if err != nil {
		return nil, err
	}
	if bbox == nil {
		return nil, pdf.Error("viewport missing required BBox field")
	}
	vp.BBox = *bbox

	// Extract optional Name field
	if dict["Name"] != nil {
		name, err := pdf.Optional(pdf.GetString(r, dict["Name"]))
		if err != nil {
			return nil, err
		}
		vp.Name = string(name)
	}

	// Extract optional Measure field
	if dict["Measure"] != nil {
		measure, err := Extract(r, dict["Measure"])
		if err != nil {
			// Use Optional for permissive reading
			if _, isMalformed := err.(*pdf.MalformedFileError); isMalformed {
				// Ignore malformed measure, continue without it
			} else {
				return nil, err
			}
		} else {
			vp.Measure = measure
		}
	}

	return vp, nil
}

// Embed converts the Viewport into a PDF object.
func (v *Viewport) Embed(res *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// Version check for PDF 1.6+
	if err := pdf.CheckVersion(res.Out, "viewport dictionaries", pdf.V1_6); err != nil {
		return nil, zero, err
	}

	dict := pdf.Dict{}

	// Optional Type field
	if res.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Viewport")
	}

	// Required BBox field
	dict["BBox"] = &v.BBox

	// Optional Name field
	if v.Name != "" {
		dict["Name"] = pdf.String(v.Name)
	}

	// Optional Measure field
	if v.Measure != nil {
		embedded, _, err := v.Measure.Embed(res)
		if err != nil {
			return nil, zero, err
		}
		dict["Measure"] = embedded
	}

	if v.SingleUse {
		return dict, zero, nil
	}

	ref := res.Out.Alloc()
	err := res.Out.Put(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// ViewPortArray represents an array of viewport dictionaries.
type ViewPortArray []*Viewport

// Select finds the appropriate viewport for a given point.
// Implements the algorithm from PDF spec: examine in reverse order,
// return first viewport whose BBox contains the point.
func (va ViewPortArray) Select(point vec.Vec2) *Viewport {
	// Iterate backwards through viewports array
	for i := len(va) - 1; i >= 0; i-- {
		if va[i].BBox.Contains(point) {
			return va[i]
		}
	}
	return nil
}

// SelectViewport finds the appropriate viewport for a given point.
// Deprecated: Use ViewPortArray.Select method instead.
func SelectViewport(point vec.Vec2, viewports []*Viewport) *Viewport {
	va := ViewPortArray(viewports)
	return va.Select(point)
}

// ExtractViewportArray extracts an array of viewports from a PDF array.
func ExtractViewportArray(r pdf.Getter, arr pdf.Array) (ViewPortArray, error) {
	viewports := make(ViewPortArray, len(arr))
	for i, obj := range arr {
		vp, err := ExtractViewport(r, obj)
		if err != nil {
			return nil, err
		}
		viewports[i] = vp
	}
	return viewports, nil
}

// Embed converts the ViewPortArray into a PDF array.
func (va ViewPortArray) Embed(res *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	arr := make(pdf.Array, len(va))
	for i, viewport := range va {
		embedded, _, err := viewport.Embed(res)
		if err != nil {
			return nil, zero, err
		}
		arr[i] = embedded
	}
	return arr, zero, nil
}

// EmbedViewportArray embeds an array of viewports into a PDF array.
// Deprecated: Use ViewPortArray.Embed method instead.
func EmbedViewportArray(res *pdf.ResourceManager, viewports []*Viewport) (pdf.Array, error) {
	va := ViewPortArray(viewports)
	embedded, _, err := va.Embed(res)
	if err != nil {
		return nil, err
	}
	return embedded.(pdf.Array), nil
}
