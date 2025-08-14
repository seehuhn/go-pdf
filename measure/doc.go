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

// Package measure implements PDF measure dictionaries for measurement coordinate systems.
//
// This package provides support for PDF 1.6+ measure dictionaries as defined in section 12.9
// of the PDF specification. These dictionaries enable measurement and coordinate conversion
// within PDF documents, commonly used in technical drawings, maps, and architectural plans.
//
// # Measure Dictionaries
//
// The core [Measure] interface is implemented by different coordinate system types:
//
//   - [RectilinearMeasure]: For rectilinear coordinate systems (RL subtype)
//   - GeospatialMeasure: For geospatial coordinate systems (GEO subtype, not yet implemented)
//
// Use [Extract] to read measure dictionaries from PDF files:
//
//	measure, err := measure.Extract(reader, measureObj)
//	if err != nil {
//		// handle error
//	}
//
//	// Check the type
//	switch m := measure.(type) {
//	case *measure.RectilinearMeasure:
//		// Handle rectilinear measurements
//	default:
//		// Unknown or unsupported measure type
//	}
//
// # Number Format Dictionaries
//
// [NumberFormat] dictionaries specify how numerical measurement values should be formatted
// for display. They support various formatting options including units, precision, fractions,
// and separators.
//
//	format := &measure.NumberFormat{
//		Unit:             "ft",
//		ConversionFactor: 12,
//		Precision:        8,
//		FractionFormat:   measure.FractionFraction,
//	}
//
// Use [FormatMeasurement] to format numeric values according to PDF spec 12.9.2:
//
//	formatted, err := measure.FormatMeasurement(1.75, []*measure.NumberFormat{format})
//	// Result: "1 ft 9 in" (depending on format configuration)
//
// # Viewport Dictionaries
//
// [Viewport] dictionaries define rectangular regions on a page where specific measurement
// information applies. They can contain optional measure dictionaries for coordinate conversion.
//
//	viewport := &measure.Viewport{
//		BBox:    pdf.Rectangle{LLx: 0, LLy: 0, URx: 612, URy: 792},
//		Name:    "Drawing Area",
//		Measure: rectilinearMeasure,
//	}
//
// Use [ViewPortArray] for managing multiple viewports:
//
//	viewports := measure.ViewPortArray{viewport1, viewport2}
//
//	// Find viewport containing a point
//	selected := viewports.Select(vec.Vec2{X: 100, Y: 200})
//
//	// Embed array in PDF
//	embedded, _, err := viewports.Embed(resourceManager)
//
// # PDF Integration
//
// All types implement the standard PDF embedding patterns:
//
//   - Extract functions for reading from PDF objects
//   - Embed methods for writing to PDF objects
//   - Support for SingleUse pattern (direct vs indirect objects)
//   - Proper error handling for malformed input
//
// The package follows PDF specification requirements:
//
//   - Version checking (PDF 1.6+ required for measure dictionaries)
//   - Permissive reading with fallbacks for unknown subtypes
//   - Strict validation during PDF generation
//   - Optimization for common cases (e.g., omitting Y-axis when equal to X-axis)
//
// # Examples
//
// Creating a complete measurement system:
//
//	// Create number formats for different units
//	feetFormat := &measure.NumberFormat{
//		Unit:             "ft",
//		ConversionFactor: 1.0,
//		Precision:        100,
//	}
//
//	inchFormat := &measure.NumberFormat{
//		Unit:             "in",
//		ConversionFactor: 12,
//		Precision:        8,
//		FractionFormat:   measure.FractionFraction,
//	}
//
//	// Create rectilinear measure
//	rm := &measure.RectilinearMeasure{
//		ScaleRatio: "1 in = 10 ft",
//		XAxis:      []*measure.NumberFormat{feetFormat, inchFormat},
//		YAxis:      []*measure.NumberFormat{feetFormat, inchFormat},
//		Distance:   []*measure.NumberFormat{feetFormat, inchFormat},
//		Area:       []*measure.NumberFormat{feetFormat},
//	}
//
//	// Create viewport
//	viewport := &measure.Viewport{
//		BBox:    pdf.Rectangle{LLx: 72, LLy: 72, URx: 540, URy: 720},
//		Name:    "Floor Plan",
//		Measure: rm,
//	}
//
//	// Embed in PDF
//	embedded, _, err := viewport.Embed(resourceManager)
package measure
