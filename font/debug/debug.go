package debug

import (
	"bytes"

	"golang.org/x/image/font/gofont/goregular"
	"seehuhn.de/go/pdf/font/sfntcff"
)

// Build a font for use in unit tests.
func Build() (*sfntcff.Info, error) {
	info, err := sfntcff.Read(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}

	// See the following link for converting truetype outlines to CFF outlines:
	// https://pomax.github.io/bezierinfo/#reordering

	return info, nil
}
