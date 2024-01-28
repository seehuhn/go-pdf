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
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
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
	FontTypeAFM
	FontTypeSfnt
)

// A FontLoader can load fonts, in case fonts are not embedded in the PDF file.
// Every FontLoader contains the 14 standard fonts required by the PDF
// specification.  Other external fonts can be added using AddFontMap and
// AddFont.
//
// It is safe to use a FontLoader concurrently from multiple goroutines.
type FontLoader struct {
	sync.RWMutex
	lookup map[key]*val
}

type key struct {
	psname   string
	fontType FontType
}

type val struct {
	fname     string
	isBuiltin bool
}

// NewFontLoader creates a new font loader.
// The loader is initialized with the 14 standard fonts required by the PDF
// specification.
func NewFontLoader() *FontLoader {
	res := &FontLoader{
		lookup: make(map[key]*val),
	}

	// There should not be any errors for the builtin fonts.
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
func (l *FontLoader) Open(postscriptName string, tp FontType) (io.ReadCloser, error) {
	key := key{postscriptName, tp}
	l.RLock()
	font, ok := l.lookup[key]
	l.RUnlock()
	if !ok {
		return nil, fs.ErrNotExist
	}

	fname := font.fname
	if font.isBuiltin {
		return builtin.Open(fname)
	}

	return os.Open(fname)
}

// AddFontMap reads a font map from r and adds it to the loader.  A font map
// consists of lines of the form
//
//	<name> <type> <path>
//
// where <name> is the PostScript name of the font, <type> is either "type1",
// "afm" or "sfnt", and <path> is the path to the font file.  The fields must
// be separated by single spaces.  Lines starting with '#' or '%' are ignored.
//
// Any previous mapping for (<name>, <type>) is overwritten.
func (l *FontLoader) AddFontMap(r io.Reader) error {
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
		case "afm":
			fontType = FontTypeAFM
		case "sfnt":
			fontType = FontTypeSfnt
		default:
			return fmt.Errorf("invalid font type %q", parts[1])
		}

		key := key{parts[0], fontType}
		l.Lock()
		l.lookup[key] = &val{fname: parts[2]}
		l.Unlock()
	}
	return lines.Err()
}

// AddFont adds a font to the loader.  Any previous mapping for the same
// PostScript name and font type is overwritten.
func (l *FontLoader) AddFont(postscriptName string, tp FontType, fname string) {
	key := key{postscriptName, tp}
	l.Lock()
	l.lookup[key] = &val{fname: fname}
	l.Unlock()
}

// builtin holds the 14 standard fonts required by the PDF specification.
//
//go:embed builtin
var builtin embed.FS
