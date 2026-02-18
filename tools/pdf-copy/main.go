// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/tools/internal/buildinfo"
	"seehuhn.de/go/pdf/tools/internal/profile"
)

var (
	out        = flag.String("o", "out.pdf", "output file name")
	force      = flag.Bool("f", false, "overwrite output file if it exists")
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
	memprofile = flag.String("memprofile", "", "write memory profile to `file`")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "pdf-copy \u2014 copy a PDF file\n")
		fmt.Fprintf(os.Stderr, "%s\n\n", buildinfo.Short("pdf-copy"))
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  pdf-copy [options] <input.pdf>\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  input.pdf   PDF file to copy\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  pdf-copy input.pdf\n")
		fmt.Fprintf(os.Stderr, "  pdf-copy -o copy.pdf -f input.pdf\n")
	}
	flag.Parse()

	if flag.NArg() != 1 {
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

	if !*force {
		if _, err := os.Stat(*out); !os.IsNotExist(err) {
			return fmt.Errorf("output file %q already exists (use -f to overwrite)", *out)
		}
	}

	return copyPDF(flag.Arg(0), *out)
}

func copyPDF(inFile, outFile string) (retErr error) {
	r, err := pdf.Open(inFile, nil)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := pdf.Create(outFile, pdf.GetVersion(r), nil)
	if err != nil {
		return err
	}
	defer func() {
		if retErr != nil {
			os.Remove(outFile)
		}
	}()

	trans := pdf.NewCopier(w, r)

	newDict, err := trans.CopyDict(pdf.AsDict(r.GetMeta().Catalog))
	if err != nil {
		return err
	}
	newCatalog, err := pdf.ExtractCatalog(w, newDict)
	if err != nil {
		return err
	}
	w.GetMeta().Catalog = newCatalog

	w.GetMeta().Info = r.GetMeta().Info

	w.GetMeta().ID = r.GetMeta().ID

	return w.Close()
}
