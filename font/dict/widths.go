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
	"slices"

	"golang.org/x/exp/maps"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/encoding"
	"seehuhn.de/go/postscript/cid"
)

const DefaultWidthDefault = 1000.0

// getSimpleWidths populates ww with glyph widths from a font dictionary.
// It sets default widths for all glyphs, then updates specific widths from
// the Widths array in fontDict.
//
// Returns true if widths were successfully read.
func getSimpleWidths(ww []float64, r pdf.Getter, fontDict pdf.Dict, defaultWidth float64) bool {
	for c := range ww {
		ww[c] = defaultWidth
	}

	firstChar, _ := pdf.GetInteger(r, fontDict["FirstChar"])
	widths, _ := pdf.GetArray(r, fontDict["Widths"])
	if widths == nil || len(widths) > 256 || firstChar < 0 || firstChar >= 256 {
		return false
	}

	for i, w := range widths {
		w, err := pdf.GetNumber(r, w)
		if err != nil {
			continue
		}
		if code := firstChar + pdf.Integer(i); code < 256 {
			ww[code] = float64(w)
		}
	}
	return true
}

// setSimpleWidths updates fontDict with a FirstChar, LastChar, and Widths
// array based on non-default width values in ww. It ignores unused codes.
//
// Returns any additional objects and their references if an indirect object is
// created.
func setSimpleWidths(w *pdf.Writer, fontDict pdf.Dict, ww []float64, enc encoding.Type1, defaultWidth float64) ([]pdf.Object, []pdf.Reference) {
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

func decodeCompositeWidths(r pdf.Getter, obj pdf.Object) (map[cid.CID]float64, error) {
	w, err := pdf.GetArray(r, obj)
	if w == nil {
		return nil, err
	}

	res := make(map[cid.CID]float64)
	for len(w) > 1 {
		c0, err := pdf.GetInteger(r, w[0])
		if err != nil {
			return nil, err
		}
		obj1, err := pdf.Resolve(r, w[1])
		if err != nil {
			return nil, err
		}
		if c1, ok := obj1.(pdf.Integer); ok {
			if len(w) < 3 || c0 < 0 || c1 < c0 || c1-c0 > 65536 {
				return nil, pdf.Error("invalid W entry in CIDFont dictionary")
			}
			wi, err := pdf.GetNumber(r, w[2])
			if err != nil {
				return nil, err
			}
			for c := c0; c <= c1; c++ {
				cid := cid.CID(c)
				if pdf.Integer(cid) != c {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				res[cid] = float64(wi)
			}
			w = w[3:]
		} else {
			wi, err := pdf.GetArray(r, w[1])
			if err != nil {
				return nil, err
			}
			for _, wiObj := range wi {
				wi, err := pdf.GetNumber(r, wiObj)
				if err != nil {
					return nil, err
				}
				cid := cid.CID(c0)
				if pdf.Integer(cid) != c0 {
					return nil, pdf.Error("invalid W entry in CIDFont dictionary")
				}
				res[cid] = float64(wi)
				c0++
			}
			w = w[2:]
		}
	}
	if len(w) != 0 {
		return nil, pdf.Error("invalid W entry in CIDFont dictionary")
	}

	return res, nil
}

func encodeCompositeWidths(widthMap map[cid.CID]float64) pdf.Array {
	cidList := maps.Keys(widthMap)
	slices.Sort(cidList)

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
