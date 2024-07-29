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

package font

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/sfnt/os2"
)

func TestRoundTrip(t *testing.T) {
	fd1 := &Descriptor{
		FontName:     "Test Name",
		FontFamily:   "Test Family",
		FontStretch:  os2.WidthCondensed,
		FontWeight:   os2.WeightBold,
		IsFixedPitch: true,
		IsSerif:      true,
		IsSymbolic:   true,
		IsScript:     true,
		IsItalic:     true,
		IsAllCap:     true,
		IsSmallCap:   true,
		ForceBold:    true,
		FontBBox:     &pdf.Rectangle{LLx: 10, LLy: 20, URx: 30, URy: 40},
		ItalicAngle:  50,
		Ascent:       60,
		Descent:      -70,
		Leading:      80,
		CapHeight:    90,
		XHeight:      100,
		StemV:        110,
		StemH:        120,
		MaxWidth:     130,
		AvgWidth:     140,
		MissingWidth: 150,
	}

	data := pdf.NewData(pdf.V2_0)
	fdDict := fd1.AsDict()
	fdRef := data.Alloc()
	err := data.Put(fdRef, fdDict)
	if err != nil {
		t.Fatal(err)
	}

	fd2, err := ExtractDescriptor(data, fdRef)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(fd1, fd2); d != "" {
		t.Errorf("diff: %s", d)
	}
}

func FuzzFontDescriptor(f *testing.F) {
	fd := &Descriptor{}
	data, err := embedFD(fd)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)

	fd = &Descriptor{
		FontName:     "Test Name",
		FontFamily:   "Test Family",
		FontStretch:  os2.WidthCondensed,
		FontWeight:   os2.WeightExtraBold,
		IsFixedPitch: true,
		IsSerif:      true,
		IsSymbolic:   true,
		IsScript:     true,
		IsItalic:     true,
		IsAllCap:     true,
		IsSmallCap:   true,
		ForceBold:    true,
		FontBBox:     &pdf.Rectangle{LLx: 100, LLy: 200, URx: 300, URy: 400},
		ItalicAngle:  -10,
		Ascent:       800,
		Descent:      -200,
		Leading:      1200,
		CapHeight:    600,
		XHeight:      400,
		StemV:        50,
		StemH:        30,
		MaxWidth:     1000,
		AvgWidth:     800,
		MissingWidth: 700,
	}
	data, err = embedFD(fd)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)

	f.Fuzz(func(t *testing.T, data1 []byte) {
		opt := &pdf.ReaderOptions{
			ErrorHandling: pdf.ErrorHandlingReport,
		}
		r, err := pdf.NewReader(bytes.NewReader(data1), opt)
		if err != nil {
			t.Skip()
		}
		fdDict, err := pdf.GetDictTyped(r, pdf.NewReference(1, 0), "FontDescriptor")
		if err != nil {
			t.Skip()
		}
		fd1, err := ExtractDescriptor(r, fdDict)
		if err != nil {
			t.Skip()
		}

		data := pdf.NewData(pdf.V2_0)
		fdDict = fd1.AsDict()
		fdRef := data.Alloc()
		err = data.Put(fdRef, fdDict)
		if err != nil {
			t.Fatal(err)
		}
		fd2, err := ExtractDescriptor(data, fdRef)
		if err != nil {
			t.Fatal(err)
		}

		if d := cmp.Diff(fd1, fd2); d != "" {
			t.Errorf("diff: %s", d)
		}
	})
}

func embedFD(fd *Descriptor) ([]byte, error) {
	buf := &bytes.Buffer{}
	w, err := pdf.NewWriter(buf, pdf.V1_7, nil)
	if err != nil {
		return nil, err
	}

	err = w.Put(pdf.NewReference(1, 0), fd.AsDict())
	if err != nil {
		return nil, err
	}
	// w.GetMeta().Catalog.Pages = pdf.NewReference(2, 0)
	err = w.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
