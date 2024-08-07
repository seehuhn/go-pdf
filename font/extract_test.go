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

package font_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	sfntcff "seehuhn.de/go/sfnt/cff"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cff"
	"seehuhn.de/go/pdf/font/opentype"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/pdf/font/type3"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/pagetree"
)

// TestExtract makes sure that information about all PDF font types
// can be extracted.
func TestExtract(t *testing.T) {
	for _, sample := range fonttypes.All {
		t.Run(sample.Label, func(t *testing.T) {
			buf := &bytes.Buffer{}
			page, err := document.WriteSinglePage(buf, document.A4, pdf.V1_7, nil)
			if err != nil {
				t.Fatal(err)
			}

			F := sample.MakeFont(page.RM)

			page.TextBegin()
			page.TextSetFont(F, 12)
			page.TextFirstLine(100, 100)
			page.TextShow("Hello World!")
			page.TextEnd()

			fontRef, _, err := pdf.ResourceManagerEmbed(page.RM, F)
			if err != nil {
				t.Fatal(err)
			}

			err = page.Close()
			if err != nil {
				t.Fatal(err)
			}

			r, err := pdf.NewReader(bytes.NewReader(buf.Bytes()), nil)
			if err != nil {
				t.Fatal(err)
			}
			_, err = font.ExtractDicts(r, fontRef)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func FuzzExtract(f *testing.F) {
	for _, fontInfo := range fonttypes.All {
		buf := &bytes.Buffer{}
		w, err := document.WriteSinglePage(buf, document.A4, pdf.V1_7, nil)
		if err != nil {
			f.Fatal(err)
		}

		F := fontInfo.MakeFont(w.RM)

		w.SetFontNameInternal(F, "X")

		w.TextBegin()
		w.TextSetFont(F, 12)
		w.TextFirstLine(100, 100)
		w.TextShow("X")
		w.TextEnd()
		err = w.Close()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(buf.Bytes())
	}
	f.Fuzz(func(t *testing.T, pdfData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(pdfData), nil)
		if err != nil {
			t.Skip("invalid PDF file")
		}
		info, err := extractX(r)
		if err != nil {
			t.Skip("font X not found")
		}

		data := pdf.NewData(pdf.V1_7)
		ref := data.Alloc()
		err = info.Embed(data, ref)
		if err != nil {
			t.Fatal(err)
		}

		// fmt.Println("writing out.pdf")
		// fd, err := os.Create("out.pdf")
		// if err != nil {
		// 	t.Fatal(err)
		// }
		// err = data.Write(fd)
		// if err != nil {
		// 	t.Fatal(err)
		// }
		// err = fd.Close()
		// if err != nil {
		// 	t.Fatal(err)
		// }

		info2, err := extractFont(data, ref)
		if err != nil {
			t.Fatal(err)
		}
		cmpFDSelectFn := cmp.Comparer(func(fn1, fn2 sfntcff.FDSelectFn) bool {
			return true
		})
		if d := cmp.Diff(info, info2, cmpFDSelectFn); d != "" {
			t.Fatal(d)
		}
	})
}

func extractX(r pdf.Getter) (Dict, error) {
	_, page, err := pagetree.GetPage(r, 0)
	if err != nil {
		return nil, err
	}
	fontDict, err := getResource(r, page["Resources"], "Font", "X")
	if err != nil {
		return nil, err
	}
	return extractFont(r, fontDict)
}

func extractFont(r pdf.Getter, fontDict pdf.Object) (Dict, error) {
	dicts, err := font.ExtractDicts(r, fontDict)
	if err != nil {
		return nil, err
	}
	switch dicts.Type {
	case font.CFFComposite:
		return cff.ExtractComposite(r, dicts)
	case font.CFFSimple:
		return cff.ExtractSimple(r, dicts)
	case font.MMType1:
		return nil, errors.New("MMType1 not supported")
	case font.OpenTypeCFFComposite:
		return opentype.ExtractCFFComposite(r, dicts)
	case font.OpenTypeCFFSimple:
		return opentype.ExtractCFFSimple(r, dicts)
	case font.OpenTypeGlyfComposite:
		return opentype.ExtractGlyfComposite(r, dicts)
	case font.OpenTypeGlyfSimple:
		return opentype.ExtractGlyfSimple(r, dicts)
	case font.TrueTypeComposite:
		return truetype.ExtractComposite(r, dicts)
	case font.TrueTypeSimple:
		return truetype.ExtractSimple(r, dicts)
	case font.Type1:
		return type1.Extract(r, dicts)
	case font.Type3:
		return type3.Extract(r, dicts)
	default:
		panic("unreachable") // unknown font type
	}
}

func getResource(r pdf.Getter, resources pdf.Object, class, name pdf.Name) (pdf.Object, error) {
	resDict, err := pdf.GetDict(r, resources)
	if err != nil {
		return nil, err
	}
	res, err := pdf.GetDict(r, resDict[class])
	if err != nil {
		return nil, err
	}
	return res[name], nil
}
