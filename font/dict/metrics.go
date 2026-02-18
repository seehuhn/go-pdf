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

package dict

import (
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
	_ "seehuhn.de/go/pdf/font" // for the doc strings
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/postscript/cid"
)

// DefaultWidthDefault is the value of DefaultWidth which is used if no
// value is specified in the PDF file.  Using this value for the "DefaultWidth"
// field in a composite font dictionary will slightly reduce PDF file size.
const DefaultWidthDefault = 1000

// setSimpleWidths updates fontDict with a FirstChar, LastChar, and Widths
// array based on non-default width values in ww. It ignores unused codes.
//
// Returns any additional objects and their references if an indirect object is
// created.
func setSimpleWidths(w *pdf.Writer, fontDict pdf.Dict, ww []float64, enc encoding.Simple, defaultWidth float64) ([]pdf.Object, []pdf.Reference) {
	// Find the range [firstChar, lastChar] of characters that need encoding.
	// The spec requires at least one character to be encoded.
	// The firstChar < lastChar condition ensures firstChar <= lastChar,
	// creating a one-element array when all chars have default width.
	firstChar, lastChar := 0, 255
	for lastChar > 0 && (enc(byte(lastChar)) == "" || ww[lastChar] == defaultWidth) {
		lastChar--
	}
	for firstChar < lastChar && (enc(byte(firstChar)) == "" || ww[firstChar] == defaultWidth) {
		firstChar++
	}

	widths := make(pdf.Array, lastChar-firstChar+1)
	for i := range widths {
		widths[i] = pdf.Number(ww[firstChar+i])
	}

	fontDict["FirstChar"] = pdf.Integer(firstChar)
	fontDict["LastChar"] = pdf.Integer(lastChar)

	if len(widths) <= 10 {
		fontDict["Widths"] = widths
		return nil, nil
	}

	widthRef := w.Alloc()
	fontDict["Widths"] = widthRef
	return []pdf.Object{widths}, []pdf.Reference{widthRef}
}

// encodeCompositeWidths creates a W array for a composite font from a map of CID to width values.
// Uses the most compact representation possible.
func encodeCompositeWidths(widthMap map[cid.CID]float64) pdf.Array {
	cidList := slices.Sorted(maps.Keys(widthMap))

	// There are two ways to encode the widths:
	//   - elements `startCID, endCID, width` indicate a run of CIDs with the same width
	//   - elements `startCID [w1 ... wn]` give the widths for a range of CIDs
	var res pdf.Array
	var runStart, runEnd cid.CID
	var run pdf.Array
	var allEqual bool
	for _, cid := range cidList {
		w := pdf.Number(widthMap[cid])

		if len(run) > 0 && cid != runEnd+1 || // forced break of run, or
			len(run) > 2 && allEqual && w != run[0] { // can use compressed form
			// flush previous run
			if allEqual && len(run) > 1 {
				res = append(res,
					pdf.Integer(runStart),
					pdf.Integer(runEnd),
					run[0])
				run = run[:0] // slice not used in output -> can re-use
			} else {
				res = append(res,
					pdf.Integer(runStart),
					run)
				run = nil // prevent the slice from being overwritten
			}
		}

		// now run is empty or cid == runEnd+1

		if len(run) == 0 {
			runStart = cid
			allEqual = true
		} else {
			allEqual = allEqual && w == run[0]
		}
		runEnd = cid
		run = append(run, w)
	}

	if len(run) > 0 {
		// flush final run
		if allEqual && len(run) > 1 {
			res = append(res,
				pdf.Integer(runStart),
				pdf.Integer(runEnd),
				pdf.Number(widthMap[runStart]))
		} else {
			res = append(res,
				pdf.Integer(runStart),
				run)
		}
	}

	return res
}

// DefaultVMetricsDefault is the value of DefaultVMetrics used, if no value is
// specified in the PDF file.  Using this value for a "DefaultVMetrics" field
// in a composite font dictionary will slightly reduce PDF file size.
var DefaultVMetricsDefault = DefaultVMetrics{
	OffsY:  880,
	DeltaY: -1000,
}

// DefaultVMetrics represents the default vertical positioning and advancement
// for a font in vertical writing mode.
type DefaultVMetrics struct {
	// OffsY is the vertical component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved down by OffsY, compared to
	// horizontal writing.
	//
	// There is no separate OffsX default value for vertical writing. Instead,
	// half of the horizontal glyph width is used as the default.
	OffsY float64

	// DeltaY is the vertical displacement of the following glyph, in PDF glyph
	// space units. (The horizontal displacement is always zero.)
	//
	// This is normally negative, so that writing goes from top to bottom.
	DeltaY float64
}

// encodeVDefault creates a PDF array representation of default vertical metrics.
// Returns nil if the metrics match the default values to save space.
func encodeVDefault(metrics DefaultVMetrics) pdf.Array {
	if metrics.OffsY == 880 && metrics.DeltaY == -1000 {
		return nil
	}
	return pdf.Array{
		pdf.Number(metrics.OffsY),
		pdf.Number(metrics.DeltaY),
	}
}

// VMetrics represents the vertical metrics for an individual CID.
type VMetrics struct {
	// OffsX is the horizontal component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved left by OffsX units,
	// compared to horizontal writing.
	OffsX float64

	// OffsY is the vertical component of the glyph position vector, in PDF
	// glyph space units.
	//
	// The effect of this is that the glyph is moved down by OffsY, compared to
	// horizontal writing.
	OffsY float64

	// DeltaY is the vertical displacement for the following glyph.
	// (The horizontal displacement is always zero.)
	//
	// This is normally negative, so that writing goes from top to bottom.
	DeltaY float64
}

// encodeVMetrics creates a W2 array for a composite font from a map of CID to vertical metrics.
// Uses the most compact representation possible.
func encodeVMetrics(metrics map[cid.CID]VMetrics) pdf.Array {
	if metrics == nil {
		return nil
	}

	res := pdf.Array{}

	cids := slices.Sorted(maps.Keys(metrics))
	for len(cids) > 0 {
		// Try to find a range of consecutive CIDs
		// with pairwise different metrics:
		end := 0
		for end < len(cids) {
			if end > 0 && cids[end-1]+1 != cids[end] {
				// not consecutive
				break
			}
			if end+1 < len(cids) &&
				cids[end]+1 == cids[end+1] &&
				metrics[cids[end]] == metrics[cids[end+1]] {
				// increasing enc would split consecutive CIDs with the same metrics
				break
			}
			end++
		}
		if end > 0 {
			// Individual format: cid [dy1 ox1 oy1 ...]
			data := make(pdf.Array, 0, 3*end)
			for i := 0; i < end; i++ {
				cid := cids[i]
				data = append(data,
					pdf.Number(metrics[cid].DeltaY),
					pdf.Number(metrics[cid].OffsX),
					pdf.Number(metrics[cid].OffsY))
			}
			res = append(res, pdf.Integer(cids[0]), data)
			cids = cids[end:]
		}

		// If we reach this point without having removed any elements from the
		// slice cids, the loop above must have terminated during the first
		// iteration (end == 0). This implies that all of the following are
		// true:
		//  - end+1 < len(cids)
		//  - cids[end]+1 == cids[end+1]
		//  - metrics[cids[end]] == metrics[cids[end+1]]
		// Thus, in this case the following code will remove at least two cid
		// values.

		// Try to find a range of consecutive CIDs
		// with identical metrics:
		end = 1
		for end < len(cids) {
			if cids[end-1]+1 != cids[end] || metrics[cids[end-1]] != metrics[cids[end]] {
				// not consecutive
				break
			}
			end++
		}
		if end >= 2 {
			// Range format: cid1 cid2 dy ox oy
			res = append(res,
				pdf.Integer(cids[0]),
				pdf.Integer(cids[end-1]),
				pdf.Number(metrics[cids[0]].DeltaY),
				pdf.Number(metrics[cids[0]].OffsX),
				pdf.Number(metrics[cids[0]].OffsY))
			cids = cids[end:]
		}
	}

	return res
}
