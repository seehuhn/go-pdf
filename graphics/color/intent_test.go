// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026 Jochen Voss <voss@seehuhn.de>
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
	"sync"
	"testing"

	"seehuhn.de/go/icc"
	"seehuhn.de/go/icc/icctest"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

var allIntents = []icc.RenderingIntent{
	icc.Perceptual,
	icc.RelativeColorimetric,
	icc.Saturation,
	icc.AbsoluteColorimetric,
}

func intentGraySpace(t *testing.T) *SpaceICCBased {
	t.Helper()
	data, err := icctest.IntentProfile(icc.Version4_3_0, icc.OutputDeviceProfile).Encode()
	if err != nil {
		t.Fatal(err)
	}
	s, err := ICCBased(data, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func xyzDiffers(x1, y1, z1, x2, y2, z2 float64) bool {
	return math.Abs(x1-x2)+math.Abs(y1-y2)+math.Abs(z1-z2) > 1e-3
}

func TestICCBasedIntent(t *testing.T) {
	s := intentGraySpace(t)
	ws := &icc.Workspace{}
	v := []float64{0.5}

	px, py, pz := s.ToXYZ(v, icc.Perceptual, ws)
	rx, ry, rz := s.ToXYZ(v, icc.RelativeColorimetric, ws)
	sx, sy, sz := s.ToXYZ(v, icc.Saturation, ws)
	if !xyzDiffers(px, py, pz, rx, ry, rz) {
		t.Error("perceptual and relative give the same XYZ")
	}
	if !xyzDiffers(sx, sy, sz, rx, ry, rz) {
		t.Error("saturation and relative give the same XYZ")
	}
	if !xyzDiffers(sx, sy, sz, px, py, pz) {
		t.Error("saturation and perceptual give the same XYZ")
	}

	// the colour method uses the relative intent
	cx, cy, cz := grayColor(s, 0.5).ToXYZ()
	if xyzDiffers(cx, cy, cz, rx, ry, rz) {
		t.Error("Color.ToXYZ is not relative colorimetric")
	}

	// FromXYZ honours the intent: the relative XYZ means another gray value
	// under the perceptual tables
	var gp, gr [1]float64
	s.FromXYZ(rx, ry, rz, icc.Perceptual, gp[:], ws)
	s.FromXYZ(rx, ry, rz, icc.RelativeColorimetric, gr[:], ws)
	if math.Abs(gr[0]-0.5) > 0.01 {
		t.Errorf("relative round trip: got %g, want 0.5", gr[0])
	}
	if math.Abs(gp[0]-gr[0]) < 0.01 {
		t.Error("FromXYZ ignores the intent")
	}
}

func grayColor(s *SpaceICCBased, v float64) Color {
	c, err := s.New([]float64{v})
	if err != nil {
		panic(err)
	}
	return c
}

func TestSpecialSpacesIntent(t *testing.T) {
	icsp := intentGraySpace(t)
	ws := &icc.Workspace{}

	idx, err := Indexed([]Color{grayColor(icsp, 0.2), grayColor(icsp, 0.5)})
	if err != nil {
		t.Fatal(err)
	}
	fn := &function.Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1}
	sep, err := Separation("spot", icsp, fn)
	if err != nil {
		t.Fatal(err)
	}
	dn, err := DeviceN([]pdf.Name{"ink"}, icsp, fn, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		space Space
		in    []float64
		base  float64
	}{
		{"indexed", idx, []float64{1}, 128.0 / 255},
		{"separation", sep, []float64{0.5}, 0.5},
		{"deviceN", dn, []float64{0.5}, 0.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, ri := range allIntents {
				wx, wy, wz := icsp.ToXYZ([]float64{tc.base}, ri, ws)
				gx, gy, gz := tc.space.ToXYZ(tc.in, ri, ws)
				if xyzDiffers(gx, gy, gz, wx, wy, wz) {
					t.Errorf("%v: got %g %g %g, want %g %g %g",
						ri, gx, gy, gz, wx, wy, wz)
				}
			}
			px, py, pz := tc.space.ToXYZ(tc.in, icc.Perceptual, ws)
			rx, ry, rz := tc.space.ToXYZ(tc.in, icc.RelativeColorimetric, ws)
			if !xyzDiffers(px, py, pz, rx, ry, rz) {
				t.Error("intent has no effect")
			}
		})
	}
}

func TestSourceBlackPointForwarding(t *testing.T) {
	icsp := intentGraySpace(t)
	fn := &function.Type2{XMin: 0, XMax: 1, C0: []float64{0}, C1: []float64{1}, N: 1}
	sep, err := Separation("spot", icsp, fn)
	if err != nil {
		t.Fatal(err)
	}
	dn, err := DeviceN([]pdf.Name{"ink"}, icsp, fn, nil)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Indexed([]Color{grayColor(icsp, 0.2)})
	if err != nil {
		t.Fatal(err)
	}
	unc := SpacePatternUncolored{Base: icsp}

	for _, ri := range allIntents {
		want := icsp.SourceBlackPoint(ri)
		for name, s := range map[string]Space{
			"separation": sep, "deviceN": dn, "indexed": idx, "pattern": unc,
		} {
			if got := s.SourceBlackPoint(ri); got != want {
				t.Errorf("%s %v: got %v, want %v", name, ri, got, want)
			}
		}
	}

	// the intent profile has a lifted black under the perceptual tables
	if icsp.SourceBlackPoint(icc.Perceptual) == icsp.SourceBlackPoint(icc.RelativeColorimetric) {
		t.Error("source black point ignores the intent")
	}

	for _, s := range []Space{SpaceDeviceGray, SpaceDeviceRGB, SpaceSRGB, SpacePatternColored} {
		if bp := s.SourceBlackPoint(icc.Perceptual); bp != [3]float64{} {
			t.Errorf("%s: black point %v, want zero", s.Family(), bp)
		}
	}
	for _, s := range []BidirectionalSpace{SpaceDeviceGray, SpaceDeviceRGB, SpaceSRGB} {
		if bp := s.DestinationBlackPoint(icc.Perceptual); bp != [3]float64{} {
			t.Errorf("%T: destination black point %v, want zero", s, bp)
		}
	}
}

func TestDeviceCMYKBlackPoint(t *testing.T) {
	for _, ri := range allIntents {
		xf := cmykXform(ri)
		if got := SpaceDeviceCMYK.SourceBlackPoint(ri); got != xf.SourceBlackPoint() {
			t.Errorf("%v: source black point %v, want %v", ri, got, xf.SourceBlackPoint())
		}
		if got := SpaceDeviceCMYK.DestinationBlackPoint(ri); got != xf.DestinationBlackPoint() {
			t.Errorf("%v: destination black point %v, want %v", ri, got, xf.DestinationBlackPoint())
		}
	}
	if cmykXform(icc.RenderingIntent(99)) != cmykXform(icc.RelativeColorimetric) {
		t.Error("out-of-range intent is not relative colorimetric")
	}
}

func TestICCBasedTransformCache(t *testing.T) {
	s := intentGraySpace(t)
	for _, ri := range allIntents {
		t1 := s.Transform(ri)
		if t1 == nil {
			t.Fatalf("%v: no transform", ri)
		}
		if t1.Intent != ri {
			t.Errorf("%v: transform has intent %v", ri, t1.Intent)
		}
		if s.Transform(ri) != t1 {
			t.Errorf("%v: transform not cached", ri)
		}
	}
	if s.Transform(icc.RenderingIntent(99)) != s.Transform(icc.RelativeColorimetric) {
		t.Error("out-of-range intent is not relative colorimetric")
	}
}

func TestICCBasedTransformConcurrent(t *testing.T) {
	s := intentGraySpace(t)
	var wg sync.WaitGroup
	res := make([][]*icc.Transform, 8)
	for g := range res {
		res[g] = make([]*icc.Transform, len(allIntents))
		wg.Go(func() {
			ws := &icc.Workspace{}
			for i, ri := range allIntents {
				res[g][i] = s.Transform(ri)
				s.ToXYZ([]float64{0.5}, ri, ws)
				s.SourceBlackPoint(ri)
			}
		})
	}
	wg.Wait()
	for g := range res {
		for i := range allIntents {
			if res[g][i] != res[0][i] || res[g][i] == nil {
				t.Errorf("goroutine %d intent %d: inconsistent transform", g, i)
			}
		}
	}
}

func TestCIEAbsoluteIntent(t *testing.T) {
	d65 := []float64{0.9505, 1, 1.089}
	d50 := icc.PCSWhitePoint[:]
	matrix := []float64{0.4124, 0.2126, 0.0193, 0.3576, 0.7152, 0.1192, 0.1805, 0.0722, 0.9505}

	for _, tc := range []struct {
		name      string
		white     []float64
		wantEqual bool
	}{{"D65", d65, false}, {"D50", d50, true}} {
		t.Run(tc.name, func(t *testing.T) {
			calGray, err := CalGray(tc.white, nil, 1)
			if err != nil {
				t.Fatal(err)
			}
			calRGB, err := CalRGB(tc.white, nil, []float64{1, 1, 1}, matrix)
			if err != nil {
				t.Fatal(err)
			}
			lab, err := Lab(tc.white, nil, []float64{-100, 100, -100, 100})
			if err != nil {
				t.Fatal(err)
			}
			ws := &icc.Workspace{}
			for _, tt := range []struct {
				s  Space
				in []float64
			}{
				{calGray, []float64{0.6}},
				{calRGB, []float64{0.3, 0.5, 0.7}},
				{lab, []float64{60, 20, -30}},
			} {
				ax, ay, az := tt.s.ToXYZ(tt.in, icc.AbsoluteColorimetric, ws)
				rx, ry, rz := tt.s.ToXYZ(tt.in, icc.RelativeColorimetric, ws)
				if differs := xyzDiffers(ax, ay, az, rx, ry, rz); differs == tc.wantEqual {
					t.Errorf("%s: absolute %g %g %g, relative %g %g %g",
						tt.s.Family(), ax, ay, az, rx, ry, rz)
				}

				// each direction round-trips under every intent
				for _, ri := range allIntents {
					x, y, z := tt.s.ToXYZ(tt.in, ri, ws)
					got := make([]float64, len(tt.in))
					tt.s.(BidirectionalSpace).FromXYZ(x, y, z, ri, got, ws)
					for i := range got {
						if math.Abs(got[i]-tt.in[i]) > 1e-4*(1+math.Abs(tt.in[i])) {
							t.Errorf("%s %v: round trip %v -> %v", tt.s.Family(), ri, tt.in, got)
							break
						}
					}
				}
			}
		})
	}
}

func TestCIEBlackPoint(t *testing.T) {
	matrix := []float64{0.4124, 0.2126, 0.0193, 0.3576, 0.7152, 0.1192, 0.1805, 0.0722, 0.9505}
	d65 := []float64{0.9505, 1, 1.089}
	d50 := []float64{icc.PCSWhitePoint[0], icc.PCSWhitePoint[1], icc.PCSWhitePoint[2]}

	for _, tc := range []struct {
		name  string
		white []float64
	}{{"D65", d65}, {"D50", d50}} {
		// a black point proportional to the white point is mapped to the
		// same multiple of the PCS white point
		bp := []float64{0.05 * tc.white[0], 0.05 * tc.white[1], 0.05 * tc.white[2]}
		var want [3]float64
		for k := range want {
			want[k] = 0.05 * icc.PCSWhitePoint[k]
		}

		calRGB, err := CalRGB(tc.white, bp, []float64{1, 1, 1}, matrix)
		if err != nil {
			t.Fatal(err)
		}
		calGray, err := CalGray(tc.white, bp, 1)
		if err != nil {
			t.Fatal(err)
		}
		lab, err := Lab(tc.white, bp, []float64{-100, 100, -100, 100})
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range []BidirectionalSpace{calRGB, calGray, lab} {
			for _, ri := range allIntents {
				for _, got := range [][3]float64{s.SourceBlackPoint(ri), s.DestinationBlackPoint(ri)} {
					for k := range want {
						if math.Abs(got[k]-want[k]) > 1e-6 {
							t.Errorf("%s %s %v: got %v, want %v", tc.name, s.Family(), ri, got, want)
							break
						}
					}
				}
			}
		}

		if tc.name == "D50" {
			if got := calGray.SourceBlackPoint(icc.RelativeColorimetric); got != [3]float64(bp) {
				t.Errorf("D50 black point changed: got %v, want %v", got, bp)
			}
		}
	}
}

// TestICCBasedUnusableProfile checks that a space whose profile yields no
// transform has no black points and still converts via the fallback.
func TestICCBasedUnusableProfile(t *testing.T) {
	s := &SpaceICCBased{N: 3, Ranges: []float64{0, 1, 0, 1, 0, 1}, Profile: &icc.Profile{}}
	if s.Transform(icc.RelativeColorimetric) != nil {
		t.Fatal("expected no transform")
	}
	for _, ri := range allIntents {
		if bp := s.SourceBlackPoint(ri); bp != ([3]float64{}) {
			t.Errorf("source black point %v: got %v", ri, bp)
		}
		if bp := s.DestinationBlackPoint(ri); bp != ([3]float64{}) {
			t.Errorf("destination black point %v: got %v", ri, bp)
		}
	}

	ws := &icc.Workspace{}
	X, Y, Z := s.ToXYZ([]float64{0.2, 0.4, 0.6}, icc.RelativeColorimetric, ws)
	wantX, wantY, wantZ := srgbToXYZ(0.2, 0.4, 0.6)
	if X != wantX || Y != wantY || Z != wantZ {
		t.Errorf("ToXYZ: got %g %g %g, want %g %g %g", X, Y, Z, wantX, wantY, wantZ)
	}
	got := make([]float64, 3)
	s.FromXYZ(X, Y, Z, icc.RelativeColorimetric, got, ws)
	for i, v := range []float64{0.2, 0.4, 0.6} {
		if math.Abs(got[i]-v) > 1e-6 {
			t.Errorf("FromXYZ component %d: got %g, want %g", i, got[i], v)
		}
	}
}
