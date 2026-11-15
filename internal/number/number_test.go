// Copyright 2026 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package number

import (
	"math"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		kind Kind
		i    int64
		x    float64
	}{
		// integers
		{"0", Integer, 0, 0},
		{"42", Integer, 42, 0},
		{"-1", Integer, -1, 0},
		{"+1", Integer, 1, 0},
		{"009", Integer, 9, 0},
		{"9223372036854775807", Integer, math.MaxInt64, 0},

		// reals
		{"0.0", Real, 0, 0},
		{".5", Real, 0, 0.5},
		{"-.5", Real, 0, -0.5},
		{"+.5", Real, 0, 0.5},
		{"100.", Real, 0, 100},
		{"-3.14", Real, 0, -3.14},
		{"9223372036854775808", Real, 0, 9223372036854775808},

		// exponential format
		{"6.02E23", Real, 0, 6.02e23},
		{"1e5", Real, 0, 1e5},
		{"1E+5", Real, 0, 1e5},
		{"1.0e-5", Real, 0, 1e-5},
		{"-.5e1", Real, 0, -5},
		{"4.e2", Real, 0, 400},
		{"0e0", Real, 0, 0},
		{"1e-999", Real, 0, 0},

		// radix format
		{"16#FFFE", Integer, 0xFFFE, 0},
		{"16#fffe", Integer, 0xFFFE, 0},
		{"8#1777", Integer, 0o1777, 0},
		{"2#1000", Integer, 8, 0},
		{"36#Zz", Integer, 35*36 + 35, 0},
		{"16#7FFFFFFF", Integer, math.MaxInt32, 0},
		{"16#80000000", Integer, math.MinInt32, 0},
		{"16#FFFFFFFF", Integer, -1, 0},
		{"2#11111111111111111111111111111110", Integer, -2, 0},
		{"016#0010", Integer, 16, 0},

		// invalid
		{"", Invalid, 0, 0},
		{"abc", Invalid, 0, 0},
		{"+", Invalid, 0, 0},
		{"-", Invalid, 0, 0},
		{".", Invalid, 0, 0},
		{"-.", Invalid, 0, 0},
		{"1.2.3", Invalid, 0, 0},
		{"1..2", Invalid, 0, 0},
		{"1-2", Invalid, 0, 0},
		{"--1", Invalid, 0, 0},
		{"12abc", Invalid, 0, 0},
		{"1_000", Invalid, 0, 0},
		{"0x10", Invalid, 0, 0},
		{"inf", Invalid, 0, 0},
		{"1e", Invalid, 0, 0},
		{"1e+", Invalid, 0, 0},
		{"e5", Invalid, 0, 0},
		{".e5", Invalid, 0, 0},
		{"1e5.0", Invalid, 0, 0},
		{"1e5e5", Invalid, 0, 0},
		{"1e999", Invalid, 0, 0},
		{"-1e999", Invalid, 0, 0},
		{"16#", Invalid, 0, 0},
		{"#FF", Invalid, 0, 0},
		{"16#FG", Invalid, 0, 0},
		{"2#2", Invalid, 0, 0},
		{"1#0", Invalid, 0, 0},
		{"37#1", Invalid, 0, 0},
		{"-16#FF", Invalid, 0, 0},
		{"+16#FF", Invalid, 0, 0},
		{"16#-FF", Invalid, 0, 0},
		{"1.6#FF", Invalid, 0, 0},
		{"16#F_F", Invalid, 0, 0},
		{"16#100000000", Invalid, 0, 0},
		{"16#FFFFFFFFFFFFFFFFFFFF", Invalid, 0, 0},
		{"99999999999999999999#1", Invalid, 0, 0},
		{"16#FF#1", Invalid, 0, 0},
	}
	for _, tt := range tests {
		kind, i, x := Parse([]byte(tt.in))
		if kind != tt.kind || i != tt.i || x != tt.x {
			t.Errorf("Parse(%q) = (%v, %d, %g), want (%v, %d, %g)",
				tt.in, kind, i, x, tt.kind, tt.i, tt.x)
		}
	}
}
