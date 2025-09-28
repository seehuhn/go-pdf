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

// PDF 2.0 sections: 12.9

// Viewport represents a rectangular region of a page with measurement information.
type Viewport struct {
	// BBox specifies the location of the viewport on the page.
	BBox pdf.Rectangle

	// Name is a descriptive title of the viewport (optional).
	Name string

	// Measure specifies the scale and units for measurements within the viewport (optional).
	Measure Measure

	// PtData (optional; PDF 2.0) contains extended geospatial point data.
	PtData *PtData

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

	// Extract optional PtData field
	if ptData, err := pdf.Optional(ExtractPtData(r, dict["PtData"])); err != nil {
		return nil, err
	} else {
		vp.PtData = ptData
	}

	return vp, nil
}

// Embed converts the Viewport into a PDF object.
func (v *Viewport) Embed(res *pdf.EmbedHelper) (pdf.Native, error) {

	// Version check for PDF 1.6+
	if err := pdf.CheckVersion(res.Out(), "viewport dictionaries", pdf.V1_6); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	// Optional Type field
	if res.Out().GetOptions().HasAny(pdf.OptDictTypes) {
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
		embedded, err := res.Embed(v.Measure)
		if err != nil {
			return nil, err
		}
		dict["Measure"] = embedded
	}

	// Optional PtData field (PDF 2.0)
	if v.PtData != nil {
		if err := pdf.CheckVersion(res.Out(), "viewport PtData entry", pdf.V2_0); err != nil {
			return nil, err
		}
		embedded, err := res.Embed(v.PtData)
		if err != nil {
			return nil, err
		}
		dict["PtData"] = embedded
	}

	if v.SingleUse {
		return dict, nil
	}

	ref := res.Alloc()
	err := res.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// ViewPortArray represents an array of viewport dictionaries.
type ViewPortArray struct {
	// Viewports contains the array of viewport dictionaries.
	Viewports []*Viewport

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// Select finds the appropriate viewport for a given point.
// Implements the algorithm from PDF spec: examine in reverse order,
// return first viewport whose BBox contains the point.
func (va *ViewPortArray) Select(point vec.Vec2) *Viewport {
	// Iterate backwards through viewports array
	for i := len(va.Viewports) - 1; i >= 0; i-- {
		if va.Viewports[i].BBox.Contains(point) {
			return va.Viewports[i]
		}
	}
	return nil
}

// ExtractViewportArray extracts an array of viewports from a PDF array.
func ExtractViewportArray(r pdf.Getter, obj pdf.Object) (*ViewPortArray, error) {
	_, isIndirect := obj.(pdf.Reference)

	a, err := pdf.GetArray(r, obj)
	if err != nil {
		return nil, err
	}

	viewports := make([]*Viewport, len(a))
	for i, obj := range a {
		vp, err := ExtractViewport(r, obj)
		if err != nil {
			return nil, err
		}
		viewports[i] = vp
	}
	return &ViewPortArray{Viewports: viewports, SingleUse: !isIndirect}, nil
}

// Embed converts the ViewPortArray into a PDF array.
func (va *ViewPortArray) Embed(res *pdf.EmbedHelper) (pdf.Native, error) {

	arr := make(pdf.Array, len(va.Viewports))
	for i, viewport := range va.Viewports {
		embedded, err := res.Embed(viewport)
		if err != nil {
			return nil, err
		}
		arr[i] = embedded
	}

	if va.SingleUse {
		return arr, nil
	}

	ref := res.Alloc()
	err := res.Out().Put(ref, arr)
	if err != nil {
		return nil, err
	}

	return ref, nil
}
