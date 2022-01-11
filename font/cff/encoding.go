package cff

import (
	"fmt"

	"seehuhn.de/go/pdf/font/parser"
)

type encoding struct{}

func (cff *Font) readEncoding(p *parser.Parser) (*encoding, error) {
	format, err := p.ReadUInt8()
	if err != nil {
		return nil, err
	}

	supplement := format&128 != 0
	format &= 127

	switch format {
	case 0:
		nCodes, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		data, err := p.ReadBlob(int(nCodes))
		if err != nil {
			return nil, err
		}
		for i, c := range data {
			fmt.Println(c, "->", cff.GlyphName[i+1])
		}
	case 1:
		fmt.Println("format 1")
	default:
		return nil, fmt.Errorf("unsupported encoding format %d", format)
	}

	if supplement {
		nSups, err := p.ReadUInt8()
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(nSups); i++ {

		}
	}

	return nil, nil
}
