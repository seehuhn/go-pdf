// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2021  Jochen Voss <voss@seehuhn.de>
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

package cff

// TODO(voss): post answer to
// https://stackoverflow.com/questions/18351580/is-there-any-library-for-subsetting-opentype-ps-cff-fonts
// once this is working

import (
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf/font/parser"
)

type Font struct {
	FontName string
	topDict  cffDict
	strings  []string
	gsubrs   [][]byte

	IsCIDFont bool
}

func ReadCFF(r io.ReadSeeker) (*Font, error) {
	length, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}

	p := parser.New(r)
	err = p.SetRegion("CFF", 0, length)
	if err != nil {
		return nil, err
	}
	x, err := p.ReadUInt32()
	if err != nil {
		return nil, err
	}
	major := x >> 24
	minor := (x >> 16) & 0xFF
	nameIndexPos := int64((x >> 8) & 0xFF)
	offSize := x & 0xFF // TODO(voss): what is this used for?
	if major == 2 {
		return nil, fmt.Errorf("unsupported CFF version %d.%d", major, minor)
	} else if major != 1 || nameIndexPos < 4 || offSize > 4 {
		return nil, errors.New("not a CFF font")
	}

	cff := &Font{}

	// read the Name INDEX
	err = p.SeekPos(nameIndexPos)
	if err != nil {
		return nil, err
	}
	names, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(names) != 1 {
		return nil, errors.New("CFF with multiple fonts not supported")
	}
	cff.FontName = string(names[0])

	// read the Top DICT
	topDicts, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	if len(topDicts) != 1 {
		return nil, errors.New("invalid CFF")
	}
	topDict, err := parseDict(topDicts[0])
	if err != nil {
		return nil, err
	}

	// read the String INDEX
	strings, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	cff.strings = make([]string, len(strings))
	for i, s := range strings {
		cff.strings[i] = string(s)
	}

	// read the Global Subr INDEX
	gsubrs, err := readIndex(p)
	if err != nil {
		return nil, err
	}
	cff.gsubrs = gsubrs

	for _, entry := range topDict {
		key := entry.op
		// fmt.Printf("  - 0x%04x %v\n", key, entry.args)
		switch key {
		case keyROS:
			cff.IsCIDFont = true
		case keyCharstringType:
			if len(entry.args) != 1 {
				return nil, errors.New("invalid CFF")
			}
			val, ok := entry.args[0].(int32)
			if !ok {
				return nil, errors.New("invalid CFF")
			}
			if val != 2 {
				return nil, errors.New("unsupported charstring type")
			}
			// case keyFamilyName:
			// 	fmt.Println("family name =", cff.GetString(int(entry.args[0].(int32))))
			// case keyNotice:
			// 	fmt.Println("notice =", cff.GetString(int(entry.args[0].(int32))))
			// case keyCopyright:
			// 	fmt.Println("copyright =", cff.GetString(int(entry.args[0].(int32))))
		}
	}

	return cff, nil
}

func (cff *Font) WriteCFF(w io.Writer) error {

}
