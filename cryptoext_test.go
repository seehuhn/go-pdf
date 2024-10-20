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
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
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
