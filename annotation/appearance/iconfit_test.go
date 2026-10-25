// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package appearance

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

var iconFitCases = []*IconFit{
	{},
	{
		ScaleWhen:     IconScaleWhenBigger,
		Scaling:       IconScalingProportional,
		LeftoverSpace: &[2]float64{0.25, 0.75},
		FitToBounds:   true,
	},
	{ScaleWhen: IconScaleNever},
	{Scaling: IconScalingAnamorphic, SingleUse: true},
	{LeftoverSpace: &[2]float64{0, 1}},
	{FitToBounds: true, SingleUse: true},
}

func roundTripIconFit(t *testing.T, version pdf.Version, data *IconFit) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(t, version, nil)
	rm := pdf.NewResourceManager(w)
	ref, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed failed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close failed: %v", err)
	}

	x := pdf.NewExtractor(w)
	decoded, err := pdf.CursorAt(x, nil).Decode(ref, ExtractIconFit)
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}

	if diff := cmp.Diff(data, decoded); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestIconFitRoundTrip(t *testing.T) {
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, data := range iconFitCases {
			t.Run(fmt.Sprintf("%d/%s", i, version), func(t *testing.T) {
				roundTripIconFit(t, version, data)
			})
		}
	}
}

func FuzzIconFitRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, data := range iconFitCases {
			w, buf := memfile.NewPDFWriter(f, version, opt)
			rm := pdf.NewResourceManager(w)
			ref, err := rm.Embed(data)
			if err != nil {
				continue
			}
			if rm.Close() != nil {
				continue
			}
			w.GetMeta().Trailer["Quir:E"] = ref
			if w.Close() != nil {
				continue
			}
			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		objPDF := r.GetMeta().Trailer["Quir:E"]
		if objPDF == nil {
			t.Skip("missing PDF object")
		}

		x := pdf.NewExtractor(r)
		objGo, _ := pdf.CursorAt(x, nil).Decode(objPDF, ExtractIconFit)
		if objGo == nil {
			t.Skip("no icon fit dictionary")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		roundTripIconFit(t, version, objGo)
	})
}

// TestIconFitInvalidEntries checks that a dangling reference, null and a
// value of the wrong type are read like an absent entry.
func TestIconFitInvalidEntries(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	decode := func(dict pdf.Dict) *IconFit {
		t.Helper()
		fit, err := ExtractIconFit(c, dict, true)
		if err != nil {
			t.Fatal(err)
		}
		return fit
	}
	absent := decode(pdf.Dict{})

	if !decode(pdf.Dict{"FB": pdf.Boolean(true)}).FitToBounds {
		t.Error("FB true has no effect")
	}

	invalid := map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"null":     nil,
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(1),
	}
	for name, v := range invalid {
		if diff := cmp.Diff(absent, decode(pdf.Dict{"FB": v})); diff != "" {
			t.Errorf("FB %s differs from absent entry (-absent +got):\n%s", name, diff)
		}
	}

	// A: a component which is not a number or is out of range drops the entry
	for name, a := range map[string]pdf.Array{
		"non-numeric":  {pdf.Real(0.5), pdf.Name("x")},
		"out of range": {pdf.Real(0.5), pdf.Integer(2)},
		"short":        {pdf.Real(0.5)},
	} {
		if diff := cmp.Diff(absent, decode(pdf.Dict{"A": a})); diff != "" {
			t.Errorf("A %s differs from absent entry (-absent +got):\n%s", name, diff)
		}
	}
}

// TestIconFitReadErrors checks that read errors in the A entry are returned.
func TestIconFitReadErrors(t *testing.T) {
	errTest := errors.New("test read error")
	failRef := pdf.NewReference(7, 0)
	c := pdf.NewCursor(failGetter{Getter: mock.Getter, fail: failRef, err: errTest})
	for name, a := range map[string]pdf.Object{
		"array":   failRef,
		"element": pdf.Array{pdf.Real(0.5), failRef},
	} {
		_, err := ExtractIconFit(c, pdf.Dict{"A": a}, true)
		if !errors.Is(err, errTest) {
			t.Errorf("%s: got %v, want the read error", name, err)
		}
	}
}

// failGetter is a pdf.Getter which fails for one reference with an error
// that is not a malformed-file error.
type failGetter struct {
	pdf.Getter
	fail pdf.Reference
	err  error
}

func (g failGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.fail {
		return nil, g.err
	}
	return g.Getter.Get(ref, canObjStm)
}
