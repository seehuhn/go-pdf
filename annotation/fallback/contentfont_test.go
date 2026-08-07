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
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/acroform"
	"seehuhn.de/go/pdf/annotation"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/form"
)

// fontsUsed returns the fonts the content stream of f selects, in the order it
// selects them.
func fontsUsed(t *testing.T, f *form.Form) []font.Instance {
	t.Helper()

	var out []font.Instance
	iter := f.Content.NewIter()
	for op, args := range iter.All() {
		if op != content.OpTextSetFont || len(args) < 2 {
			continue
		}
		name, ok := args[0].(pdf.Name)
		if !ok {
			continue
		}
		if F, ok := f.Res.Font[name]; ok {
			out = append(out, F)
		}
	}
	if err := iter.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// textFieldWidget returns a widget for a text field holding the given value.
func textFieldWidget(t *testing.T, value string) *annotation.Widget {
	t.Helper()
	f := acroform.NewTextField("t")
	f.V = &pdf.StringOrStream{Value: value}
	return annotation.AddWidget(f, pdf.Rectangle{LLx: 0, LLy: 0, URx: 120, URy: 20})
}

// The font a Style chooses is the one which draws annotation text, rather than
// merely being recorded somewhere.
func TestContentFontDrawsTheText(t *testing.T) {
	courier := font.Must(standard.Courier.New())
	s := &Style{
		NewContentFont: func() (font.Layouter, error) { return courier, nil },
	}
	gen, err := s.New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}

	w := textFieldWidget(t, "hello")
	if err := gen.AddAppearance(w); err != nil {
		t.Fatal(err)
	}

	used := fontsUsed(t, w.Appearance.Normal)
	if len(used) != 1 {
		t.Fatalf("the appearance selects %d fonts, want 1", len(used))
	}
	if used[0] != courier {
		t.Error("the appearance was drawn with a font other than the chosen one")
	}
}

// A caller which names the generator's font elsewhere in its document, for
// example in a default appearance string, must get the same instance the
// appearance streams use, or the document ends up with two copies of the font.
func TestContentFontIsTheOneReported(t *testing.T) {
	gen, err := NewStyle().New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}

	w := textFieldWidget(t, "hello")
	if err := gen.AddAppearance(w); err != nil {
		t.Fatal(err)
	}

	used := fontsUsed(t, w.Appearance.Normal)
	if len(used) != 1 {
		t.Fatalf("the appearance selects %d fonts, want 1", len(used))
	}
	if used[0] != gen.ContentFont() {
		t.Error("the appearance used a font other than the reported ContentFont")
	}
}

// The default content font is made only once an appearance needs it.  Most
// documents have no annotation which draws text, and reading a font program
// costs more than building the appearance streams that use it.
func TestContentFontMadeOnFirstUse(t *testing.T) {
	gen, err := NewStyle().New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}
	if gen.contentFont != nil {
		t.Error("the font was made before any appearance needed it")
	}

	// an annotation which draws no text leaves the font unmade
	square := &annotation.Square{
		Common: annotation.Common{Rect: pdf.Rectangle{URx: 20, URy: 20}},
	}
	if err := gen.AddAppearance(square); err != nil {
		t.Fatal(err)
	}
	if gen.contentFont != nil {
		t.Error("an annotation with no text made the font")
	}

	if err := gen.AddAppearance(textFieldWidget(t, "hello")); err != nil {
		t.Fatal(err)
	}
	if gen.contentFont == nil {
		t.Error("an annotation with text did not make the font")
	}
}

// A Style may be used for any number of documents, so it must hand each
// generator a font of its own: sharing one would mix the character codes of
// separate documents into a single encoding.
func TestContentFontMadeOncePerGenerator(t *testing.T) {
	calls := 0
	s := &Style{
		NewContentFont: func() (font.Layouter, error) {
			calls++
			return standard.Helvetica.New()
		},
	}

	first, err := s.New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}
	// several appearances must not each ask for a font
	for range 3 {
		if err := first.AddAppearance(textFieldWidget(t, "hello")); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("one generator asked for %d fonts, want 1", calls)
	}

	second, err := s.New(pdf.V2_0)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Errorf("two generators asked for %d fonts, want 2", calls)
	}
	if first.ContentFont() == second.ContentFont() {
		t.Error("two generators share one font instance")
	}
}

// A font which cannot be made is reported to the caller rather than left to
// surface later as a missing or wrong appearance.
func TestContentFontError(t *testing.T) {
	errNoFont := errors.New("no font today")
	s := &Style{
		NewContentFont: func() (font.Layouter, error) { return nil, errNoFont },
	}

	gen, err := s.New(pdf.V2_0)
	if !errors.Is(err, errNoFont) {
		t.Errorf("error = %v, want the one from NewContentFont", err)
	}
	if gen != nil {
		t.Error("a generator was returned along with the error")
	}
}
