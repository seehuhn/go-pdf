// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

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
//	propList, err := property.ExtractList(c, obj, isDirect)
//	dict := propList.AsDirectDict() // nil if indirect
//
// To create property lists for writing, use the specific types like
// [ActualText] (defined in this package) or
// [seehuhn.de/go/pdf/file.AF] (defined alongside file specifications):
//
//	actual := &property.ActualText{Text: "Hello"}
//	embedded, err := rm.Embed(actual)
//
// The [List] interface provides a common API for both reading and creating
// property lists.
package property
