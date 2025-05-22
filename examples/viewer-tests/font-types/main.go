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

		text.Show(out.Writer,
			text.M{X: 36, Y: 360}, F,
			text.Wrap(500, `In the bustling metropolis of Docuville, where words flowed like rivers and fonts flourished in every corner, there lived a brave hero named Go-Pdf. With his trusty sidekick, Vector Graphics, Go-Pdf was known throughout the land for his ability to render crisp, clean documents with lightning speed. The citizens of Docuville relied on Go-Pdf to protect their precious files from the clutches of the dreaded Font Rasterizer, a monstrous villain who sought to pixelate and blur the beauty of their beloved typefaces.`),
			text.NL,
			text.Wrap(500, `One fateful day, the Font Rasterizer launched a surprise attack on the city's central printing press, threatening to convert every letter into a jagged, low-resolution mess. Go-Pdf leapt into action, harnessing the power of his vector-based superpowers to combat the Font Rasterizer's bitmap-based brutality. With Vector Graphics at his side, Go-Pdf engaged in an epic battle, dodging pixelated projectiles and countering with sleek, scalable strokes. In the end, Go-Pdf emerged victorious, banishing the Font Rasterizer to the depths of the recycle bin and restoring clarity and legibility to the documents of Docuville once more.`),
		)

		err = out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
