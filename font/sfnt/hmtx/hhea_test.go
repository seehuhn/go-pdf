package hmtx

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestRoundtrip(t *testing.T) {
	i1 := &Info{
		Widths: []uint16{100, 200, 300, 300},
		GlyphExtent: []font.Rect{
			{LLx: 10, LLy: 0, URx: 90, URy: 100},
			{LLx: 20, LLy: 0, URx: 200, URy: 100},
			{LLx: 30, LLy: 0, URx: 300, URy: 100},
			{LLx: 40, LLy: 0, URx: 300, URy: 100},
		},
		Ascent:      100,
		Descent:     -100,
		LineGap:     120,
		CaretAngle:  math.Pi / 180 * 10,
		CaretOffset: 2,
	}
	hhea, hmtx := EncodeHmtx(i1)
	i2, err := DecodeHmtx(hhea, hmtx)
	if err != nil {
		t.Errorf("error decoding hmtx: %v", err)
	}
	if !reflect.DeepEqual(i1.Widths, i2.Widths) {
		t.Errorf("widths differ: %d vs %d", i1.Widths, i2.Widths)
	}
	if i1.Ascent != i2.Ascent {
		t.Errorf("ascent differs: %d vs %d", i1.Ascent, i2.Ascent)
	}
	if i1.Descent != i2.Descent {
		t.Errorf("descent differs: %d vs %d", i1.Descent, i2.Descent)
	}
	if i1.LineGap != i2.LineGap {
		t.Errorf("line gap differs: %d vs %d", i1.LineGap, i2.LineGap)
	}
	if math.Abs(i1.CaretAngle-i2.CaretAngle) > 1e-4 {
		t.Errorf("caret angle differs: %g vs %g", i1.CaretAngle, i2.CaretAngle)
	}
	if i1.CaretOffset != i2.CaretOffset {
		t.Errorf("caret offset differs: %d vs %d", i1.CaretOffset, i2.CaretOffset)
	}
}

func TestLengths(t *testing.T) {
	info := &Info{
		Widths: []uint16{100, 200, 300, 300, 300},
		GlyphExtent: []font.Rect{
			{0, 0, 100, 100},
			{10, 0, 100, 100},
			{20, 0, 100, 100},
			{30, 0, 100, 100},
			{40, 0, 100, 100},
		},
		Ascent:      0,
		Descent:     0,
		LineGap:     0,
		CaretAngle:  0,
		CaretOffset: 0,
	}
	hhea, hmtx := EncodeHmtx(info)

	if len(hhea) != hheaLength {
		t.Errorf("expected %d, got %d", hheaLength, len(hhea))
	}

	numGlyphs := len(info.Widths)
	numWidths := 3
	hmtxLength := 4*numWidths + 2*(numGlyphs-numWidths)
	if len(hmtx) != hmtxLength {
		t.Errorf("expected %d, got %d", hmtxLength, len(hhea))
	}
}

func TestRationalApproximation(t *testing.T) {
	a, b := bestRationalApproximation(math.Pi, 10000)
	if a != 355 || b != 113 {
		t.Errorf("approximation for π not found: a=%d, b=%d", a, b)
	}

	for _, x := range []float64{1, 0, -1, 3.0 / 2.0, -math.Pi, math.Sqrt2, math.E, -22.0 / 7.0} {
		for _, N := range []int{10, 100, 256, 512, 1000, 1024, 65535} {
			a, b := bestRationalApproximation(x, N)
			// fmt.Printf("%g ≈ %d/%d (N=%d)\n", x, a, b, N)
			if a > N || a < -N || b < 1 || b > N || (a == 0 && b == 0) {
				t.Errorf("%g ≈ %d/%d is out of range", x, a, b)
			}

			q := float64(a) / float64(b)
			qNaive := math.Round(x*float64(b)) / float64(b)
			if math.Abs(x-q) > math.Abs(x-qNaive) {
				t.Errorf("%g ≈ %d/%d (N=%d) is not a good approximation",
					x, a, b, N)
			}
		}
	}
}

func FuzzBestRational1(f *testing.F) {
	f.Fuzz(func(t *testing.T, x float64, N0 uint16) {
		N := int(N0) + 1
		p, q := bestRationalApproximation(x, N)
		limit := 1
		if p == 0 {
			limit = -1
		}
		if p < -N || p > N || q < limit || q > N || (p == 0 && q == 0) {
			t.Errorf("%g ≈ %d/%d out of range (N=%d)", x, p, q, N)
		}
		y := float64(p) / float64(q)
		p2, q2 := bestRationalApproximation(y, N)
		if p == 0 && q2*q < 0 {
			p2, q2 = -p2, -q2
		}

		if p != p2 || q != q2 {
			t.Errorf("%g ≈ %d/%d ≈ %d/%d (N=%d)", x, p, q, p2, q2, N)
		}
	})
}

func FuzzBestRational2(f *testing.F) {
	f.Fuzz(func(t *testing.T, a, b int, N0 uint16) {
		N := int(N0) + 1
		if a > N || a < -N || b < 1 || b > N {
			return
		}
		x := float64(a) / float64(b)
		p, q := bestRationalApproximation(x, N)
		y := float64(p) / float64(q)
		if x != y {
			t.Errorf("%d/%d != %d/%d (N=%d)", a, b, p, q, N)
		}
	})
}

func TestAngle(t *testing.T) {
	rise, run := fromAngle(0)
	if rise != 1 || run != 0 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}

	// TODO(voss): is this the right answer?
	rise, run = fromAngle(-math.Pi / 4)
	if rise != 1 || run != 1 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}

	// TODO(voss): is this the right answer?
	rise, run = fromAngle(-math.Pi / 2)
	if rise != 0 || run != 1 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}

}

func FuzzAngle(f *testing.F) {
	f.Fuzz(func(t *testing.T, rise, run int16) {
		if run == 0 {
			rise = 1
		} else if rise == 0 {
			run = 1
		}

		a := rise
		b := run
		for b != 0 {
			a, b = b, a%b
		}
		if a < 0 {
			a = -a
		}

		rise2, run2 := fromAngle(toAngle(rise, run))
		if rise/a != rise2 || run/a != run2 {
			t.Errorf("%d/%d != %d/%d", rise/a, run/a, rise2, run2)
		}
	})
}

func FuzzHmtx(f *testing.F) {
	i1 := &Info{
		Widths: []uint16{100, 200, 300, 300},
		GlyphExtent: []font.Rect{
			{LLx: 10, LLy: 0, URx: 90, URy: 100},
			{LLx: 20, LLy: 0, URx: 200, URy: 100},
			{LLx: 30, LLy: 0, URx: 300, URy: 100},
			{LLx: 40, LLy: 0, URx: 300, URy: 100},
		},
		Ascent:      100,
		Descent:     -100,
		LineGap:     120,
		CaretAngle:  math.Pi / 180 * 10,
		CaretOffset: 2,
	}
	hhea, hmtx := EncodeHmtx(i1)
	f.Add(hhea, hmtx)

	f.Fuzz(func(t *testing.T, hhea, hmtx []byte) {
		i1, err := DecodeHmtx(hhea, hmtx)
		if err != nil {
			return
		}

		hhea, hmtx = EncodeHmtx(i1)
		i2, err := DecodeHmtx(hhea, hmtx)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(i1, i2) {
			fmt.Printf("%#v\n", i1)
			fmt.Printf("%#v\n", i2)
			t.Error("different")
		}
	})
}
