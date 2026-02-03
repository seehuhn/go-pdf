// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// Package property provides types for working with marked-content property
// lists in PDF content streams.
//
// # Marked Content
//
// PDF content streams (which define what appears on a page) can contain
// "marked content" sections. These are regions of content bracketed by
// special operators (BDC/EMC) that attach metadata to the enclosed graphics
// operations. For example, marked content is used to:
//
//   - Associate files with portions of page content (AF tag)
//   - Provide replacement text for accessibility (ActualText)
//   - Control optional content visibility (OC tag)
//   - Mark content as artifacts like headers or watermarks (Artifact tag)
//
// # Property Lists
//
// A property list is a PDF dictionary that carries the metadata for a marked
// content section. Property lists can be embedded directly in the content
// stream (if they contain only simple values) or referenced by name from a
// resource dictionary (if they contain indirect references).
//
// # Usage
//
// To read property lists from PDF, use [ExtractList]:
//
//	propList, err := property.ExtractList(x, obj)
//	keys := propList.Keys()
//	val, err := propList.Get("ActualText")
//
// To create property lists for writing, use the specific types like [AF] or
// [ActualText]:
//
//	af := &property.AF{
//	    AssociatedFiles: []*file.Specification{spec},
//	}
//	embedded, err := rm.Embed(af)
//
// The [List] interface provides a common API for both reading and creating
// property lists.
package property
