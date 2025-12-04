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

// Package builder provides a type-safe API for constructing PDF content streams.
//
// The [Builder] type offers methods corresponding to PDF graphics operators.
// It tracks graphics state, manages resources, and produces a [content.Stream]
// that can be written using [content.Writer]. Errors are reported using the
// Builder.Err field. Once an error occurs, all methods return immediately
// without doing anything.
//
// The following code illustrates how to use a Builder to draw a red
// square with a black outline:
//
//	b := builder.New(content.Page, nil)
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
//	// Use content.Writer to write the stream.
//	// Use b.Resources for the associated resource dictionary.
package builder
