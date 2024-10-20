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
	"os"
	"path/filepath"
	"text/template"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/pagetree"
)

func main() {
	fileTmple := flag.String("t", "{{ .BaseName }} {{ .Pages }}", "file output template")
	summaryTmpl := flag.String("T", "total: {{ .Files }} files, {{ .Bytes }} bytes, {{ .Pages }} pages, {{ .Errors }} errors", "summary output template")
	flag.Parse()

	runner := &runner{}
	runner.fileTmpl = template.Must(template.New("file").Parse(*fileTmple))
	runner.summaryTmpl = template.Must(template.New("summary").Parse(*summaryTmpl))

	for _, fileName := range flag.Args() {
		runner.summary.Files++
		err := runner.run(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error:%s: %v\n", fileName, err)
			runner.summary.Errors++
		}
	}

	err := runner.summaryTmpl.Execute(os.Stdout, runner.summary)
	fmt.Println()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}

type fileInfo struct {
	Name     string
	BaseName string
	Bytes    int64
	Version  pdf.Version
	Pages    int
}

type summaryInfo struct {
	Files  int
	Errors int

	Bytes int64
	Pages int
}

type runner struct {
	fileTmpl    *template.Template
	summaryTmpl *template.Template

	summary summaryInfo
}

func (r *runner) run(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}
	size := stat.Size()

	pdfFile, err := pdf.NewReader(file, nil)
	if err != nil {
		return err
	}
	numPages, err := pagetree.NumPages(pdfFile)
	if err != nil {
		return err
	}

	meta := pdfFile.GetMeta()

	info := fileInfo{
		Name:     fileName,
		BaseName: filepath.Base(fileName),
		Bytes:    size,

		Version: meta.Version,
		Pages:   numPages,
	}

	r.summary.Bytes += size
	r.summary.Pages += numPages

	err = r.fileTmpl.Execute(os.Stdout, info)
	fmt.Println()
	return err
}
