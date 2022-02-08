package hhea

import (
	"fmt"
	"math"
	"testing"

	"seehuhn.de/go/pdf/font"
)

func TestLengths(t *testing.T) {
	info := &HmtxInfo{
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

func TestRiseAndRun(t *testing.T) {
	rise, run := riseAndRun(0)
	if rise != 1 || run != 0 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}

	rise, run = riseAndRun(math.Pi / 4)
	if rise != 1 || run != 1 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}

	rise, run = riseAndRun(math.Pi / 2)
	if rise != 0 || run != 1 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}

	rise, run = riseAndRun(math.Pi / 180 * 9)
	if rise != 1 || run != 1 {
		t.Errorf("rise=%d, run=%d", rise, run)
	}
}

func TestRationalApproximation(t *testing.T) {
	a, b := rationalApproximation(math.Pi, 10000)
	if a != 355 || b != 113 {
		t.Errorf("approximation for π not found: a=%d, b=%d", a, b)
	}

	for _, x := range []float64{1, 0, -1, 3.0 / 2.0, -math.Pi, math.Sqrt2, math.E, -22.0 / 7.0} {
		for _, N := range []int{10, 100, 256, 512, 1000, 1024, 65535} {
			a, b := rationalApproximation(x, N)
			fmt.Printf("%g ≈ %d/%d (N=%d)\n", x, a, b, N)
			if a > N || a < -N || b < 1 || b > N {
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
