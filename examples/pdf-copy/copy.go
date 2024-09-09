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
	"log"
	"os"
	"runtime/pprof"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pdfcopy"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	r, err := pdf.Open(flag.Arg(0), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	w, err := pdf.Create("out.pdf", pdf.GetVersion(r), nil)
	if err != nil {
		log.Fatal(err)
	}

	trans := pdfcopy.NewCopier(w, r)

	newCatalog, err := pdfcopy.CopyStruct(trans, r.GetMeta().Catalog)
	if err != nil {
		log.Fatal(err)
	}
	w.GetMeta().Catalog = newCatalog

	newInfo, err := pdfcopy.CopyStruct(trans, r.GetMeta().Info)
	if err != nil {
		log.Fatal(err)
	}
	w.GetMeta().Info = newInfo

	w.GetMeta().ID = r.GetMeta().ID

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}
}
