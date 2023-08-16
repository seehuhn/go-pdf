// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"fmt"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/extract"
	"seehuhn.de/go/pdf/internal/debug"
	"seehuhn.de/go/pdf/pagetree"
)

func TestExtract(t *testing.T) {
	text := `“Hello World!”`

	fonts, err := debug.MakeFonts()
	if err != nil {
		t.Fatal(err)
	}
	for i, font := range fonts {
		t.Run(fmt.Sprintf("%d:%s", i, font.Type), func(t *testing.T) {
			buf := &bytes.Buffer{}
			page, err := document.WriteSinglePage(buf, document.A4, nil)
			if err != nil {
				t.Fatal(err)
			}
			F, err := font.Font.Embed(page.Out, "F")
			if err != nil {
				t.Fatal(err)
			}
			page.TextStart()
			page.TextSetFont(F, 12)
			page.TextFirstLine(72, 72)
			page.TextShow(text)
			page.TextEnd()
			err = page.Close()
			if err != nil {
				t.Fatal(err)
			}

			r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatal(err)
			}

			decoders := map[pdf.Reference]func(pdf.String) string{}
			getDecoder := func(obj pdf.Object) (func(pdf.String) string, error) {
				ref, ok := obj.(pdf.Reference)
				if !ok {
					return nil, fmt.Errorf("expected reference, got %T", obj)
				}
				if f, ok := decoders[ref]; ok {
					return f, nil
				}
				f, err := extract.MakeTextDecoder(r, ref)
				if err != nil {
					return nil, err
				}
				decoders[ref] = f
				return f, nil
			}

			pageDict, err := pagetree.GetPage(r, 0)
			if err != nil {
				t.Fatal(err)
			}
			var fragments []string
			err = ForAllText(r, pageDict, func(ctx *Context, s pdf.String) error {
				ref := ctx.Resources.Font[ctx.State.Font]
				f, err := getDecoder(ref)
				if err != nil {
					return err
				}
				fragments = append(fragments, f(s))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(fragments, "") != text {
				t.Errorf("got %q, want %q", strings.Join(fragments, ""), text)
			}
		})
	}
}
