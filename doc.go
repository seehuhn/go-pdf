// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

// Package pdf provides support for reading and writing PDF files.
//
// The package treats PDF files as containers containing a sequence of objects
// (typically Dictionaries and Streams).  Object are written sequentially, but
// can be read in any order.  These objects represent pages of text, fonts,
// images and so on.  Subpackages implement support to produce PDF files
// representing pages of text and images.
//
// A [Reader] can be used to read objects from an existing PDF file:
//
//	r, err := pdf.Open("in.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Close()
//	... use r.Catalog to locate objects in the file ...
//
// A [Writer] can be used to write objects to a new PDF file:
//
//	w, err := pdf.Create("out.pdf")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	... add objects to the document using w.Write() and w.OpenStream() ...
//
//	w.Catalog.Pages = ... // set the page tree
//
//	err = out.Close()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The following classes represent the native PDF object types: [Array],
// [Bool], [Dict], [Integer], [Name], [Real], [Reference], [*Stream], [String].
// All of these implement the [Object] interface.
package pdf
