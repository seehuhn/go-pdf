// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

package color

import (
	"math"
	"testing"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestICCBased(t *testing.T) {
	for _, profile := range [][]byte{icc.SRGBv2Profile, icc.SRGBv4Profile} {
		icc, err := ICCBased(profile, nil)
		if err != nil {
			t.Errorf("ICCBased: %v", err)
			continue
		}

		if icc.N != 3 {
			t.Errorf("expected 3 components, got %d", icc.N)
		}
		if !isValues(icc.Ranges, 0, 1, 0, 1, 0, 1) {
			t.Errorf("invalid ranges: %v", icc.Ranges)
		}
	}
}

func TestICCBasedEmbedVersion(t *testing.T) {
	cases := []struct {
		name    string
		profile []byte
		version pdf.Version
		ok      bool
	}{
		{"v4 in 1.4", icc.SRGBv4Profile, pdf.V1_4, false},
		{"v4 in 1.6", icc.SRGBv4Profile, pdf.V1_6, false},
		{"v4 in 1.7", icc.SRGBv4Profile, pdf.V1_7, true},
		{"v2 in 1.3", icc.SRGBv2Profile, pdf.V1_3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			space, err := ICCBased(c.profile, nil)
			if err != nil {
				t.Fatal(err)
			}
			w, _ := memfile.NewPDFWriter(t, c.version, nil)
			rm := pdf.NewResourceManager(w)
			_, err = rm.Embed(space)
			if (err == nil) != c.ok {
				t.Errorf("unexpected result: %v", err)
			}
		})
	}
}

func TestMinPDFVersion(t *testing.T) {
	cases := []struct {
		v    icc.Version
		want pdf.Version
	}{
		{icc.Version2_1_0, pdf.V1_3},
		{icc.Version2_3_0, pdf.V1_3},
		{icc.Version4_0_0, pdf.V1_5},
		{icc.Version4_1_0, pdf.V1_6},
		{icc.Version4_2_0, pdf.V1_7},
		{icc.Version4_3_0, pdf.V2_0},
		{icc.Version4_4_0, pdf.V2_0},
		{0x0300_0000, pdf.V2_0},
		{0x0500_0000, pdf.V2_0},
	}
	for _, c := range cases {
		if got := minPDFVersion(c.v); got != c.want {
			t.Errorf("%v: got %v, want %v", c.v, got, c.want)
		}
	}
}

func TestROMMPrimaries(t *testing.T) {
	cases := []struct {
		in   []float64
		want [3]float64
	}{
		{[]float64{1, 0, 0}, [3]float64{0.7977, 0.2881, 0.0000}},
		{[]float64{0, 1, 0}, [3]float64{0.1352, 0.7118, 0.0000}},
		{[]float64{0, 0, 1}, [3]float64{0.0314, 0.0001, 0.8249}},
	}
	for _, c := range cases {
		X, Y, Z := FromValues(SpaceROMMRGB, c.in, nil).ToXYZ()
		if !near(X, c.want[0], 2e-3) || !near(Y, c.want[1], 2e-3) || !near(Z, c.want[2], 2e-3) {
			t.Errorf("%v: got (%g,%g,%g), want %v", c.in, X, Y, Z, c.want)
		}
	}
}

func TestROMMWhiteAndGrey(t *testing.T) {
	grey := func(v float64) (X, Y, Z float64) {
		return FromValues(SpaceROMMRGB, []float64{v, v, v}, nil).ToXYZ()
	}
	X, Y, Z := grey(1)
	if !near(X, WhitePointD50[0], 1e-3) || !near(Y, 1, 1e-3) || !near(Z, WhitePointD50[2], 1e-3) {
		t.Errorf("white: got (%g,%g,%g)", X, Y, Z)
	}

	prev := -1.0
	for i := 0; i <= 20; i++ {
		v := float64(i) / 20
		_, Y, _ := grey(v)
		if Y <= prev {
			t.Errorf("grey ramp not monotone at %g", v)
		}
		prev = Y
	}
	_, Y, _ = grey(0.5)
	if want := math.Pow(0.5, 1.8); !near(Y, want, 5e-3) {
		t.Errorf("grey 0.5: got %g, want %g", Y, want)
	}
}

func TestROMMRoundTrip(t *testing.T) {
	ws := &icc.Workspace{}
	for _, v := range [][3]float64{
		{0.2, 0.4, 0.6}, {0.9, 0.1, 0.3}, {0.5, 0.5, 0.5}, {0.1, 0.8, 0.2}, {0.7, 0.7, 0.1},
	} {
		X, Y, Z := SpaceROMMRGB.ToXYZ(v[:], icc.RelativeColorimetric, ws)
		var got [3]float64
		SpaceROMMRGB.FromXYZ(X, Y, Z, icc.RelativeColorimetric, got[:], ws)
		for i := range got {
			if !near(got[i], v[i], 1e-3) {
				t.Errorf("%v: got %v", v, got)
				break
			}
		}
	}
}

func TestROMMEmbed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)
	obj, err := rm.Embed(SpaceROMMRGB)
	if err != nil {
		t.Fatal(err)
	}
	x := pdf.NewExtractor(w)
	space, err := ExtractSpace(pdf.CursorAt(x, nil), obj, false)
	if err != nil {
		t.Fatal(err)
	}
	if !SpacesEqual(space, SpaceROMMRGB) {
		t.Errorf("round trip changed the space: %v", space)
	}

	// the version 4 profile is not allowed before PDF 1.7
	w, _ = memfile.NewPDFWriter(t, pdf.V1_6, nil)
	rm = pdf.NewResourceManager(w)
	if _, err := rm.Embed(SpaceROMMRGB); err == nil {
		t.Error("expected error for PDF 1.6")
	}
}
