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

package loader

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// https://learn.microsoft.com/en-us/typography/opentype/spec/name#nid6
// the name string must be no longer than 63 characters and restricted to the
// printable ASCII subset, codes 33 to 126, except for the 10 characters '[',
// ']', '(', ')', '{', '}', '<', '>', '/', '%'.

// FontType is the type of a font.
type FontType int

// Supported font types.
const (
	FontTypeType1 FontType = iota + 1
	FontTypeSfnt
)

type Loader struct {
	lookup map[string]*font
}

type font struct {
	fname     string
	fontType  FontType
	isBuiltin bool
}

// New creates a new font loader.
// The loader is initialized with the 14 standard fonts required by the PDF
// specification.
func New() *Loader {
	res := &Loader{
		lookup: make(map[string]*font),
	}

	defaultMap, err := builtin.Open("builtin/font.map")
	if err != nil {
		panic(err)
	}
	err = res.AddFontMap(defaultMap)
	if err != nil {
		panic(err)
	}
	err = defaultMap.Close()
	if err != nil {
		panic(err)
	}

	for _, font := range res.lookup {
		font.isBuiltin = true
	}

	return res
}

// Open opens the font with the given PostScript name.  The returned
// io.ReadCloser must be closed by the caller.
func (l *Loader) Open(postscriptName string) (FontType, io.ReadCloser, error) {
	font, ok := l.lookup[postscriptName]
	if !ok {
		return 0, nil, ErrNotFound
	}

	fname := font.fname
	if font.isBuiltin {
		r, err := builtin.Open(fname)
		return font.fontType, r, err
	}

	r, err := os.Open(fname)
	return font.fontType, r, err
}

// AddFontMap reads a font map from r and adds it to the loader. The format of
// the font map is:
//
//	<name> <type> <path>
//
// where <name> is the PostScript name of the font, <type> is either "type1" or
// "sfnt", and <path> is the path to the font file.  The fields must be
// separated by a single spaces.  Lines starting with '#' or '%' are ignored.
//
// Any previous mapping for <name> is overwritten.
func (l *Loader) AddFontMap(r io.Reader) error {
	lines := bufio.NewScanner(r)
	for lines.Scan() {
		line := lines.Text()
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' || line[0] == '%' {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			return fmt.Errorf("invalid font map line: %q", line)
		}
		var fontType FontType
		switch parts[1] {
		case "type1":
			fontType = FontTypeType1
		case "sfnt":
			fontType = FontTypeSfnt
		default:
			return fmt.Errorf("invalid font type %q", parts[1])
		}

		l.lookup[parts[0]] = &font{
			fname:    parts[2],
			fontType: fontType,
		}
	}
	return lines.Err()
}

// ErrNotFound is returned by Open if the font is not known to the loader.
var ErrNotFound = errors.New("unknown font")

// builtin holds the 14 standard fonts required by the PDF specification.
//
//go:embed builtin
var builtin embed.FS
