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

package gofont

import (
	"bytes"
	"errors"
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

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/truetype"
	"seehuhn.de/go/sfnt"
)

// Font identifies individual fonts in the Go font family.
type Font int

// Constants for the available fonts in the Go font family.
const (
	_               Font = iota
	Bold                 // Go Semi Bold
	BoldItalic           // Go Semi Bold Italic
	Italic               // Go Italic
	Medium               // Go Medium Regular
	MediumItalic         // Go Medium Italic
	Regular              // Go Regular
	Smallcaps            // Go Smallcaps Regular
	SmallcapsItalic      // Go Smallcaps Italic
	Mono                 // Go Mono Regular
	MonoBold             // Go Mono Semi Bold
	MonoBoldItalic       // Go Mono Semi Bold Italic
	MonoItalic           // Go Mono Italic
)

// New returns a new font instance for the given Go font.
func (f Font) New(opt *font.Options) *truetype.Instance {
	data, ok := ttf[f]
	if !ok {
		panic("invalid Go font ID")
	}

	info, err := sfnt.Read(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Sprintf("built-in fonts corrupted??? %s", err))
	}

	F, err := truetype.New(info, opt)
	if err != nil {
		panic(fmt.Sprintf("built-in fonts corrupted??? %s", err))
	}

	return F
}

// All contains all available fonts in the Go font family.
var All = []Font{
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

// ErrInvalidFontID indicates that a FontID is invalid.
var ErrInvalidFontID = errors.New("invalid font ID")
