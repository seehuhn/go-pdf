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

// Package destination implements PDF destinations as specified in section
// 12.3.2 of PDF 32000-1:2008.
//
// A destination defines a particular view of a document, consisting of:
//   - The page of the document to display
//   - The location of the document window on that page
//   - The magnification (zoom) factor
//
// # Explicit Destinations
//
// Eight explicit destination types are supported, corresponding to the syntax
// defined in Table 149 of the PDF specification:
//
//   - [XYZ] displays the page at given coordinates with a zoom factor.
//   - [Fit] fits the entire page in the window.
//   - [FitH] fits the page width, positioning at a vertical coordinate.
//   - [FitV] fits the page height, positioning at a horizontal coordinate.
//   - [FitR] fits a specified rectangle in the window.
//   - [FitB] fits the page's bounding box in the window (PDF 1.1).
//   - [FitBH] fits the bounding box width, positioning at a vertical coordinate (PDF 1.1).
//   - [FitBV] fits the bounding box height, positioning at a horizontal coordinate (PDF 1.1).
//
// # Named Destinations
//
// [Named] destinations provide indirection: instead of embedding the full
// destination, a name or string is used that references a destination stored
// in the document catalog's Dests dictionary or Names/Dests name tree.
//
// # Optional Coordinates
//
// Some destination types have optional coordinate parameters. Use the [Unset]
// sentinel value to indicate that a parameter should retain its current value.
// For example:
//
//	dest := &destination.XYZ{
//	    Page: pageRef,
//	    Left: 100,              // set to 100
//	    Top:  destination.Unset, // retain current value
//	    Zoom: destination.Unset, // retain current value
//	}
package destination
