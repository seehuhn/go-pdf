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
	"bytes"
	"go/format"
	"os"
	"text/template"

	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/pdf/font/standard"
)

func main() {
	err := Generate("data.go")
	if err != nil {
		panic(err)
	}
}

func Generate(fname string) error {
	data, err := getAllData()
	if err != nil {
		return err
	}

	tmpl := template.Must(template.New("out").Parse(outTmpl))

	// Execute the template with the data.
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		return err
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return err
	}

	err = os.WriteFile(fname, formatted, 0644)
	if err != nil {
		return err
	}

	return nil
}

type Data map[string]*fontMetrics

func getAllData() (Data, error) {
	data := make(Data)
	for _, font := range standard.All {
		err := getFontData(data, font)
		if err != nil {
			return nil, err
		}
	}
	return data, nil
}

func getFontData(data Data, font standard.Font) error {
	F := font.New()
	family := F.Font.FamilyName

	var weight string
	switch F.Font.Weight {
	case "Regular":
		weight = "os2.WeightNormal"
	case "Bold":
		weight = "os2.WeightBold"
	default:
		panic("unreachable")
	}

	bbox := F.Font.FontBBoxPDF()

	widths := make(map[string]float64)
	for name, info := range F.Metrics.Glyphs {
		widths[name] = info.WidthX
	}

	isSymbolic := false
	encoding := "standardEncoding"
	switch F.FontName {
	case "Symbol":
		encoding = "symbolEncoding"
		isSymbolic = true
	case "ZapfDingbats":
		encoding = "zapfDingbatsEncoding"
		isSymbolic = true
	}

	data[F.FontName] = &fontMetrics{
		FontFamily:   family,
		FontWeight:   weight,
		IsFixedPitch: F.Font.IsFixedPitch,
		IsSerif:      F.IsSerif,
		IsSymbolic:   isSymbolic,
		FontBBox:     bbox,
		ItalicAngle:  F.Font.ItalicAngle,
		Ascent:       F.Metrics.Ascent,
		Descent:      F.Metrics.Descent,
		CapHeight:    F.Metrics.CapHeight,
		XHeight:      F.Metrics.XHeight,
		StemV:        F.Font.Private.StdVW,
		StemH:        F.Font.Private.StdHW,

		Widths: widths,

		Encoding: encoding,
	}

	return nil
}

type fontMetrics struct {
	FontFamily   string
	FontWeight   string
	IsFixedPitch bool
	IsSerif      bool
	IsSymbolic   bool
	FontBBox     rect.Rect
	ItalicAngle  float64
	Ascent       float64
	Descent      float64 // negative
	CapHeight    float64
	XHeight      float64
	StemV        float64
	StemH        float64

	Widths map[string]float64

	Encoding string
}

const outTmpl = `// Code generated by seehuhn.de/go/pdf/internal/stdmtx/generate; DO NOT EDIT.

package stdmtx

import (
	"seehuhn.de/go/geom/rect"

	"seehuhn.de/go/sfnt/os2"
)

var metrics = map[string]*FontData{
{{- range $fontName, $metrics := . }}
    "{{ $fontName }}": {
		FontFamily: "{{ $metrics.FontFamily }}",
		FontWeight: {{ $metrics.FontWeight }},
		IsFixedPitch: {{ $metrics.IsFixedPitch }},
		IsSerif: {{ $metrics.IsSerif }},
		IsSymbolic: {{ $metrics.IsSymbolic }},
		FontBBox: rect.Rect{ LLx: {{ $metrics.FontBBox.LLx }}, LLy: {{ $metrics.FontBBox.LLy }}, URx: {{ $metrics.FontBBox.URx }}, URy: {{ $metrics.FontBBox.URy }} },
		ItalicAngle: {{ $metrics.ItalicAngle }},
		Ascent: {{ $metrics.Ascent }},
		Descent: {{ $metrics.Descent }},
		CapHeight: {{ $metrics.CapHeight }},
		XHeight: {{ $metrics.XHeight }},
		StemV: {{ $metrics.StemV }},
		StemH: {{ $metrics.StemH }},

		Width: map[string]float64{
        	{{- range $char, $width := $metrics.Widths }}
        	    "{{ $char }}": {{ $width }},
        	{{- end }}
        },

		Encoding: {{ $metrics.Encoding }},
    },
{{- end }}
}
`
