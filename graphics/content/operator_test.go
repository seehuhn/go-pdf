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

package content

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
)

func TestIsValidName(t *testing.T) {
	tests := []struct {
		name    string
		op      OpName
		version pdf.Version
		wantErr error
	}{
		// known operators in valid versions
		{"q in PDF 1.0", OpPushGraphicsState, pdf.V1_0, nil},
		{"Q in PDF 1.7", OpPopGraphicsState, pdf.V1_7, nil},
		{"sh in PDF 1.3", OpShading, pdf.V1_3, nil},
		{"gs in PDF 1.2", OpSetExtGState, pdf.V1_2, nil},
		{"ri in PDF 1.1", OpSetRenderingIntent, pdf.V1_1, nil},

		// operators too new for version
		{"sh in PDF 1.0", OpShading, pdf.V1_0, ErrVersion},
		{"sh in PDF 1.2", OpShading, pdf.V1_2, ErrVersion},
		{"gs in PDF 1.0", OpSetExtGState, pdf.V1_0, ErrVersion},
		{"gs in PDF 1.1", OpSetExtGState, pdf.V1_1, ErrVersion},
		{"ri in PDF 1.0", OpSetRenderingIntent, pdf.V1_0, ErrVersion},
		{"SCN in PDF 1.1", OpSetStrokeColorN, pdf.V1_1, ErrVersion},
		{"scn in PDF 1.1", OpSetFillColorN, pdf.V1_1, ErrVersion},
		{"MP in PDF 1.0", OpMarkedContentPoint, pdf.V1_0, ErrVersion},
		{"BX in PDF 1.0", OpBeginCompatibility, pdf.V1_0, ErrVersion},

		// unknown operators
		{"xyz in PDF 1.0", "xyz", pdf.V1_0, ErrUnknown},
		{"xyz in PDF 2.0", "xyz", pdf.V2_0, ErrUnknown},
		{"foo in PDF 1.7", "foo", pdf.V1_7, ErrUnknown},

		// deprecated operators
		{"F in PDF 2.0", OpFillCompat, pdf.V2_0, ErrDeprecated},

		// deprecated operators in old versions (still valid)
		{"F in PDF 1.0", OpFillCompat, pdf.V1_0, nil},
		{"F in PDF 1.7", OpFillCompat, pdf.V1_7, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := Operator{Name: tt.op}
			err := op.isValidName(tt.version)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("IsValidName() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAllOperators(t *testing.T) {
	// verify we have all 75 operators (73 standard + 2 pseudo-operators)
	if len(operators) != 75 {
		t.Errorf("expected 75 operators, got %d", len(operators))
	}

	// verify all operators are valid in PDF 2.0
	for name, info := range operators {
		if info.Deprecated != 0 {
			continue
		}
		op := Operator{Name: name}
		if err := op.isValidName(pdf.V2_0); err != nil {
			t.Errorf("operator %s should be valid in PDF 2.0, got error: %v", name, err)
		}
	}
}

func TestOperatorCategories(t *testing.T) {
	// spot check operators from each category
	categories := map[string][]OpName{
		"graphics state":    {OpPushGraphicsState, OpPopGraphicsState, OpTransform, OpSetLineWidth, OpSetExtGState},
		"path construction": {OpMoveTo, OpLineTo, OpCurveTo, OpCurveToV, OpClosePath, OpRectangle},
		"path painting":     {OpStroke, OpFill, OpFillAndStroke, OpEndPath},
		"clipping":          {OpClipNonZero, OpClipEvenOdd},
		"text objects":      {OpTextBegin, OpTextEnd},
		"text state":        {OpTextSetCharacterSpacing, OpTextSetWordSpacing, OpTextSetFont},
		"text positioning":  {OpTextMoveOffset, OpTextMoveOffsetSetLeading, OpTextSetMatrix, OpTextNextLine},
		"text showing":      {OpTextShow, OpTextShowArray, OpTextShowMoveNextLine, OpTextShowMoveNextLineSetSpacing},
		"type 3 fonts":      {OpType3ColoredGlyph, OpType3UncoloredGlyph},
		"colour":            {OpSetStrokeColorSpace, OpSetFillColorSpace, OpSetStrokeColor, OpSetStrokeGray, OpSetFillGray, OpSetStrokeRGB, OpSetFillRGB, OpSetStrokeCMYK, OpSetFillCMYK},
		"shading":           {OpShading},
		"inline images":     {opBeginInlineImage, opInlineImageData, opEndInlineImage},
		"xobjects":          {OpXObject},
		"marked content":    {OpMarkedContentPoint, OpMarkedContentPointWithProperties, OpBeginMarkedContent, OpBeginMarkedContentWithProperties, OpEndMarkedContent},
		"compatibility":     {OpBeginCompatibility, OpEndCompatibility},
	}

	for category, ops := range categories {
		for _, name := range ops {
			if _, exists := operators[name]; !exists {
				t.Errorf("operator %s from category %s not found in operators map", name, category)
			}
		}
	}
}
