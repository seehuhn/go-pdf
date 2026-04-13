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

// Package jbig2 provides a high-level JBIG2 encoder for 1-bit (bi-level)
// images in PDF files.
//
// # Overview
//
// JBIG2 is a bi-level image compression standard (ISO/IEC 14492) that
// PDF supports via the JBIG2Decode filter.  It compresses black-and-white
// images by encoding regions of pixels.  Three region types are
// available:
//
//   - Generic regions encode arbitrary bitmaps (see [Image.AddGenericRegion]).
//   - Text regions place instances of shared symbols, useful for pages
//     of text where the same glyphs repeat (see [Image.AddTextRegion]).
//   - Halftone regions render grayscale images using pattern tiles
//     (see [Image.AddHalftoneRegion]).
//
// An [Image] collects one or more regions into a single PDF image stream.
// It implements [graphics.ImageData] and can be used as the Data field of
// an [image.Dict] (for a 1-bit DeviceGray image) or the Source field of
// an [image.Mask] (for a stencil mask).
//
// # Sharing globals
//
// When multiple images reuse the same symbols or patterns, a [Globals]
// instance stores the shared dictionaries in a separate PDF stream.
// This avoids duplicating dictionary data across images.
//
//	g := jbig2.NewGlobals()
//	id, _ := g.AddSymbol(symbolBitmap)
//
//	im1 := jbig2.NewImage(w, h, g)
//	im1.AddTextRegion(&jbig2.TextRegion{...})
//
//	im2 := jbig2.NewImage(w, h, g)
//	im2.AddGenericRegion(someBitmap, 0, 0, nil)
//
//	mask := &image.Mask{Width: w, Height: h, Source: im1}
//	page.DrawXObject(mask)
//
// Once a [Globals] is embedded (either directly or via its first image),
// it becomes frozen.  Subsequent calls to [Globals.AddSymbol] and friends
// return an error.
package jbig2
