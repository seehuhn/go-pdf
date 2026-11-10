// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

// Package builder provides a type-safe API for constructing PDF content streams.
//
// The [Builder] type offers methods corresponding to PDF graphics operators.
// It tracks graphics state, manages resources, and produces a
// [*content.Operators] segment.  Use [*content.Operators.RawBytes] to obtain
// a reader over the formatted content-stream bytes.
// Errors are reported using the [Builder.Err] field. Once an error occurs,
// all methods return immediately without doing anything.
//
// The following code illustrates how to use a Builder to draw a red
// square with a black outline:
//
//	b := builder.New(content.Page, nil, pdf.V2_0)
//
//	b.SetLineWidth(2)
//	b.SetFillColor(color.DeviceRGB.New(1, 0, 0))
//	b.SetStrokeColor(color.DeviceGray.New(0))
//	b.Rectangle(100, 100, 200, 200)
//	b.FillAndStroke()
//
//	stream, err := b.Harvest()
//	if err != nil {
//		log.Fatal(err)
//	}
//	// Use stream as the content of a form XObject, pattern or Type 3 glyph,
//	// or use content.WriteStream to write it for a specific PDF file.
//	// Use b.Resources for the associated resource dictionary.
package builder
