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

package action

import (
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

var errTestRead = errors.New("test read failure")

// failGetter is a pdf.Getter whose Get fails with a non-malformed error.
type failGetter struct{}

func (failGetter) GetMeta() *pdf.MetaInfo { return &pdf.MetaInfo{Version: pdf.V2_0} }

func (failGetter) Get(pdf.Reference, bool) (pdf.Native, error) {
	return nil, errTestRead
}

// TestReadErrorPropagates checks that a genuine read error is returned
// rather than being treated as an absent entry.
func TestReadErrorPropagates(t *testing.T) {
	c := pdf.NewCursor(failGetter{})
	ref := pdf.NewReference(1, 0)

	if _, err := readStringOrStream(c, ref); !errors.Is(err, errTestRead) {
		t.Errorf("readStringOrStream: err = %v, want read error", err)
	}
}

// TestReadMalformedIsAbsent checks that a dangling reference is read like
// an absent entry.
func TestReadMalformedIsAbsent(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	ref := pdf.NewReference(99, 0)

	if v, err := readStringOrStream(c, ref); v != nil || err != nil {
		t.Errorf("readStringOrStream: v = %v, err = %v, want absent", v, err)
	}
}

// TestDecodeReadErrorPropagates checks that a genuine read error in an
// optional entry is returned by the action decoders instead of being dropped.
func TestDecodeReadErrorPropagates(t *testing.T) {
	c := pdf.NewCursor(failGetter{})
	bad := pdf.NewReference(1, 0)
	spec := pdf.Dict{"Type": pdf.Name("Filespec"), "FS": pdf.Name("URL"), "F": pdf.String("a.pdf")}

	cases := []pdf.Dict{
		{"S": pdf.Name("Launch"), "F": spec, "Win": bad},
		{"S": pdf.Name("SubmitForm"), "F": pdf.String("x"), "Fields": bad},
		{"S": pdf.Name("SubmitForm"), "F": pdf.String("x"), "Flags": bad},
		{"S": pdf.Name("ResetForm"), "Fields": bad},
		{"S": pdf.Name("ResetForm"), "Flags": bad},
		{"S": pdf.Name("Movie"), "T": pdf.String("m"), "FWScale": pdf.Array{bad, pdf.Integer(1)}},
		{"S": pdf.Name("Movie"), "T": pdf.String("m"), "FWScale": pdf.Array{pdf.Integer(1), bad}},
		{"S": pdf.Name("Movie"), "T": pdf.String("m"), "FWPosition": pdf.Array{bad, pdf.Real(0.5)}},
		{"S": pdf.Name("Movie"), "T": pdf.String("m"), "FWPosition": pdf.Array{pdf.Real(0.5), bad}},
		{"S": pdf.Name("URI"), "URI": pdf.String("x"), "Next": bad},
		{"S": pdf.Name("URI"), "URI": pdf.String("x"), "Next": pdf.Array{bad}},
	}
	for i, dict := range cases {
		if _, err := Decode(c, dict, true); !errors.Is(err, errTestRead) {
			t.Errorf("case %d (%s): err = %v, want read error", i, dict["S"], err)
		}
	}
}
