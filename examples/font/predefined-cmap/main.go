// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

// This program sets Japanese text using a predefined CMap.  It writes
// test.pdf.
//
// A composite font normally uses the Identity encoding, where a character
// code is the number of a glyph in the embedded font program.  A predefined
// CMap instead encodes text in a registered character collection, so the
// codes say which characters are meant rather than which glyphs.  A consumer
// can then recover the text without consulting the font, which is why no
// ToUnicode CMap is written.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"

	"seehuhn.de/go/sfnt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/cmap"
	"seehuhn.de/go/pdf/font/opentype"
)

const (
	fontURL    = "https://cdn.jsdelivr.net/gh/google/fonts@7b203a635ebe80801c80f29633d4fc467cd1214e/ofl/notosansjp/NotoSansJP-Regular.otf"
	fontFile   = "font.otf"
	outputFile = "test.pdf"

	// UniJIS-UCS2-H maps UTF-16BE character codes to CIDs of the
	// Adobe-Japan1 character collection, for horizontal writing.
	cmapName = "UniJIS-UCS2-H"
)

const (
	fontSize = 24.0
	leading  = 38.0
	margin   = 56.0
)

// the Iroha, a poem which uses every kana of the classical syllabary once
var poem = []string{
	"いろはにほへと　ちりぬるを",
	"わかよたれそ　つねならむ",
	"うゐのおくやま　けふこえて",
	"あさきゆめみし　ゑひもせす",
}

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	err := downloadFontIfNeeded()
	if err != nil {
		return err
	}

	info, err := sfnt.ReadFile(fontFile)
	if err != nil {
		return err
	}

	encoding, err := cmap.Predefined(cmapName)
	if err != nil {
		return err
	}
	fmt.Printf("%s encodes text as %s-%s-%d, %s\n", encoding.Name,
		encoding.ROS.Registry, encoding.ROS.Ordering, encoding.ROS.Supplement,
		encoding.WMode)

	F, err := opentype.NewComposite(info, &opentype.OptionsComposite{CMap: encoding})
	if err != nil {
		return err
	}

	paper := &pdf.Rectangle{
		URx: 2*margin + textWidth(F, poem),
		URy: 2*margin + leading*float64(len(poem)-1) + fontSize,
	}
	page, err := document.CreateSinglePage(outputFile, paper, pdf.V2_0, nil)
	if err != nil {
		return err
	}

	page.TextBegin()
	page.TextSetFont(F, fontSize)
	page.TextSetLeading(leading)
	page.TextFirstLine(margin, paper.URy-margin-fontSize)
	for i, line := range poem {
		if i > 0 {
			page.TextNextLine()
		}
		page.TextShow(line)
	}
	page.TextEnd()

	return page.Close()
}

func textWidth(F font.Layouter, lines []string) float64 {
	var width float64
	for _, line := range lines {
		width = max(width, F.Layout(nil, fontSize, line).TotalWidth())
	}
	return width
}

func downloadFontIfNeeded() error {
	if _, err := os.Stat(fontFile); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	resp, err := http.Get(fontURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", fontURL, resp.Status)
	}

	// Write to a temporary file first, so that an interrupted download
	// cannot leave a truncated font behind.
	tmp, err := os.CreateTemp(".", fontFile+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	_, err = io.Copy(tmp, resp.Body)
	if err != nil {
		tmp.Close()
		return err
	}
	err = tmp.Close()
	if err != nil {
		return err
	}

	return os.Rename(tmp.Name(), fontFile)
}
