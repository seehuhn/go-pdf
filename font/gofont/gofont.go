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

// Package gofont provides access to the Go font family.
package gofont

import (
	"bytes"
	"fmt"

	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomediumitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/gofont/gosmallcaps"
	"golang.org/x/image/font/gofont/gosmallcapsitalic"

	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/sfnt"
)

// TODO(voss): Make it easier to use the alterative zeros provided in the
// last two glyphs of the Go fonts.

// Font identifies individual fonts in the Go font family.
type Font int

// Constants for the available fonts in the Go font family.
const (
	Regular         Font = iota // Go Regular
	Bold                        // Go Semi Bold
	BoldItalic                  // Go Semi Bold Italic
	Italic                      // Go Italic
	Medium                      // Go Medium Regular
	MediumItalic                // Go Medium Italic
	Smallcaps                   // Go Smallcaps Regular
	SmallcapsItalic             // Go Smallcaps Italic
	Mono                        // Go Mono Regular
	MonoBold                    // Go Mono Semi Bold
	MonoBoldItalic              // Go Mono Semi Bold Italic
	MonoItalic                  // Go Mono Italic
)

// New returns a new font instance for the given Go font and options.
func (f Font) New(opt *truetype.Options) (*truetype.Instance, error) {
	data, ok := ttf[f]
	if !ok {
		return nil, fmt.Errorf("gofont: unknown font %d", f)
	}

	info, err := sfnt.Read(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gofont: %w", err)
	}

	F, err := truetype.New(info, opt)
	if err != nil {
		return nil, fmt.Errorf("gofont: %w", err)
	}

	return F, nil
}

var ttf = map[Font][]byte{
	Bold:            gobold.TTF,
	BoldItalic:      gobolditalic.TTF,
	Italic:          goitalic.TTF,
	Medium:          gomedium.TTF,
	MediumItalic:    gomediumitalic.TTF,
	Regular:         goregular.TTF,
	Smallcaps:       gosmallcaps.TTF,
	SmallcapsItalic: gosmallcapsitalic.TTF,
	Mono:            gomono.TTF,
	MonoBold:        gomonobold.TTF,
	MonoBoldItalic:  gomonobolditalic.TTF,
	MonoItalic:      gomonoitalic.TTF,
}

// All contains all the Go font family fonts available in this package.
var All = allGoFonts

var allGoFonts = []Font{
	Bold,
	BoldItalic,
	Italic,
	Medium,
	MediumItalic,
	Regular,
	Smallcaps,
	SmallcapsItalic,
	Mono,
	MonoBold,
	MonoBoldItalic,
	MonoItalic,
}

// Gopher is the Unicode code point for the gopher symbol in the Go fonts.
const Gopher = '\uF800'
