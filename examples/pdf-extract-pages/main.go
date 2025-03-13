// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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
	"io"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
	"seehuhn.de/go/pdf/pdfcopy"
)

func main() {
	out := flag.String("o", "out.pdf", "output file name")
	force := flag.Bool("f", false, "overwrite output file if it exists")
	pages := &PageRange{}
	flag.Var(pages, "p", "range of pages to extract")
	flag.Parse()

	if len(flag.Args()) != 1 {
		fmt.Fprintln(os.Stderr, "error: no input file given")
		flag.Usage()
		os.Exit(1)
	}
	if *out == "" {
		fmt.Fprintln(os.Stderr, "error: no output file specified")
		flag.Usage()
		os.Exit(1)
	}

	file, closer, err := openOutputFile(*out, *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = extractPages(file, flag.Arg(0), pages)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if closer != nil {
		err = closer.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func openOutputFile(outputFile string, forceOverwrite bool) (io.Writer, io.Closer, error) {
	if outputFile == "-" {
		return os.Stdout, nil, nil
	}

	flags := os.O_WRONLY | os.O_CREATE
	if forceOverwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(outputFile, flags, 0666)
	if err != nil {
		if os.IsExist(err) {
			return nil, nil, fmt.Errorf("file %s already exists and -f is not set", outputFile)
		}
		return nil, nil, err
	}

	return file, file, nil
}

func extractPages(w io.Writer, inputFile string, pages *PageRange) error {
	in, err := pdf.Open(inputFile, nil)
	if err != nil {
		return err
	}
	metaIn := in.GetMeta()

	numPages, err := pagetree.NumPages(in)
	if err != nil {
		return err
	}

	startPage := pages.Start
	endPage := pages.End
	if startPage < 1 {
		startPage = 1
	}
	if endPage > numPages {
		endPage = numPages
	}

	out, err := pdf.NewWriter(w, metaIn.Version, nil)
	if err != nil {
		return err
	}
	pageTreeOut := pagetree.NewWriter(out)

	copy := pdfcopy.NewCopier(out, in)

	for pageNo := startPage; pageNo <= endPage; pageNo++ {
		refIn, pageIn, err := pagetree.GetPage(in, pageNo-1)
		if err != nil {
			return err
		}

		// We remove the annotations here, because they may reference pages
		// which we don't want to include; these references would force these
		// pages to be included in the output files as well (with no entries in
		// the output page tree, but taking up space).
		//
		// TODO(voss): keep annotations which reference pages which are
		// included in the output file.
		delete(pageIn, "Annots")

		pageOut, err := copy.CopyDict(pageIn)
		if err != nil {
			return err
		}

		refOut := out.Alloc()
		if refIn != 0 {
			copy.Redirect(refIn, refOut)
		}

		pageTreeOut.AppendPageRef(refOut, pageOut)
	}

	treeRef, err := pageTreeOut.Close()
	if err != nil {
		return err
	}

	metaOut := out.GetMeta()
	metaOut.Catalog.Pages = treeRef
	metaOut.Info = metaIn.Info

	err = out.Close()
	if err != nil {
		return err
	}

	return nil
}
