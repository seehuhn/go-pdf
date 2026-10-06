// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package sample

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

const big = math.MaxFloat64

// testRanges include ranges whose width overflows, an empty range, and
// ranges too narrow for floats to separate all codes.
var testRanges = [][2]float64{
	{0, 1},
	{1, 0},
	{10, 200},
	{-1, 1},
	{-big, big},
	{big, -big},
	{0, big},
	{-big, -1},
	{7, 7},
	{1, 1 + 4*0x1p-52},
	{0, 5e-324},
	{-3, -1e-300},
}

// testCodes returns codes for the given bit width: all codes up to 16 bits,
// and a fixed selection for wider codes.
func testCodes(bits int) []uint32 {
	n := uint32(uint64(1)<<bits - 1)
	if bits <= 16 {
		codes := make([]uint32, 0, n+1)
		for c := range uint64(n) + 1 {
			codes = append(codes, uint32(c))
		}
		return codes
	}
	codes := []uint32{0, 1, 2, 3, n / 3, n / 2, n/2 + 1, n - 3, n - 2, n - 1, n}
	x := uint64(12345)
	for range 2000 {
		x = x*6364136223846793005 + 1442695040888963407
		codes = append(codes, uint32(x>>32)&n)
	}
	return codes
}

// checkCode verifies the properties of m which involve the given code.
func checkCode(t *testing.T, m Map, code uint32) {
	t.Helper()
	n := m.maxCode()
	lo, hi := min(m.Min, m.Max), max(m.Min, m.Max)

	v := m.Decode(code)
	if !(v >= lo && v <= hi) {
		t.Fatalf("%v: code %d decodes to %g, outside the range", m, code, v)
	}
	if code < n {
		w := m.Decode(code + 1)
		if m.Max >= m.Min && w < v || m.Max < m.Min && w > v {
			t.Fatalf("%v: decode not monotone at code %d: %g, %g", m, code, v, w)
		}
	}

	e := m.Encode(v)
	if e > n {
		t.Fatalf("%v: code %d out of range", m, e)
	}
	if e != code && m.Decode(e) != v {
		t.Fatalf("%v: code %d decodes to %g, which encodes to %d", m, code, v, e)
	}
}

// checkNeighbours verifies that x lies no further than the values of the
// neighbouring codes of Encode(x).  This fails if Encode needed more than
// one correction step.
func checkNeighbours(t *testing.T, m Map, x float64) {
	t.Helper()
	if m.Min == m.Max || math.IsNaN(x) {
		return
	}
	j := m.Encode(x)
	up := m.Max > m.Min
	if j < m.maxCode() {
		if v := m.Decode(j + 1); up && x > v || !up && x < v {
			t.Fatalf("%v: %g encodes to %d, but lies beyond code %d (%g)", m, x, j, j+1, v)
		}
	}
	if j > 0 {
		if v := m.Decode(j - 1); up && x < v || !up && x > v {
			t.Fatalf("%v: %g encodes to %d, but lies beyond code %d (%g)", m, x, j, j-1, v)
		}
	}
}

func TestEndpoints(t *testing.T) {
	for bits := 1; bits <= 32; bits++ {
		for _, r := range testRanges {
			m := Map{Bits: bits, Min: r[0], Max: r[1]}
			if v := m.Decode(0); v != m.Min {
				t.Errorf("%v: code 0 decodes to %g", m, v)
			}
			if v := m.Decode(m.maxCode()); v != m.Max {
				t.Errorf("%v: maximal code decodes to %g", m, v)
			}
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, bits := range []int{1, 2, 3, 4, 7, 8, 12, 16, 24, 31, 32} {
		codes := testCodes(bits)
		for _, r := range testRanges {
			m := Map{Bits: bits, Min: r[0], Max: r[1]}
			for _, code := range codes {
				checkCode(t, m, code)
				v := m.Decode(code)
				checkNeighbours(t, m, v)
				checkNeighbours(t, m, math.Nextafter(v, math.Inf(1)))
				checkNeighbours(t, m, math.Nextafter(v, math.Inf(-1)))
			}
		}
	}
}

// In [0, 1], code j covers the interval [j/(N+1), (j+1)/(N+1)).
func TestEqualBins(t *testing.T) {
	for bits := 1; bits <= 16; bits++ {
		m := Map{Bits: bits, Min: 0, Max: 1}
		n := m.maxCode()
		for j := uint32(1); j <= n; j++ {
			edge := float64(j) / (float64(n) + 1)
			if got := m.Encode(edge); got != j {
				t.Fatalf("bits %d: bin edge %g encodes to %d, want %d", bits, edge, got, j)
			}
			if got := m.Encode(math.Nextafter(edge, 0)); got != j-1 {
				t.Fatalf("bits %d: below bin edge %g encodes to %d, want %d", bits, edge, got, j-1)
			}
		}
		if got := m.Encode(1); got != n {
			t.Fatalf("bits %d: 1 encodes to %d, want %d", bits, got, n)
		}
	}
}

func TestEncodeClamp(t *testing.T) {
	cases := []struct {
		m    Map
		x    float64
		want uint32
	}{
		{Map{16, 0, 1}, -5, 0},
		{Map{16, 0, 1}, 1e300, 0xFFFF},
		{Map{16, 0, 1}, math.Inf(1), 0xFFFF},
		{Map{16, 0, 1}, math.Inf(-1), 0},
		{Map{16, 0, 1}, math.NaN(), 0},
		{Map{16, 1, 0}, 2, 0},
		{Map{16, 1, 0}, -2, 0xFFFF},
		{Map{16, -big, 0}, big, 0xFFFF},
		{Map{16, big, 0}, -big, 0xFFFF},
		{Map{16, -big, big}, math.Inf(1), 0xFFFF},
		{Map{16, -big, big}, math.NaN(), 0},
		{Map{16, 7, 7}, 7, 0},
		{Map{16, 7, 7}, 100, 0},
		{Map{32, 0, 1}, 2, 0xFFFFFFFF},
	}
	for _, c := range cases {
		if got := c.m.Encode(c.x); got != c.want {
			t.Errorf("%v: Encode(%g) = %d, want %d", c.m, c.x, got, c.want)
		}
	}
}

func TestEncodeMonotone(t *testing.T) {
	for _, bits := range []int{1, 4, 8, 16, 32} {
		for _, r := range testRanges {
			m := Map{Bits: bits, Min: r[0], Max: r[1]}
			lo, hi := min(r[0], r[1]), max(r[0], r[1])
			prev := m.Encode(math.Inf(-1))
			const steps = 10000
			for i := range steps + 1 {
				x := Lerp(float64(i)/steps, lo, hi)
				j := m.Encode(x)
				if r[1] >= r[0] && j < prev || r[1] < r[0] && j > prev {
					t.Fatalf("%v: encode not monotone at %g: %d after %d", m, x, j, prev)
				}
				prev = j
			}
		}
	}
}

func TestLerp(t *testing.T) {
	pairs := [][2]float64{{0, 1}, {1, 0}, {-big, big}, {big, -big}, {3, 3}, {2, 5}, {-5, -2}, {0, 5e-324}}
	for _, p := range pairs {
		a, b := p[0], p[1]
		if v := Lerp(0, a, b); v != a {
			t.Errorf("Lerp(0, %g, %g) = %g", a, b, v)
		}
		if v := Lerp(1, a, b); v != b {
			t.Errorf("Lerp(1, %g, %g) = %g", a, b, v)
		}
		prev := a
		for i := range 10001 {
			v := Lerp(float64(i)/10000, a, b)
			if b >= a && v < prev || b < a && v > prev {
				t.Fatalf("Lerp(·, %g, %g) not monotone at step %d", a, b, i)
			}
			if v < min(a, b) || v > max(a, b) {
				t.Fatalf("Lerp(·, %g, %g) = %g at step %d, outside the range", a, b, v, i)
			}
			prev = v
		}
	}
}

// goldenHash is the SHA-256 of the outputs computed by TestSameOnAllPlatforms.
// A platform which fuses a multiply and an add computes different outputs.
const goldenHash = "0c3e670da890d16426bea3093d7e537ae9426f49f54bf141a57de3d3374c2087"

func TestSameOnAllPlatforms(t *testing.T) {
	h := sha256.New()
	var buf [8]byte
	for _, bits := range []int{1, 8, 16, 32} {
		codes := testCodes(min(bits, 12))
		for _, r := range testRanges {
			m := Map{Bits: bits, Min: r[0], Max: r[1]}
			for _, code := range codes {
				code = code * uint32(uint64(1)<<bits/4096+1) & m.maxCode()
				v := m.Decode(code)
				binary.BigEndian.PutUint64(buf[:], math.Float64bits(v))
				h.Write(buf[:])

				x := Lerp(float64(code)/4095, r[0]-1, r[1]+1)
				binary.BigEndian.PutUint32(buf[:4], m.Encode(x))
				h.Write(buf[:4])
			}
		}
	}
	got := fmt.Sprintf("%x", h.Sum(nil))
	if got != goldenHash {
		t.Errorf("hash = %s, want %s", got, goldenHash)
	}
}

func FuzzMap(f *testing.F) {
	f.Add(uint8(15), 0.0, 1.0, uint32(1234), 0.5, 0.75)
	f.Add(uint8(31), -big, big, uint32(1), -big, big)
	f.Add(uint8(31), 0.0, 1.0, uint32(0xFFFFFFFE), 0.999, 1.0)
	f.Add(uint8(7), 1.0, 1+4*0x1p-52, uint32(77), 1.0, 1+0x1p-52)
	f.Add(uint8(3), 5.0, 5.0, uint32(3), 4.0, 6.0)
	f.Add(uint8(23), 1e300, -1e-300, uint32(99999), 0.0, 1e299)
	f.Fuzz(func(t *testing.T, bits uint8, lo, hi float64, code uint32, x, y float64) {
		if math.IsInf(lo, 0) || math.IsNaN(lo) || math.IsInf(hi, 0) || math.IsNaN(hi) {
			t.Skip()
		}
		m := Map{Bits: 1 + int(bits%32), Min: lo, Max: hi}
		n := m.maxCode()
		code &= n

		if v := m.Decode(0); v != lo {
			t.Fatalf("%v: code 0 decodes to %g", m, v)
		}
		if v := m.Decode(n); v != hi {
			t.Fatalf("%v: maximal code decodes to %g", m, v)
		}
		checkCode(t, m, code)
		checkNeighbours(t, m, x)
		checkNeighbours(t, m, y)

		if math.IsNaN(x) || math.IsNaN(y) {
			return
		}
		if x > y {
			x, y = y, x
		}
		jx, jy := m.Encode(x), m.Encode(y)
		if jx > n || jy > n {
			t.Fatalf("%v: code out of range", m)
		}
		if hi > lo && jx > jy || hi < lo && jx < jy {
			t.Fatalf("%v: encode not monotone: %g→%d, %g→%d", m, x, jx, y, jy)
		}
	})
}
