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

package embed

import (
	"errors"
	"os"

	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/font/type1"
	"seehuhn.de/go/postscript/afm"
	pst1 "seehuhn.de/go/postscript/type1"
)

// Type1Options controls how a Type 1 font is prepared for embedding.
type Type1Options struct {
	// Variations selects an instance of a multiple master font.
	// Keys are axis names as reported by [seehuhn.de/go/postscript/type1.Font.VariationAxes].
	// This is only allowed for multiple master fonts; a nil or empty map
	// selects the font's default instance.
	Variations map[string]float64
}

// Type1File loads and embeds a Type 1 font.
// The file `psname` can be either an .pfb or .pfa file.
// The file `afmname` is the corresponding .afm file.
// Both `psname` and `afmname` are optional, but at least one of them must be given.
// See Type1Options and Type1Font for multiple-master font instancing behavior.
func Type1File(psname, afmname string, opt *Type1Options) (font.Layouter, error) {
	var psFont *pst1.Font
	var metrics *afm.Metrics
	if psname != "" {
		fd, err := os.Open(psname)
		if err != nil {
			return nil, err
		}
		psFont, err = pst1.Read(fd)
		if err != nil {
			fd.Close()
			return nil, err
		}
		err = fd.Close()
		if err != nil {
			return nil, err
		}
	}
	if afmname != "" {
		fd, err := os.Open(afmname)
		if err != nil {
			return nil, err
		}
		metrics, err = afm.Read(fd)
		if err != nil {
			fd.Close()
			return nil, err
		}
		err = fd.Close()
		if err != nil {
			return nil, err
		}
	}
	return Type1Font(psFont, metrics, opt)
}

// Type1Font embeds a Type 1 font.
// The `psFont` and `metrics` parameters are optional, but at least one of them must be given.
//
// If psFont is a multiple master font, it is always instantiated before
// embedding: at the coordinates given by opt.Variations, or at the font's
// default instance if opt is nil or opt.Variations is empty. Variations may
// only be given for multiple master fonts. Metrics may only be combined
// with a non-empty Variations selection if metrics describe that same
// instance; since AFM metrics cannot be verified against an arbitrary
// instance, this combination is rejected.
func Type1Font(psFont *pst1.Font, metrics *afm.Metrics, opt *Type1Options) (font.Layouter, error) {
	var variations map[string]float64
	if opt != nil {
		variations = opt.Variations
	}

	if psFont != nil && psFont.MM != nil {
		if len(variations) != 0 && metrics != nil {
			return nil, errors.New("AFM metrics do not describe the requested instance")
		}
		instance, err := psFont.Instantiate(variations)
		if err != nil {
			return nil, err
		}
		psFont = instance
	} else if len(variations) != 0 {
		return nil, errors.New("font is not a multiple master font")
	}

	return type1.New(psFont, metrics)
}
