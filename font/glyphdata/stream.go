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

package glyphdata

import (
	"bytes"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 9.9

// Stream represents a font file stream that can be embedded in a PDF file.
//
// The Type field specifies the font file format (Type1, TrueType, CFF, etc.)
// and the WriteTo function writes the actual font file data.
type Stream struct {
	// Type specifies the font file format.
	Type Type

	// WriteTo writes the font file data to w. If length is non-nil and the
	// font type requires length information (Type1 or TrueType), the function
	// should populate the corresponding length fields.
	WriteTo func(w io.Writer, length *Lengths) error
}

// Lengths holds the length values required for Type1 and TrueType font streams.
type Lengths struct {
	// Length1 is the clear text portion length for Type1, or total length for
	// TrueType.
	Length1 pdf.Integer

	// Length2 is the encrypted portion length for Type1 fonts only.
	Length2 pdf.Integer

	// Length3 is the fixed portion length for Type1 fonts.
	Length3 pdf.Integer
}

var _ pdf.Embedder[pdf.Unused] = (*Stream)(nil)

// ExtractStream extracts a font file stream from a PDF file.
//
// The dictType parameter specifies the font dictionary type (e.g., "Type1", "TrueType", "Type0").
// The fdKey parameter specifies the font descriptor key ("FontFile", "FontFile2" or "FontFile3").
func ExtractStream(x *pdf.Extractor, obj pdf.Object, dictType, fdKey pdf.Name) (*Stream, error) {
	stm, err := pdf.Optional(pdf.GetStream(x.R, obj))
	if err != nil {
		return nil, err
	} else if stm == nil {
		return nil, nil
	}

	tp, err := determineType(dictType, fdKey, stm.Dict)
	if err != nil {
		return nil, err
	}

	length1, err := pdf.Optional(pdf.GetInteger(nil, stm.Dict["Length1"]))
	if err != nil {
		return nil, err
	}
	length2, err := pdf.Optional(pdf.GetInteger(nil, stm.Dict["Length2"]))
	if err != nil {
		return nil, err
	}
	length3, err := pdf.Optional(pdf.GetInteger(nil, stm.Dict["Length3"]))
	if err != nil {
		return nil, err
	}

	return &Stream{
		Type: tp,
		WriteTo: func(w io.Writer, length *Lengths) error {
			body, err := pdf.DecodeStream(x.R, stm, 0)
			if err != nil {
				return err
			}
			defer body.Close()

			if length != nil {
				length.Length1 = length1
				length.Length2 = length2
				length.Length3 = length3
			}

			_, err = io.Copy(w, body)
			return err
		},
	}, nil
}

// Embed adds the font file stream to a PDF file.
//
// This method implements the pdf.Embedder interface and handles all necessary
// PDF version checks and stream formatting based on the font type.
func (s *Stream) Embed(rm *pdf.EmbedHelper) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	switch s.Type {
	case Type1:
		// pass
	case TrueType:
		if err := pdf.CheckVersion(rm.Out(), "TrueType font file", pdf.V1_1); err != nil {
			return nil, zero, err
		}
	case CFFSimple:
		if err := pdf.CheckVersion(rm.Out(), "CFF font file", pdf.V1_2); err != nil {
			return nil, zero, err
		}
	case CFF:
		if err := pdf.CheckVersion(rm.Out(), "CID-keyed CFF font file", pdf.V1_3); err != nil {
			return nil, zero, err
		}
	case OpenTypeCFFSimple, OpenTypeCFF, OpenTypeGlyf:
		if err := pdf.CheckVersion(rm.Out(), "OpenType font file", pdf.V1_6); err != nil {
			return nil, zero, err
		}
	default:
		return nil, zero, pdf.Errorf("unexpected font type %s", s.Type)
	}

	ref := rm.Alloc()
	dict := pdf.Dict{}

	if subtype := s.Type.subtype(); subtype != "" {
		dict["Subtype"] = subtype
	}

	stm, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}

	needsLengths := s.Type == Type1 || s.Type == TrueType
	if needsLengths {
		var lengths Lengths
		var l1, l2, l3 *pdf.Placeholder

		l1 = pdf.NewPlaceholder(rm.Out(), 10)
		dict["Length1"] = l1
		if s.Type == Type1 {
			l2 = pdf.NewPlaceholder(rm.Out(), 10)
			dict["Length2"] = l2
			l3 = pdf.NewPlaceholder(rm.Out(), 10)
			dict["Length3"] = l3
		}

		err = s.WriteTo(stm, &lengths)
		if err != nil {
			stm.Close()
			return nil, zero, err
		}

		err = l1.Set(pdf.Integer(lengths.Length1))
		if err != nil {
			stm.Close()
			return nil, zero, err
		}
		if s.Type == Type1 {
			err = l2.Set(pdf.Integer(lengths.Length2))
			if err != nil {
				stm.Close()
				return nil, zero, err
			}
			err = l3.Set(pdf.Integer(lengths.Length3))
			if err != nil {
				stm.Close()
				return nil, zero, err
			}
		}
	} else {
		err = s.WriteTo(stm, nil)
		if err != nil {
			stm.Close()
			return nil, zero, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// determineType determines the glyphdata.Type from the font dictionary type,
// font descriptor key, and stream dictionary.
func determineType(dictType, fdKey pdf.Name, streamDict pdf.Dict) (Type, error) {
	subtype, err := pdf.Optional(pdf.GetName(nil, streamDict["Subtype"]))
	if err != nil {
		return 0, err
	}

	switch fdKey {
	case "FontFile":
		return Type1, nil

	case "FontFile2":
		return TrueType, nil

	case "FontFile3":
		switch subtype {
		case "Type1C":
			return CFFSimple, nil
		case "CIDFontType0C":
			return CFF, nil
		case "OpenType":
			switch dictType {
			case "Type1":
				return OpenTypeCFFSimple, nil
			case "TrueType":
				return OpenTypeGlyf, nil
			case "Type0":
				return OpenTypeCFF, nil
			default:
				return 0, fmt.Errorf("unexpected font dict type %q", dictType)
			}
		default:
			return 0, pdf.Errorf("unexpected FontFile3 subtype %q", subtype)
		}

	default:
		return 0, fmt.Errorf("unexpected font descriptor key %q", fdKey)
	}
}

// Equal returns true if two Stream objects are equivalent.
// This compares the Type field and the output of WriteTo function calls,
// including length information for applicable font types.
//
// This is primarily useful for testing purposes.  For normal usage,
// equal font file streams should be represented by equal pointer values.
func (s *Stream) Equal(other *Stream) bool {
	if s == nil && other == nil {
		return true
	}
	if s == nil || other == nil {
		return false
	}

	if s.Type != other.Type {
		return false
	}

	// Compare the WriteTo functions by capturing and comparing their output.
	var sBuf, otherBuf bytes.Buffer
	var sLengths, otherLengths Lengths
	sErr := s.WriteTo(&sBuf, &sLengths)
	otherErr := other.WriteTo(&otherBuf, &otherLengths)
	if sErr != otherErr {
		return false
	}

	if !bytes.Equal(sBuf.Bytes(), otherBuf.Bytes()) {
		return false
	}

	if sLengths.Length1 != otherLengths.Length1 ||
		sLengths.Length2 != otherLengths.Length2 ||
		sLengths.Length3 != otherLengths.Length3 {
		return false
	}

	return true
}
