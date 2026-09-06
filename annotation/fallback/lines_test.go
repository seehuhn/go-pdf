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

package fallback

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	gtext "seehuhn.de/go/pdf/graphics/text"
)

// A known string at a known width splits into lines at the word boundaries a
// reader would expect, using the default font and breaker.
func TestStyleLinesWordBreaks(t *testing.T) {
	s := NewStyle()
	F := font.Must(standard.Helvetica.New())
	contents := "a b c"

	// choose a width just wide enough for "a b" but not for "a b c"
	width := F.Layout(nil, freeTextFontSize, "a b").TotalWidth() + 1

	got, err := s.Lines(contents, width)
	if err != nil {
		t.Fatal(err)
	}

	want := []Line{
		{Start: 0, End: 3}, // "a b"
		{Start: 4, End: 5}, // "c"
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Lines() mismatch (-want +got):\n%s", d)
	}
	for _, line := range got {
		t.Logf("line %q", contents[line.Start:line.End])
	}
}

// A custom LineBreaker with a hyphenation point produces a line ending in a
// hyphen, reported through Hyphen.
type fixedHyphenBreaker struct{}

func (fixedHyphenBreaker) Opportunities(s string) []gtext.BreakOpportunity {
	return []gtext.BreakOpportunity{{Start: 5, End: 5, Replace: "-"}}
}

func TestStyleLinesHyphen(t *testing.T) {
	s := &Style{LineBreaker: fixedHyphenBreaker{}}

	// choose a width just wide enough for "photo-"
	F := font.Must(standard.Helvetica.New())
	width := F.Layout(nil, freeTextFontSize, "photo-").TotalWidth() + 1

	got, err := s.Lines("photograph", width)
	if err != nil {
		t.Fatal(err)
	}

	want := []Line{
		{Start: 0, End: 5, Hyphen: true},   // "photo"
		{Start: 5, End: 10, Hyphen: false}, // "graph"
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Lines() mismatch (-want +got):\n%s", d)
	}
}

// Style.Lines uses NewContentFont, not always Helvetica.
func TestStyleLinesUsesNewContentFont(t *testing.T) {
	courier := font.Must(standard.Courier.New())
	s := &Style{
		NewContentFont: func() (font.Layouter, error) { return courier, nil },
	}

	contents := "a b c"
	width := courier.Layout(nil, freeTextFontSize, "a b").TotalWidth() + 1

	got, err := s.Lines(contents, width)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2", len(got))
	}

	// Helvetica is narrower than Courier, so the same width fits more text
	// with the wrong font; check we actually broke using courier's metrics.
	helvetica := font.Must(standard.Helvetica.New())
	if helvetica.Layout(nil, freeTextFontSize, contents).TotalWidth() <= width {
		t.Fatal("test is not discriminating: Helvetica also fits on one line")
	}
}

// A NewContentFont error is reported to the caller.
func TestStyleLinesFontError(t *testing.T) {
	errNoFont := errors.New("no font today")
	s := &Style{
		NewContentFont: func() (font.Layouter, error) { return nil, errNoFont },
	}

	_, err := s.Lines("hello", 100)
	if !errors.Is(err, errNoFont) {
		t.Errorf("error = %v, want the one from NewContentFont", err)
	}
}

// Style.Lines reproduces exactly the line breaks addFreeTextAppearance
// produces for the same style, font, size and breaker: both go through
// gtext.WrapWith with the same parameters, so a wrap built the same way as
// the appearance generator's must yield the same number of lines and the
// same hyphenation as Style.Lines does.
func TestStyleLinesMatchesGenerator(t *testing.T) {
	breaker := fixedHyphenBreaker{}
	s := &Style{LineBreaker: breaker}
	gen, err := s.New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}

	contents := "photograph"
	width := 0.0

	got, err := s.Lines(contents, width)
	if err != nil {
		t.Fatal(err)
	}

	// build the wrap the same way addFreeTextAppearance does
	wrapper := gtext.WrapWith(gen.breaker(), width, contents)
	wantLines := slices.Collect(wrapper.Lines(gen.ContentFont(), freeTextFontSize))

	if len(got) != len(wantLines) {
		t.Fatalf("Style.Lines produced %d lines, Generator's wrap produced %d", len(got), len(wantLines))
	}
	for i, line := range got {
		text := ""
		for _, g := range wantLines[i].Seq {
			text += g.Text
		}
		gotText := contents[line.Start:line.End]
		if line.Hyphen {
			gotText += "-"
		}
		if gotText != text {
			t.Errorf("line %d: Style.Lines gives %q, Generator's wrap draws %q", i, gotText, text)
		}
	}
}
