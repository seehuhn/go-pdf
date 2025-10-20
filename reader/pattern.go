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

package reader

import (
	"fmt"

	"seehuhn.de/go/pdf"
)

type patternFromFile struct {
	patternType int
	paintType   int
	obj         pdf.Native
	r           pdf.Getter
}

func readPattern(r pdf.Getter, ref pdf.Object) (*patternFromFile, error) {
	obj, err := pdf.Resolve(r, ref)
	if err != nil {
		return nil, err
	}

	var patternType, paintType pdf.Integer
	switch obj := obj.(type) {
	case pdf.Dict:
		patternType, err := pdf.GetInteger(r, obj["PatternType"])
		if err != nil {
			return nil, err
		}
		if patternType != 2 {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unexpected pattern type %d", patternType),
			}
		}
		paintType = 1
	case *pdf.Stream:
		patternType, err = pdf.GetInteger(r, obj.Dict["PatternType"])
		if err != nil {
			return nil, err
		}
		if patternType != 1 {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unexpected pattern type %d", patternType),
			}
		}
		paintType, err = pdf.GetInteger(r, obj.Dict["PaintType"])
		if err != nil {
			return nil, err
		}
		if paintType != 1 && paintType != 2 {
			return nil, &pdf.MalformedFileError{
				Err: fmt.Errorf("unsupported paint type %d", paintType),
			}
		}
	default:
		return nil, &pdf.MalformedFileError{
			Err: fmt.Errorf("unexpected pattern object type %T", obj),
		}
	}

	return &patternFromFile{
		patternType: int(patternType),
		paintType:   int(paintType),
		obj:         obj,
		r:           r,
	}, nil
}

// PatternType returns 1 for tiling patterns and 2 for shading patterns.
func (p *patternFromFile) PatternType() int {
	return p.patternType
}

// PaintType returns 1 for colored patterns and 2 for uncolored patterns.
func (p *patternFromFile) PaintType() int {
	return p.paintType
}

func (p *patternFromFile) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	copier := pdf.NewCopier(rm.Out(), p.r)
	obj, err := copier.Copy(p.obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
