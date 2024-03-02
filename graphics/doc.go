// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

// Package graphics allows to write PDF content streams.
//
// The main functionality of this package is provided by the [Writer] type.
// New Writer objects are created using the [NewWriter] function.
// The methods of the Writer type allow to write PDF content streams.
// Errors are reported using the `Writer.Err` field.  Once an error has
// been reported, all writer methods will return immediately without
// doing anything.
//
// The following code illustrates how to use a Writer to draw a red
// square with a black outline:
//
//	out := &bytes.Buffer{}
//	w := graphics.NewWriter(out, pdf.V2_0)
//
//	w.SetLineWidth(2)
//	w.SetFillColor(color.DeviceRGB.New(1, 0, 0))
//	w.Rectangle(100, 100, 200, 200)
//	w.FillAndStroke()
//
//	if w.Err != nil {
//		log.Fatal(w.Err)
//	}
//	// The content stream is now in the buffer "out".
//	// The corresponding resources dictionary is in w.Resources.
package graphics
