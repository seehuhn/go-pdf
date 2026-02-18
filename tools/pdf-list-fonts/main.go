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
	"flag"
	"fmt"
	"maps"
	"os"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font/dict"
	"seehuhn.de/go/pdf/font/standard"
	"seehuhn.de/go/pdf/graphics/extract"
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

func isStandard(name string) bool {
	for _, f := range standard.All {
		if f.PostScriptName() == name {
			return true
		}
	}
	return false
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
	x := pdf.NewExtractor(r)
	for _, fontRef := range allFonts {
		d, err := extract.Dict(x, fontRef)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: %s: %v\n", fontRef, err)
			continue
		}

		var tp string
		var desc string
		var encoding string
		var fontName string
		switch f := d.(type) {
		case *dict.Type1:
			tp = "S"
			fontName = f.PostScriptName
			switch {
			case f.FontFile == nil && isStandard(f.PostScriptName):
				desc = "standard"
			case f.FontFile == nil:
				desc = "Type1/ext"
			default:
				desc = f.FontFile.Type.String()
			}
		case *dict.TrueType:
			tp = "S"
			fontName = f.PostScriptName
			if f.FontFile == nil {
				desc = "TrueType/ext"
			} else {
				desc = f.FontFile.Type.String()
			}
		case *dict.Type3:
			tp = "S"
			// https://github.com/pdf-association/pdf-issues/issues/11#issuecomment-753665847
			fontName = string(f.Name)
			desc = "Type3"
		case *dict.CIDFontType0:
			tp = "C"
			fontName = f.PostScriptName
			encoding = f.CMap.Name
			if f.FontFile == nil {
				desc = "CFF/ext"
			} else {
				desc = f.FontFile.Type.String()
			}
		case *dict.CIDFontType2:
			tp = "C"
			fontName = f.PostScriptName
			encoding = f.CMap.Name
			if f.FontFile == nil {
				desc = "TrueType/ext"
			} else {
				desc = f.FontFile.Type.String()
			}
		}

		fmt.Printf("%-10s %s %-14s %-20s %s\n", fontRef, tp, desc, encoding, fontName)
	}
	fmt.Println()

	return nil
}
