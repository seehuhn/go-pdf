// Copyright 2024 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package loader

import (
	"bufio"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
)

// FontType is the type of a font.
type FontType int

// Supported font types.
const (
	FontTypeType1 FontType = iota + 1
	FontTypeAFM
	FontTypeSfnt
)

// A FontLoader can load fonts, in case fonts are not embedded in the PDF file.
// Every FontLoader contains the bundled fonts: AFM metrics and sfnt outlines
// for each of the 14 standard font names required by the PDF specification,
// and the Quire fonts which provide those outlines.  Other external fonts can
// be added using AddFontMap and AddFont.
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
// The loader is initialized with the bundled fonts, see [FontLoader].
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
		return openFS(builtin, fname)
	}

	return os.Open(fname)
}

// IsBuiltin reports whether the loader's entry for the given PostScript name
// and font type is a bundled font, as opposed to one added by the caller.  It
// returns false if there is no such entry.
func (l *FontLoader) IsBuiltin(postscriptName string, tp FontType) bool {
	l.RLock()
	font, ok := l.lookup[key{postscriptName, tp}]
	l.RUnlock()
	return ok && font.isBuiltin
}

// openFS opens a file from fsys.  Files with the extension ".gz" are
// decompressed on the fly.
func openFS(fsys fs.FS, fname string) (io.ReadCloser, error) {
	f, err := fsys.Open(fname)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(fname, ".gz") {
		return f, nil
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &gzipReadCloser{zr: zr, f: f}, nil
}

// gzipReadCloser closes both the decompressor and the underlying file.
type gzipReadCloser struct {
	zr *gzip.Reader
	f  fs.File
}

func (g *gzipReadCloser) Read(p []byte) (int, error) {
	return g.zr.Read(p)
}

func (g *gzipReadCloser) Close() error {
	err1 := g.zr.Close()
	err2 := g.f.Close()
	if err1 != nil {
		return err1
	}
	return err2
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

// builtin holds the bundled font files: the Adobe AFM metrics of the 14
// standard fonts and the Quire sfnt fonts.
//
//go:embed builtin
var builtin embed.FS
