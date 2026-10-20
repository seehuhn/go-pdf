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

package decode

import (
	"testing"

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/optional"
)

func TestLangRoundTrip(t *testing.T) {
	for _, lang := range []language.Tag{language.Und, language.German} {
		roundTripValue(t, pdf.V2_0, &annotation.Text{
			Rect: pdf.Rectangle{LLx: 0, LLy: 0, URx: 10, URy: 10},
			Lang: optional.New(lang),
			Icon: "Note",
		})
	}
}

func TestLangDecode(t *testing.T) {
	tests := []struct {
		name string
		lang pdf.Object
		want optional.Value[language.Tag]
	}{
		{"absent", nil, optional.Value[language.Tag]{}},
		{"empty", pdf.Text(""), optional.New(language.Und)},
		{"und", pdf.Text("und"), optional.New(language.Und)},
		{"tag", pdf.Text("de-CH"), optional.New(language.MustParse("de-CH"))},
		{"unknown language", pdf.Text("qq"), optional.Value[language.Tag]{}},
		{"malformed", pdf.Text("en--US"), optional.Value[language.Tag]{}},
		{"wrong type", pdf.Integer(1), optional.Value[language.Tag]{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dict := pdf.Dict{
				"Subtype": pdf.Name("Text"),
				"Rect":    pdf.Array{pdf.Integer(0), pdf.Integer(0), pdf.Integer(10), pdf.Integer(10)},
			}
			if tt.lang != nil {
				dict["Lang"] = tt.lang
			}
			a, err := Annotation(pdf.CursorAt(pdf.NewExtractor(mock.Getter), nil), dict, false)
			if err != nil {
				t.Fatal(err)
			}
			if got := a.GetCommon().Lang; !got.Equal(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
