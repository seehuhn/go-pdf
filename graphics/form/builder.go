// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package form

import (
	"bytes"
	"time"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Properties has data for a form XObject.
//
// See section 8.10 of ISO 32000-2:2020 for details.
type Properties struct {
	BBox         *pdf.Rectangle
	Matrix       matrix.Matrix
	Metadata     pdf.Reference
	PieceInfo    pdf.Object
	LastModified time.Time
	// TODO(voss): StructParent, StructParents
	OC          pdf.Object
	DefaultName pdf.Name
	AF          pdf.Object
	Measure     pdf.Object
	PtData      pdf.Object
}

// IsDirect returns true if the Properties object does not contain any
// references to indirect PDF objects.
func (p *Properties) IsDirect() bool {
	return p.Metadata == 0 &&
		pdf.IsDirect(p.PieceInfo) &&
		pdf.IsDirect(p.OC) &&
		pdf.IsDirect(p.AF) &&
		pdf.IsDirect(p.Measure) &&
		pdf.IsDirect(p.PtData)
}

// A Builder is used to create a form XObject.
type Builder struct {
	*graphics.Writer
	*Properties
}

func NewBuilder(rm *pdf.ResourceManager, prop *Properties) *Builder {
	contents := graphics.NewWriter(&bytes.Buffer{}, rm)
	return &Builder{
		Writer:     contents,
		Properties: prop,
	}
}

func (f *Builder) Finish() (graphics.XObject, error) {
	form := &Form{
		Properties: f.Properties,
		Resources:  f.Writer.Resources,
		Contents:   f.Writer.Content.(*bytes.Buffer).Bytes(),
		RM:         f.Writer.RM,
	}

	// disable the writer to prevent further writes
	f.Writer = nil

	return form, nil
}
