// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package main

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"golang.org/x/exp/maps"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	for _, fname := range os.Args[1:] {
		err := doit(fname)
		if err != nil {
			panic(err)
		}
	}
}

func doit(fname string) error {
	fmt.Println("loading", fname, "...")
	r, err := pdf.Open(fname, nil)
	if err != nil {
		return err
	}
	defer r.Close()

	numPages, err := pagetree.NumPages(r)
	if err != nil {
		return err
	}
	fontMap := make(map[pdf.Reference]bool)
	for pageNo := 0; pageNo < numPages; pageNo++ {
		pageDict, err := pagetree.GetPage(r, pageNo)
		if err != nil {
			return err
		}

		resourcesDict, err := pdf.GetDict(r, pageDict["Resources"])
		if err != nil {
			return err
		}

		resources := &pdf.Resources{}
		err = pdf.DecodeDict(r, resources, resourcesDict)
		if err != nil {
			return err
		}

		for _, font := range resources.Font {
			fontRef, ok := font.(pdf.Reference)
			if !ok {
				fmt.Printf("funny font on page %d: %s\n", pageNo, pdf.Format(font))
				continue
			}
			fontMap[fontRef] = true
		}
	}

	allFonts := maps.Keys(fontMap)
	sort.Slice(allFonts, func(i, j int) bool {
		return allFonts[i] < allFonts[j]
	})

	fmt.Printf("%d fonts on %d pages\n", len(allFonts), numPages)
	for _, fontRef := range allFonts {
		font, err := pdf.GetDict(r, fontRef)
		if err != nil {
			return err
		}
		subtype, err := pdf.GetName(r, font["Subtype"])
		if err != nil {
			return err
		}
		fontName, err := pdf.GetName(r, font["BaseFont"])
		if err != nil {
			return err
		}
		var fontDesc pdf.Dict
		var encoding string
		if subtype == "Type0" {
			descendantFonts, err := pdf.GetArray(r, font["DescendantFonts"])
			if err != nil {
				return err
			}
			if len(descendantFonts) != 1 {
				return errors.New("expected exactly one descendant font")
			}
			cidFont, err := pdf.GetDict(r, descendantFonts[0])
			if err != nil {
				return err
			}
			subtype, err = pdf.GetName(r, cidFont["Subtype"])
			if err != nil {
				return err
			}
			fontDesc, err = pdf.GetDict(r, cidFont["FontDescriptor"])
			if err != nil {
				return err
			}
			encoding = pdf.Format(font["Encoding"])
		} else {
			fontDesc, err = pdf.GetDict(r, font["FontDescriptor"])
			if err != nil {
				return err
			}
		}
		if fontDesc == nil {
			if subtype == "Type1" && isBuiltinFont[fontName] {
				subtype = "builtin"
			} else if subtype != "Type3" {
				return errors.New("no font descriptor for " + string(fontName) + " " + string(subtype))
			}
		}

		var f3Subtype pdf.Name
		t3, err := pdf.GetStream(r, fontDesc["FontFile3"])
		if err != nil {
			return err
		} else if t3 != nil {
			f3Subtype, err = pdf.GetName(r, t3.Dict["Subtype"])
			if err != nil {
				return err
			}
		}

		var tp string
		var desc string
		switch {
		case subtype == "builtin":
			tp = "S"
			desc = "builtin"
		case subtype == "Type3":
			tp = "S"
			desc = "Type3"

		case subtype == "Type1" && fontDesc["FontFile"] != nil:
			tp = "S"
			desc = "Type1"
		case subtype == "MMType1" && fontDesc["FontFile"] != nil:
			tp = "S"
			desc = "MMType1"
		case subtype == "TrueType" && fontDesc["FontFile2"] != nil:
			tp = "S"
			desc = "TrueType"
		case subtype == "CIDFontType2" && fontDesc["FontFile2"] != nil:
			tp = "C"
			desc = "TrueType"
		case subtype == "Type1" && f3Subtype == "Type1C":
			tp = "S"
			desc = "CFF"
		case subtype == "MMType1" && f3Subtype == "Type1C":
			tp = "S"
			desc = "MMCFF"
		case subtype == "CIDFontType0" && f3Subtype == "CIDFontType0C":
			tp = "C"
			desc = "CFF"
		case subtype == "TrueType" && f3Subtype == "OpenType":
			tp = "S"
			desc = "OpenType/glyf"
		case subtype == "CIDFontType2" && f3Subtype == "OpenType":
			tp = "C"
			desc = "OpenType/glyf"
		case subtype == "CIDFontType0" && f3Subtype == "OpenType":
			tp = "C"
			desc = "OpenType/CFF"
		case subtype == "Type1" && f3Subtype == "OpenType":
			tp = "S"
			desc = "OpenType/CFF"

		case subtype == "Type1":
			tp = "S"
			desc = "Type1/ext"
		case subtype == "TrueType":
			tp = "S"
			desc = "TrueType/ext"

		default:
			fmt.Printf("%s: %q %q %q\n", fontRef, subtype, f3Subtype, fontName)
			panic("unknown font type")
		}

		fmt.Printf("%10s %s %-12s %-20s %s\n", fontRef, tp, desc, encoding, fontName)
	}
	fmt.Println()

	return nil
}

var isBuiltinFont = map[pdf.Name]bool{
	"Courier":               true,
	"Courier-Bold":          true,
	"Courier-BoldOblique":   true,
	"Courier-Oblique":       true,
	"Helvetica":             true,
	"Helvetica-Bold":        true,
	"Helvetica-BoldOblique": true,
	"Helvetica-Oblique":     true,
	"Times-Roman":           true,
	"Times-Bold":            true,
	"Times-BoldItalic":      true,
	"Times-Italic":          true,
	"Symbol":                true,
	"ZapfDingbats":          true,
}
