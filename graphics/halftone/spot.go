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

// These variables define the predefined spot functions for PDF 2.0 halftones.
// This is for use in the SpotFunction field of the [Type1] halftone
// dictionary.
var (
	SimpleDot         = spot("dup mul exch dup mul add 1 exch sub")
	InvertedSimpleDot = spot("dup mul exch dup mul add 1 sub")
	DoubleDot         = spot("360 mul sin 2 div exch 360 mul sin 2 div add")
	InvertedDoubleDot = spot("360 mul sin 2 div exch 360 mul sin 2 div add neg")
	CosineDot         = spot("180 mul cos exch 180 mul cos add 2 div")
	Double            = spot("360 mul sin 2 div exch 2 div 360 mul sin 2 div add")
	InvertedDouble    = spot("360 mul sin 2 div exch 2 div 360 mul sin 2 div add neg")
	Line              = spot("exch pop abs neg")
	LineX             = spot("pop")
	LineY             = spot("exch pop")
	Round             = spot(`abs exch abs 2 copy add 1 le
		{ dup mul exch dup mul add 1 exch sub }
		{ 1 sub dup mul exch 1 sub dup mul add 1 sub }
		ifelse`)
	Ellipse = spot(`abs exch abs 2 copy 3 mul exch 4 mul add 3 sub dup 0 lt
		{ pop dup mul exch 0.75 div dup mul add 4 div 1 exch sub }
		{ dup 1 gt
			{ pop 1 exch sub dup mul exch 1 exch sub 0.75 div dup mul add 4 div 1 sub }
			{ 0.5 exch sub exch pop exch pop }
			ifelse }
		ifelse`)
	EllipseA         = spot("dup mul 0.9 mul exch dup mul add 1 exch sub")
	InvertedEllipseA = spot("dup mul 0.9 mul exch dup mul add 1 sub")
	EllipseB         = spot("dup 5 mul 8 div mul exch dup mul exch add sqrt 1 exch sub")
	EllipseC         = spot("dup mul exch dup mul 0.9 mul add 1 exch sub")
	InvertedEllipseC = spot("dup mul exch dup mul 0.9 mul add 1 sub")
	Square           = spot("abs exch abs 2 copy lt { exch } if pop neg")
	Cross            = spot("abs exch abs 2 copy gt { exch } if pop neg")
	Rhomboid         = spot("abs exch abs 0.9 mul add 2 div")
	Diamond          = spot(`abs exch abs 2 copy add 0.75 le
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

func spot(psCode string) *function.Type4 {
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
