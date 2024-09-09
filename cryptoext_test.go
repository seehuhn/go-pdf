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

package pdf_test

import (
	"bytes"
	"iter"
	"os"
	"slices"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/internal/debug/tempfile"
	"seehuhn.de/go/pdf/pdfcopy"
)

type testFileSamples struct {
	Passwd string
	Err    error
}

func (s *testFileSamples) All() iter.Seq[[]byte] {
	paper := &pdf.Rectangle{
		URx: 100,
		URy: 100,
	}

	buf := &bytes.Buffer{}

	return func(yield func([]byte) bool) {
		if s.Err != nil {
			return
		}

		for _, v := range []pdf.Version{pdf.V1_1, pdf.V1_2, pdf.V1_3, pdf.V1_4, pdf.V1_5, pdf.V1_6, pdf.V1_7, pdf.V2_0} {
			opt := &pdf.WriterOptions{
				UserPassword:    s.Passwd,
				UserPermissions: pdf.PermPrintDegraded,
				HumanReadable:   true,
			}

			// minimal PDF file
			buf.Reset()
			w, err := pdf.NewWriter(buf, v, opt)
			if err != nil {
				s.Err = err
				break
			}
			w.GetMeta().Info.Title = "a string to encrypt"
			w.GetMeta().Catalog.Pages = w.Alloc() // pretend we have a page tree
			err = w.Close()
			if err != nil {
				s.Err = err
				break
			}
			cont := yield(slices.Clone(buf.Bytes()))
			if !cont {
				break
			}

			// minimal working PDF file
			buf.Reset()
			page, err := document.WriteSinglePage(buf, paper, v, opt)
			if err != nil {
				s.Err = err
				break
			}
			page.MoveTo(0, 0)
			page.LineTo(100, 100)
			page.Stroke()
			page.Out.GetMeta().Info.Title = "a string to encrypt"
			err = page.Close()
			if err != nil {
				s.Err = err
				break
			}

			cont = yield(slices.Clone(buf.Bytes()))
			if !cont {
				break
			}
		}
	}
}

func FuzzEncrypted(f *testing.F) {
	passwd := "secret"

	s := &testFileSamples{
		Passwd: passwd,
	}
	for body := range s.All() {
		f.Add(body)
	}
	if s.Err != nil {
		f.Fatal(s.Err)
	}

	ropt := &pdf.ReaderOptions{
		ReadPassword: func(ID []byte, try int) string {
			if try < 3 {
				return passwd
			}
			return ""
		},
		ErrorHandling: pdf.ErrorHandlingReport,
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		r1, err := pdf.NewReader(bytes.NewReader(raw), ropt)
		if err != nil {
			return
		}
		w1, tmpFile1 := tempfile.NewTempWriter(pdf.GetVersion(r1), nil)
		trans := pdfcopy.NewCopier(w1, r1)
		newCatalog, err := pdfcopy.CopyStruct(trans, r1.GetMeta().Catalog)
		if err != nil {
			return
		}
		w1.GetMeta().Catalog = newCatalog
		newInfo, err := pdfcopy.CopyStruct(trans, r1.GetMeta().Info)
		if err != nil {
			return
		}
		w1.GetMeta().Info = newInfo
		w1.GetMeta().ID = r1.GetMeta().ID
		err = w1.Close()
		if err != nil {
			t.Fatal(err)
		}

		tmpFile1.Offset = 0
		r2, err := pdf.NewReader(tmpFile1, nil)
		if err != nil {
			t.Fatal(err)
		}
		w2, tmpFile2 := tempfile.NewTempWriter(pdf.GetVersion(r2), nil)
		trans = pdfcopy.NewCopier(w2, r2)
		newCatalog, err = pdfcopy.CopyStruct(trans, r2.GetMeta().Catalog)
		if err != nil {
			return
		}
		w2.GetMeta().Catalog = newCatalog
		newInfo, err = pdfcopy.CopyStruct(trans, r2.GetMeta().Info)
		if err != nil {
			t.Fatal(err)
		}
		w2.GetMeta().Info = newInfo
		w2.GetMeta().ID = r2.GetMeta().ID
		err = w2.Close()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(tmpFile1.Data, tmpFile2.Data) {
			os.WriteFile("b.pdf", tmpFile1.Data, 0644)
			os.WriteFile("c.pdf", tmpFile2.Data, 0644)
			t.Fatalf("pdf contents differ")
		}
	})
}
