// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package builder

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/property"
)

func TestMarkedContentInline(t *testing.T) {
	type TestCase struct {
		Prop property.List
		out  string
		err  error
	}

	data, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	x := pdf.NewExtractor(data)

	// create property lists for test cases
	propDirect, _ := property.ExtractList(pdf.CursorAt(x, nil), pdf.Dict{"X": pdf.Integer(1)}, true)
	propIndirect, _ := property.ExtractList(pdf.CursorAt(x, nil), pdf.Dict{"X": pdf.NewReference(1, 0)}, true)

	var testCases = []TestCase{
		{nil, "/test MP", nil},
		{propDirect, "/test <</X 1>> DP", nil},
		{propIndirect, "", ErrNotDirect},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test%02d", i), func(t *testing.T) {
			b := New(content.Page, nil, pdf.V2_0)
			mc := &graphics.MarkedContent{
				Tag:        "test",
				Properties: tc.Prop,
				Inline:     true,
			}

			b.MarkedContentPoint(mc)
			if b.Err != tc.err {
				t.Fatalf("expected %v, got %v", tc.err, b.Err)
			}

			if tc.out != "" {
				buf := &bytes.Buffer{}
				for _, op := range b.Stream {
					op.Format(buf, 0)
				}
				if !strings.Contains(buf.String(), tc.out) {
					t.Errorf("expected %q, got %q", tc.out, buf.String())
				}
			}
		})
	}
}
