// Horizontal Header Table
// https://docs.microsoft.com/en-us/typography/opentype/spec/hhea
package hhea

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
// required and must be used for advance widths. In a font with CFF outlines,
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
	"math"

	"seehuhn.de/go/pdf/font"
)

// Info contains information from the horizontal header table.
type Info struct {
	Ascent         int16 // distance from baseline of highest ascender
	Descent        int16 // distance from baseline of lowest descender
	LineGap        int16 // typographic line gap
	CaretSlopeRise int16 // slope of the caret (rise/run), 1 for vertical
	CaretSlopeRun  int16 // slope of the caret (rise/run), 0 for vertical
	CaretOffset    int16 // set value to 0 for non-slanted fonts
}

type Hhea struct {
	Version             uint32 // 0x00010000 (1.0)
	Ascent              int16  // Distance from baseline of highest ascender
	Descent             int16  // Distance from baseline of lowest descender
	LineGap             int16  // typographic line gap
	AdvanceWidthMax     uint16 // must be consistent with horizontal metrics
	MinLeftSideBearing  int16  // must be consistent with horizontal metrics
	MinRightSideBearing int16  // must be consistent with horizontal metrics
	XMaxExtent          int16  // max(lsb + (xMax-xMin))
	CaretSlopeRise      int16  // used to calculate the slope of the caret (rise/run) set to 1 for vertical caret
	CaretSlopeRun       int16  // 0 for vertical
	CaretOffset         int16  // set value to 0 for non-slanted fonts
	_                   int16  // set value to 0
	_                   int16  // set value to 0
	_                   int16  // set value to 0
	_                   int16  // set value to 0
	MetricDataFormat    int16  // 0 for current format
	NumOfLongHorMetrics uint16 // number of advance widths in "hmtx" table
}

type Hmtx struct {
	HMetrics        []LongHorMetric
	LeftSideBearing []int16
}

type LongHorMetric struct {
	AdvanceWidth    uint16
	LeftSideBearing int16
}

// GetAdvanceWidth returns the advance width of a glyph, in font design units.
func (h *Hmtx) GetAdvanceWidth(gid int) uint16 {
	if gid >= len(h.HMetrics) {
		return h.HMetrics[len(h.HMetrics)-1].AdvanceWidth
	}
	return h.HMetrics[gid].AdvanceWidth
}

// GetLSB returns the left side bearing of a glyph, in font design units.
func (h *Hmtx) GetLSB(gid int) int16 {
	if gid < len(h.HMetrics) {
		return h.HMetrics[gid].LeftSideBearing
	}
	gid -= len(h.HMetrics)
	if gid < len(h.LeftSideBearing) {
		return h.LeftSideBearing[gid]
	}
	return 0
}

type HmtxInfo struct {
	Widths      []uint16
	GlyphExtent []font.Rect
	Ascent      int16
	Descent     int16
	LineGap     int16
	CaretAngle  float64 // in radians, 0 for vertical
	CaretOffset int16
}

func CFFEncodeHmtx(info *HmtxInfo) ([]byte, []byte) {
	numGlyphs := len(info.Widths)
	numWidths := numGlyphs
	for numWidths > 1 && info.Widths[numWidths-1] == info.Widths[numWidths-2] {
		numWidths--
	}

	hhea := &Hhea{
		Version:             0x00010000, // 1.0
		Ascent:              info.Ascent,
		Descent:             info.Descent,
		LineGap:             info.LineGap,
		CaretOffset:         info.CaretOffset,
		NumOfLongHorMetrics: uint16(numWidths),
	}

	lsbs := make([]int16, numGlyphs)
	minLsb := int16(0x7fff)
	minRsb := int16(0x7fff)
	xMaxExtent := int16(0)
	for i, w := range info.Widths {
		if w > hhea.AdvanceWidthMax {
			hhea.AdvanceWidthMax = w
		}

		bbox := info.GlyphExtent[i]
		if bbox.IsZero() {
			continue
		}

		lsb := bbox.URx
		rsb := int16(w) - bbox.URx
		lsbs[i] = lsb
		if lsb < minLsb {
			minLsb = lsb
		}
		if rsb < minRsb {
			minRsb = rsb
		}
		if bbox.URx > xMaxExtent {
			xMaxExtent = bbox.URx
		}
	}
	if minLsb < int16(0x7fff) {
		hhea.MinLeftSideBearing = minLsb
		hhea.MinRightSideBearing = minRsb
		hhea.XMaxExtent = xMaxExtent
	}

	rise, run := riseAndRun(info.CaretAngle)
	hhea.CaretSlopeRise = int16(rise)
	hhea.CaretSlopeRun = int16(run)

	buf := bytes.NewBuffer(make([]byte, 0, hheaLength))
	_ = binary.Write(buf, binary.BigEndian, hhea)
	hheaData := buf.Bytes()

	buf = bytes.NewBuffer(make([]byte, 0, 4*numWidths+2*(numGlyphs-numWidths)))
	for i := 0; i < numGlyphs; i++ {
		if i < numWidths {
			_ = binary.Write(buf, binary.BigEndian, LongHorMetric{
				AdvanceWidth:    info.Widths[i],
				LeftSideBearing: lsbs[i],
			})
		} else {
			_ = binary.Write(buf, binary.BigEndian, lsbs[i])
		}
	}
	hmtxData := buf.Bytes()

	return hheaData, hmtxData
}

const hheaLength = 36

func riseAndRun(caretAngle float64) (int, int) {
	rise := 1
	run := 0
	s := math.Sin(caretAngle)
	c := math.Cos(caretAngle)
	if math.Abs(s) >= 1/32767.0 {
		rise, run = rationalApproximation(c/s, 32767)
	}
	return rise, run
}

// rationalApproximation returns a rational approximation of x
// with numerator and denominator <= N.
func rationalApproximation(x float64, N int) (int, int) {
	sign := 1
	if x < 0 {
		x = -x
		sign = -1
	}

	// Algorithm taken from:
	// https://en.wikipedia.org/wiki/Continued_fraction#Infinite_continued_fractions_and_convergents
	hiPrev := 0
	kiPrev := 1
	hi := 1
	ki := 0
	for {
		intPart := math.Floor(x)
		x = x - intPart
		ai := int(intPart)

		// hi * ai + hiPrev > N
		//   <=> hi * ai > N - hiPrev
		//   <=> hi > (N - hiPrev) / ai
		//   <=> hi > floor((N - hiPrev) / ai)
		if ai > 0 && (hi > (N-hiPrev)/ai || ki > (N-kiPrev)/ai) {
			break
		}

		hi, hiPrev = hi*ai+hiPrev, hi
		ki, kiPrev = ki*ai+kiPrev, ki

		if math.Abs(x) < 1/float64(N) {
			break
		}
		x = 1 / x
	}

	// TODO(voss): implement "reduced terms" as described on Wikipedia.
	// This will lead to even better approximations.

	return sign * hi, ki
}
