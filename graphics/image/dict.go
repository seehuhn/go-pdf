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

package image

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/metadata"
)

type Dict struct {
	Width            int
	Height           int
	ColorSpace       color.Space
	BitsPerComponent int
	Intent           graphics.RenderingIntent
	ImageMask        bool
	// TODO(voss): Mask
	Decode      []float64
	Interpolate bool
	Alternates  []*Dict
	// TODO(voss): SMask
	// TODO(voss): SMaskInData
	Name pdf.Name
	// TODO(voss): StructParent
	ID []byte
	// TODO(voss): OPI
	Metadata *metadata.Stream
	// TODO(voss): OC
	// TODO(voss): AF
	// TODO(voss): Measure
	// TODO(voss): PtData
}

var _ pdf.Embedder[pdf.Unused] = (*Dict)(nil)

func (d *Dict) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	panic("not implemented")
}
