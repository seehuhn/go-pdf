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

package graphics

import (
	"bytes"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
)

// TestStateApplyToError tests that the error message returned by ApplyTo
// refers to the correct parameter.
func TestStateApplyToError(t *testing.T) {
	forbidden := []StateBits{
		0,
		StateTextKnockout,
		StateStrokeAdjustment,
		StateFillAlpha,
		StateStrokeAlpha | StateFillAlpha | StateAlphaSourceFlag,
		StateOverprint | StateOverprintMode,
	}
	allowed := []StateBits{
		0,
		StateStrokeColor,
		StateTextCharacterSpacing | StateTextWordSpacing,
		OpStateBits,
	}

	data := pdf.NewData(pdf.V2_0)
	rm := pdf.NewResourceManager(data)

	buf := &bytes.Buffer{}
	for _, bad := range forbidden {
		for _, good := range allowed {
			buf.Reset()
			w := NewWriter(buf, rm)

			state := NewState()
			state.TextFont = dummyFont{}
			state.Set = good | bad
			state.ApplyTo(w)

			if bad == 0 {
				if w.Err != nil {
					t.Errorf("unexpected error: %s", w.Err)
				}
				continue
			}

			if w.Err == nil {
				t.Errorf("expected error, but got none")
				continue
			}

			// Make sure that at least one of the bad parameters is mentioned
			// in the error message, and none of the allowed parameters are
			// mentioned.
			foundGood := -1
			foundBad := -1
			msg := w.Err.Error()
			for i, state := 0, StateBits(1); state < stateFirstUnused; i, state = i+1, state<<1 {
				if strings.Contains(msg, stateNames[i]) {
					if state&good != 0 {
						foundGood = i
					}
					if state&bad != 0 {
						foundBad = i
					}
				}
			}
			if foundGood >= 0 {
				t.Errorf("message %q lists allowed parameter %q",
					msg, stateNames[foundGood])
			}
			if foundBad < 0 {
				t.Errorf("message %q does not list any forbidden parameters",
					msg)
			}
		}
	}
}

type dummyFont struct{}

func (f dummyFont) PDFObject() pdf.Object {
	return nil
}

func (f dummyFont) WritingMode() int {
	return 0
}

func (f dummyFont) ForeachWidth(s pdf.String, yield func(width float64, isSpace bool)) {
	// pass
}
