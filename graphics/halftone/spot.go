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

package halftone

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
)

// These variables define the predefined spot functions for PDF halftones.
// This is for use in the SpotFunction field of the [Type1] halftone
// dictionary.
var (
	SimpleDot         = ps("dup mul exch dup mul add 1 exch sub")
	InvertedSimpleDot = ps("dup mul exch dup mul add 1 sub")
	DoubleDot         = ps("360 mul sin 2 div exch 360 mul sin 2 div add")
	InvertedDoubleDot = ps("360 mul sin 2 div exch 360 mul sin 2 div add neg")
	CosineDot         = ps("180 mul cos exch 180 mul cos add 2 div")
	Double            = ps("360 mul sin 2 div exch 2 div 360 mul sin 2 div add")
	InvertedDouble    = ps("360 mul sin 2 div exch 2 div 360 mul sin 2 div add neg")
	Line              = ps("exch pop abs neg")
	LineX             = ps("pop")
	LineY             = ps("exch pop")
	Round             = ps(`abs exch abs 2 copy add 1 le
		{ dup mul exch dup mul add 1 exch sub }
		{ 1 sub dup mul exch 1 sub dup mul add 1 sub }
		ifelse`)
	Ellipse = ps(`abs exch abs 2 copy 3 mul exch 4 mul add 3 sub dup 0 lt
		{ pop dup mul exch 0.75 div dup mul add 4 div 1 exch sub }
		{ dup 1 gt
			{ pop 1 exch sub dup mul exch 1 exch sub 0.75 div dup mul add 4 div 1 sub }
			{ 0.5 exch sub exch pop exch pop }
			ifelse }
		ifelse`)
	EllipseA         = ps("dup mul 0.9 mul exch dup mul add 1 exch sub")
	InvertedEllipseA = ps("dup mul 0.9 mul exch dup mul add 1 sub")
	EllipseB         = ps("dup 5 mul 8 div mul exch dup mul exch add sqrt 1 exch sub")
	EllipseC         = ps("dup mul exch dup mul 0.9 mul add 1 exch sub")
	InvertedEllipseC = ps("dup mul exch dup mul 0.9 mul add 1 sub")
	Square           = ps("abs exch abs 2 copy lt { exch } if pop neg")
	Cross            = ps("abs exch abs 2 copy gt { exch } if pop neg")
	Rhomboid         = ps("abs exch abs 0.9 mul add 2 div")
	Diamond          = ps(`abs exch abs 2 copy add 0.75 le
		{ dup mul exch dup mul add 1 exch sub }
		{ 2 copy add 1.23 le
			{ 0.85 mul add 1 exch sub }
			{ 1 sub dup mul exch 1 sub dup mul add 1 sub }
			ifelse }
		ifelse`)
)

var (
	spotToName = map[pdf.Function]pdf.Name{
		SimpleDot:         "SimpleDot",
		InvertedSimpleDot: "InvertedSimpleDot",
		DoubleDot:         "DoubleDot",
		InvertedDoubleDot: "InvertedDoubleDot",
		CosineDot:         "CosineDot",
		Double:            "Double",
		InvertedDouble:    "InvertedDouble",
		Line:              "Line",
		LineX:             "LineX",
		LineY:             "LineY",
		Round:             "Round",
		Ellipse:           "Ellipse",
		EllipseA:          "EllipseA",
		InvertedEllipseA:  "InvertedEllipseA",
		EllipseB:          "EllipseB",
		EllipseC:          "EllipseC",
		InvertedEllipseC:  "InvertedEllipseC",
		Square:            "Square",
		Cross:             "Cross",
		Rhomboid:          "Rhomboid",
		Diamond:           "Diamond",
	}
	nameToSpot = map[pdf.Name]pdf.Function{
		"SimpleDot":         SimpleDot,
		"InvertedSimpleDot": InvertedSimpleDot,
		"DoubleDot":         DoubleDot,
		"InvertedDoubleDot": InvertedDoubleDot,
		"CosineDot":         CosineDot,
		"Double":            Double,
		"InvertedDouble":    InvertedDouble,
		"Line":              Line,
		"LineX":             LineX,
		"LineY":             LineY,
		"Round":             Round,
		"Ellipse":           Ellipse,
		"EllipseA":          EllipseA,
		"InvertedEllipseA":  InvertedEllipseA,
		"EllipseB":          EllipseB,
		"EllipseC":          EllipseC,
		"InvertedEllipseC":  InvertedEllipseC,
		"Square":            Square,
		"Cross":             Cross,
		"Rhomboid":          Rhomboid,
		"Diamond":           Diamond,
	}
)

func ps(psCode string) *function.Type4 {
	return &function.Type4{
		Domain:  spotDomain,
		Range:   spotRange,
		Program: psCode,
	}
}

var (
	spotDomain = []float64{-1, 1, -1, 1}
	spotRange  = []float64{-1, 1}
)
