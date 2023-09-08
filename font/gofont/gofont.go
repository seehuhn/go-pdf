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
	"errors"

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
)

// FontID identifies individual fonts in the Go font family.
type FontID int

// Constants for the available fonts in the Go font family.
const (
	_                 FontID = iota
	GoBold                   // gobold
	GoBoldItalic             // gobolditalic
	GoItalic                 // goitalic
	GoMedium                 // gomedium
	GoMediumItalic           // gomediumitalic
	GoRegular                // goregular
	GoSmallcaps              // gosmallcaps
	GoSmallcapsItalic        // gosmallcapsitalic
	GoMono                   // gomono
	GoMonoBold               // gomonobold
	GoMonoBoldItalic         // gomonobolditalic
	GoMonoItalic             // gomonoitalic
)

// All is a slice containing all available fonts in the Go font family.
var All = []FontID{
	GoBold,
	GoBoldItalic,
	GoItalic,
	GoMedium,
	GoMediumItalic,
	GoRegular,
	GoSmallcaps,
	GoSmallcapsItalic,
	GoMono,
	GoMonoBold,
	GoMonoBoldItalic,
	GoMonoItalic,
}

var ttf = map[FontID][]byte{
	GoBold:            gobold.TTF,
	GoBoldItalic:      gobolditalic.TTF,
	GoItalic:          goitalic.TTF,
	GoMedium:          gomedium.TTF,
	GoMediumItalic:    gomediumitalic.TTF,
	GoRegular:         goregular.TTF,
	GoSmallcaps:       gosmallcaps.TTF,
	GoSmallcapsItalic: gosmallcapsitalic.TTF,
	GoMono:            gomono.TTF,
	GoMonoBold:        gomonobold.TTF,
	GoMonoBoldItalic:  gomonobolditalic.TTF,
	GoMonoItalic:      gomonoitalic.TTF,
}

// ErrInvalidFontID indicates that a FontID is invalid.
var ErrInvalidFontID = errors.New("invalid font ID")
