// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

// Package hmtx has code for reading and wrinting the "hhea" and "hmtx" tables.
// https://docs.microsoft.com/en-us/typography/opentype/spec/hhea
package hmtx

// Glyph metrics used for horizontal text layout include:
//   - glyph advance widths,
//   - side bearings
//   - X-direction min and max values (xMin, xMax).
// These are derived using a combination of the glyph outline data ('glyf',
// 'CFF ' or CFF2) and the horizontal metrics table ('hmtx').  The 'hmtx'
// table provides glyph advance widths and left side bearings.

// OpenType:
//
// In a font with CFF version 1 outline data, the 'CFF ' table does include
// advance widths. These values are used by PostScript processors, but are not
// used in OpenType layout.  In an OpenType context, the 'hmtx' table is
// required and must be used for advance widths.  In a font with CFF outlines,
// xMin (= left side bearing) and xMax values can be obtained from the CFF
// rasterizer. Some layout engines may use left side bearing values in the
// 'hmtx' table, however; hence, font production tools should ensure that the
// left side bearing values in the 'hmtx' table match the implicit xMin values
// reflected in the CharString data.

// TrueType:
//
// In a font with TrueType outline data, the 'glyf' table provides xMin and
// xMax values, but not advance widths or side bearings.  The advance width is
// always obtained from the 'hmtx' table.  In some fonts the left side bearings
// may be the same as the xMin values in the 'glyf' table, though this is not
// true for all fonts. (See the description of bit 1 of the flags field in the
// 'head' table.) For this reason, left side bearings are provided in the
// 'hmtx' table.
//
// In a font with TrueType outlines, xMin and xMax values for each glyph are
// given in the 'glyf' table.  The advance width (“aw”) and left side bearing
// (“lsb”) can be derived from the glyph “phantom points”, which are computed
// by the TrueType rasterizer.

// If a glyph has no contours, xMax/xMin are not defined. The left side bearing
// indicated in the 'hmtx' table for such glyphs should be zero.
//
// The right side bearing is always derived using advance width and left side
// bearing values from the 'hmtx' table, plus bounding-box information in the
// glyph description:
//
//     rsb = aw - (lsb + xMax - xMin)

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"seehuhn.de/go/pdf/font"
)

// Info contains information from the "hhea" and "hmtx" tables.
type Info struct {
	Width       []uint16
	GlyphExtent []font.Rect
	LSB         []int16

	Ascent  int16
	Descent int16 // negative
	LineGap int16

	CaretAngle  float64 // in radians, 0 for vertical
	CaretOffset int16
}

// Decode extracts information from the "hhea" and "hmtx" tables.
func Decode(hheaData, hmtxData []byte) (*Info, error) {
	r := bytes.NewReader(hheaData)
	hheaEnc := &binaryHhea{}
	err := binary.Read(r, binary.BigEndian, hheaEnc)
	if err != nil {
		return nil, err
	}
	if hheaEnc.Version != 0x00010000 {
		return nil, fmt.Errorf("unsupported hhea version %08x", hheaEnc.Version)
	}
	if hheaEnc.MetricDataFormat != 0 {
		return nil, fmt.Errorf("unsupported metric data format %d", hheaEnc.MetricDataFormat)
	}

	caretAngle := toAngle(hheaEnc.CaretSlopeRise, hheaEnc.CaretSlopeRun)
	info := &Info{
		Ascent:      hheaEnc.Ascent,
		Descent:     hheaEnc.Descent,
		LineGap:     hheaEnc.LineGap,
		CaretAngle:  caretAngle,
		CaretOffset: hheaEnc.CaretOffset,
	}

	numHorMetrics := int(hheaEnc.NumOfLongHorMetrics)
	prevWidth := uint16(0)
	var widths []uint16
	var lsbs []int16
	for i := 0; len(hmtxData) > 0; i++ {
		width := prevWidth
		if i < numHorMetrics {
			if len(hmtxData) < 2 {
				return nil, fmt.Errorf("hmtx too short")
			}
			width = uint16(hmtxData[0])<<8 | uint16(hmtxData[1])
			hmtxData = hmtxData[2:]
			prevWidth = width
		}
		widths = append(widths, width)

		if len(hmtxData) < 2 {
			return nil, fmt.Errorf("hmtx too short")
		}
		lsb := int16(hmtxData[0])<<8 | int16(hmtxData[1])
		hmtxData = hmtxData[2:]
		lsbs = append(lsbs, lsb)
	}
	if len(widths) < numHorMetrics {
		return nil, fmt.Errorf("hmtx too short")
	}
	info.Width = widths
	info.LSB = lsbs

	return info, nil
}

// Encode creates the "hhea" and "hmtx" tables.
func (info *Info) Encode() (hheaData []byte, hmtxData []byte) {
	numGlyphs := len(info.Width)
	if info.LSB != nil && len(info.LSB) != numGlyphs {
		panic("lsb length mismatch")
	}
	if info.GlyphExtent != nil && len(info.GlyphExtent) != numGlyphs {
		panic("GlyphExtent length mismatch")
	}

	numLong := numGlyphs
	for numLong > 1 && info.Width[numLong-1] == info.Width[numLong-2] {
		numLong--
	}

	rise, run := fromAngle(info.CaretAngle)

	hhea := &binaryHhea{
		Version: 0x00010000, // 1.0
		Ascent:  info.Ascent,
		Descent: info.Descent,
		LineGap: info.LineGap,

		CaretSlopeRise: rise,
		CaretSlopeRun:  run,
		CaretOffset:    info.CaretOffset,

		NumOfLongHorMetrics: uint16(numLong),
	}

	for _, w := range info.Width {
		if w > hhea.AdvanceWidthMax {
			hhea.AdvanceWidthMax = w
		}
	}

	lsbs := info.LSB
	if lsbs == nil {
		lsbs = make([]int16, numGlyphs)
		for i := 0; i < numGlyphs; i++ {
			lsbs[i] = info.GlyphExtent[i].LLx
		}
	}
	first := true
	for i, lsb := range lsbs {
		if info.GlyphExtent != nil && info.GlyphExtent[i].IsZero() {
			continue
		}
		if first || lsb < hhea.MinLeftSideBearing {
			hhea.MinLeftSideBearing = lsb
			first = false
		}
	}

	if info.GlyphExtent != nil {
		first = true
		for i, bbox := range info.GlyphExtent {
			if bbox.IsZero() {
				continue
			}

			rsb := int16(info.Width[i]) - bbox.URx
			if first || rsb < hhea.MinRightSideBearing {
				hhea.MinRightSideBearing = rsb
			}
			if first || bbox.URx > hhea.XMaxExtent {
				hhea.XMaxExtent = bbox.URx
			}
			first = false
		}
	}

	buf := bytes.NewBuffer(make([]byte, 0, hheaLength))
	_ = binary.Write(buf, binary.BigEndian, hhea)
	hheaData = buf.Bytes()

	buf = bytes.NewBuffer(make([]byte, 0, 4*numLong+2*(numGlyphs-numLong)))
	for i := 0; i < numGlyphs; i++ {
		if i < numLong {
			buf.Write([]byte{
				byte(info.Width[i] >> 8), byte(info.Width[i]),
			})
		}
		buf.Write([]byte{
			byte(lsbs[i] >> 8), byte(lsbs[i]),
		})
	}
	hmtxData = buf.Bytes()

	return hheaData, hmtxData
}

const hheaLength = 36

func toAngle(rise, run int16) float64 {
	// slope = rise / run (rise = 1, run = 0 for vertical)
	// angle = 0 for vertical, angle<0 for italic

	// avoid numbers with no negative
	if rise == -32768 {
		rise = -32767
	}
	if run == -32768 {
		run = -32767
	}

	caretAngle := math.Atan2(
		float64(rise),
		float64(run),
	) - math.Pi/2
	return caretAngle
}

func fromAngle(caretAngle float64) (rise, run int16) {
	phi := caretAngle + math.Pi/2
	s := math.Sin(phi)
	c := math.Cos(phi)
	if math.Abs(c) <= 0.5/32767.0 {
		if s >= 0 {
			return 1, 0
		}
		return -1, 0
	}
	rise0, run0 := bestRationalApproximation(s/c, 32767)
	if s*float64(rise0) < 0 {
		rise0, run0 = -rise0, -run0
	}
	return int16(rise0), int16(run0)
}

// bestRationalApproximation returns a rational approximation of x
// with abs(p)<=N and 0<q <= N and p/q ≈ x.
func bestRationalApproximation(x float64, N int) (p int, q int) {
	sign := 1
	if x < 0 {
		x = -x
		sign = -1
	}

	// x ≈ p/q

	Nf := float64(N)
	if x < 0.5/Nf {
		return 0, sign
	} else if x > Nf-0.5 {
		return sign * N, 1
	}

	maxDenom := N
	if x > 1 {
		// we need round(x*maxDenom) <= N
		// i.e. x * maxDenom < N+0.5
		maxDenom = int(math.Floor((Nf + 0.5) / x))
	}
	bestDist := math.Inf(1)
	bestDenom := 0
	bestNumerator := 0
	for denom := 1; denom <= maxDenom; denom++ {
		numerator := int(math.Round(x * float64(denom)))
		if numerator > N {
			continue
		}
		y := float64(numerator) / float64(denom)
		dist := math.Abs(x - y)
		if dist < bestDist {
			bestDist = dist
			bestDenom = denom
			bestNumerator = numerator
		}
	}
	return sign * bestNumerator, bestDenom
}

type binaryHhea struct {
	Version             uint32
	Ascent              int16
	Descent             int16
	LineGap             int16
	AdvanceWidthMax     uint16
	MinLeftSideBearing  int16
	MinRightSideBearing int16
	XMaxExtent          int16
	CaretSlopeRise      int16
	CaretSlopeRun       int16
	CaretOffset         int16
	_                   int16
	_                   int16
	_                   int16
	_                   int16
	MetricDataFormat    int16
	NumOfLongHorMetrics uint16
}
