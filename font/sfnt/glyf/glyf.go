package glyf

import (
	"fmt"

	"seehuhn.de/go/pdf/font"
)

type Glyph struct {
	data []byte
}

// Decode converts the data from the "glyf" and "loca" tables into
// a slice of Glyphs.
func Decode(glyfData, locaData []byte, locaFormat int16) ([]*Glyph, error) {
	offs, err := decodeLoca(glyfData, locaData, locaFormat)
	if err != nil {
		return nil, err
	}

	numGlyphs := len(offs) - 1

	gg := make([]*Glyph, numGlyphs)
	for i := range gg {
		data := glyfData[offs[i]:offs[i+1]]
		gg[i] = &Glyph{data: data}
	}

	return gg, nil
}

func decodeLoca(glyfData, locaData []byte, locaFormat int16) ([]int, error) {
	var offs []int
	switch locaFormat {
	case 0:
		n := len(locaData)
		if n < 4 || n%2 != 0 {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/loca",
				Reason:    "invalid table length",
			}
		}
		offs = make([]int, n/2-1)
		prev := 0
		for i := range offs {
			x := int(locaData[2*i])<<8 + int(locaData[2*i+1])
			pos := 2 * x
			if pos < prev || pos > len(glyfData) {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/loca",
					Reason:    fmt.Sprintf("invalid offset %d", pos),
				}
			}
			offs[i] = pos
			prev = pos
		}
	case 1:
		n := len(locaData)
		if n < 8 || n%4 != 0 {
			return nil, &font.InvalidFontError{
				SubSystem: "sfnt/loca",
				Reason:    "invalid table length",
			}
		}
		offs = make([]int, len(locaData)/4-1)
		prev := 0
		for i := range offs {
			pos := int(locaData[4*i])<<24 + int(locaData[4*i+1])<<16 +
				int(locaData[4*i+2])<<8 + int(locaData[4*i+3])
			if pos < prev || pos > len(glyfData) {
				return nil, &font.InvalidFontError{
					SubSystem: "sfnt/loca",
					Reason:    "invalid offset",
				}
			}
			offs[i] = pos
			prev = pos
		}
	default:
		return nil, &font.NotSupportedError{
			SubSystem: "sfnt/loca",
			Feature:   fmt.Sprintf("loca table format %d", locaFormat),
		}
	}
	return offs, nil
}
