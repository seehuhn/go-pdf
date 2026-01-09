// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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
	"fmt"
	"os"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/document"
	"seehuhn.de/go/pdf/graphics/text"
	"seehuhn.de/go/pdf/internal/fonttypes"
	"seehuhn.de/go/pdf/internal/gibberish"
)

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	for _, sample := range fonttypes.All {
		out, err := document.CreateSinglePage("test"+sample.Label+".pdf", document.A5r, pdf.V2_0, nil)
		if err != nil {
			return err
		}

		F := text.F{
			Font: sample.MakeFont(),
			Size: 15,
		}

		text.Show(out.Builder,
			text.M{X: 36, Y: 360}, F,
			text.Wrap(480, gibberish.Generate(100, 0)),
			text.NL,
			text.Wrap(480, gibberish.Generate(80, 1)),
		)

		err = out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
