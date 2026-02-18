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
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/tools/internal/buildinfo"
	"seehuhn.de/go/pdf/tools/internal/profile"
)

var (
	passwdArg  = flag.String("p", "", "PDF password")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile = flag.String("memprofile", "", "write memory profile to `file`")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "pdf-list-fonts \u2014 list fonts used in a PDF file\n")
		fmt.Fprintf(os.Stderr, "%s\n\n", buildinfo.Short("pdf-list-fonts"))
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  pdf-list-fonts [options] <file.pdf>...\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  file.pdf   one or more PDF files to inspect\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pdf-list-fonts document.pdf\n")
		fmt.Fprintf(os.Stderr, "  pdf-list-fonts -p secret encrypted.pdf\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	stop, err := profile.Start(*cpuprofile, *memprofile)
	if err != nil {
		return err
	}
	defer stop()

	for _, fname := range flag.Args() {
		err := listFonts(fname)
		if err != nil {
			return err
		}
	}
	return nil
}

func listFonts(fname string) error {
	var opt *pdf.ReaderOptions
	if *passwdArg != "" {
		opt = &pdf.ReaderOptions{
			ReadPassword: func(_ []byte, _ int) string {
				return *passwdArg
			},
		}
	}

	fmt.Println("loading", fname, "...")
	r, err := pdf.Open(fname, opt)
	if err != nil {
		return err
	}
	defer r.Close()

	numPages, err := pagetree.NumPages(r)
	if err != nil {
		return err
	}
	fontMap := make(map[pdf.Reference]bool)
	for pageNo := range numPages {
		_, pageDict, err := pagetree.GetPage(r, pageNo)
		if err != nil {
			return err
		}

		resourcesDict, err := pdf.GetDict(r, pageDict["Resources"])
		if err != nil {
			return err
		}

		fontDict, err := pdf.GetDict(r, resourcesDict["Font"])
		if err != nil {
			return err
		}

		for _, font := range fontDict {
			fontRef, ok := font.(pdf.Reference)
			if !ok {
				fmt.Printf("funny font on page %d: %s\n", pageNo, pdf.AsString(font))
				continue
			}
			fontMap[fontRef] = true
		}
	}

	allFonts := slices.Sorted(maps.Keys(fontMap))

	fmt.Printf("%d fonts on %d pages\n", len(allFonts), numPages)
	for _, fontRef := range allFonts {
		fontDict, err := pdf.GetDict(r, fontRef)
		if err != nil {
			return err
		}
		subtype, err := pdf.GetName(r, fontDict["Subtype"])
		if err != nil {
			return err
		}
		fontName, err := pdf.GetName(r, fontDict["BaseFont"])
		if err != nil {
			return err
		}
		var fontDesc pdf.Dict
		var encoding string
		if subtype == "Type0" {
			descendantFonts, err := pdf.GetArray(r, fontDict["DescendantFonts"])
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
			encoding = pdf.AsString(fontDict["Encoding"])
		} else {
			fontDesc, err = pdf.GetDict(r, fontDict["FontDescriptor"])
			if err != nil {
				return err
			}
		}
		if fontDesc == nil {
			if subtype == "Type1" {
				subtype = "standard"
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
			return fmt.Errorf("unknown font type: %s %q %q %q", fontRef, subtype, f3Subtype, fontName)
		}

		fmt.Printf("%-10s %s %-14s %-20s %s\n", fontRef, tp, desc, encoding, fontName)
	}
	fmt.Println()

	return nil
}
