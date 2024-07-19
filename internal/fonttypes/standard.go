package fonttypes

import (
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/standard"
)

// Standard is one of the 14 standard PDF fonts.
var Standard = standardEmbedder{}

type standardEmbedder struct{}

func (f standardEmbedder) Embed(w pdf.Putter) (font.Layouter, error) {
	F, err := standard.Helvetica.New(nil)
	if err != nil {
		return nil, err
	}
	return F.Embed(w)
}
