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

package content

import (
	"io"
	"slices"

	"seehuhn.de/go/pdf"
)

// Format writes op's PDF representation to out (its args followed by the
// operator name, with the appropriate separators).  The pseudo-operators
// [OpRawContent] and [OpInlineImage] have dedicated handling.
func (op Operator) Format(out io.Writer) error {
	// Handle special pseudo-operators
	switch op.Name {
	case OpRawContent:
		if len(op.Args) > 0 {
			if str, ok := op.Args[0].(pdf.String); ok {
				if _, err := out.Write([]byte(str)); err != nil {
					return err
				}
				if _, err := out.Write([]byte("\n")); err != nil {
					return err
				}
			}
		}
		return nil
	case OpInlineImage:
		if len(op.Args) >= 2 {
			dict, _ := op.Args[0].(pdf.Dict)
			data, _ := op.Args[1].(pdf.String)

			if _, err := out.Write([]byte("BI\n")); err != nil {
				return err
			}
			// sort keys for deterministic output
			keys := make([]pdf.Name, 0, len(dict))
			for key := range dict {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				val := dict[key]
				if _, err := out.Write([]byte("/")); err != nil {
					return err
				}
				if _, err := out.Write([]byte(key)); err != nil {
					return err
				}
				if _, err := out.Write([]byte(" ")); err != nil {
					return err
				}
				if natVal, ok := val.(pdf.Native); ok {
					if err := pdf.Format(out, pdf.OptContentStream, natVal); err != nil {
						return err
					}
				}
				if _, err := out.Write([]byte("\n")); err != nil {
					return err
				}
			}
			if _, err := out.Write([]byte("ID\n")); err != nil {
				return err
			}
			if _, err := out.Write([]byte(data)); err != nil {
				return err
			}
			if _, err := out.Write([]byte("\nEI\n")); err != nil {
				return err
			}
		}
		return nil
	}

	// Write arguments
	for _, arg := range op.Args {
		if err := pdf.Format(out, pdf.OptContentStream, arg); err != nil {
			return err
		}
		if _, err := out.Write([]byte(" ")); err != nil {
			return err
		}
	}

	// Write operator name
	if _, err := out.Write([]byte(op.Name)); err != nil {
		return err
	}
	if _, err := out.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}
