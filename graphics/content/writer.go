// Copyright 2025 Jochen Voss <voss@seehuhn.de>
// SPDX-License-Identifier: BSD-3-Clause

package content

import (
	"io"
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/imagedict"
)

// Format writes op's PDF representation to out (its args followed by the
// operator name, with the appropriate separators).  The pseudo-operators
// [OpRawContent] and [OpInlineImage] have dedicated handling.
//
// An inline image is written with a Length entry, which is added if the
// image dictionary does not give one.  An entry which is given must equal
// the length of the image data, and must be given only once, as either L or
// Length; otherwise Format returns an error.
//
// The output options opt should be those of the PDF file the content is
// written to (see [pdf.Writer.GetOptions]); they select version-dependent
// syntax, such as whether names may use #xx escapes.  Zero gives PDF 1.2 and
// later syntax.
func (op Operator) Format(out io.Writer, opt pdf.OutputOptions) error {
	opt = opt&^pdf.OptPretty | pdf.OptContentStream

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

			// With a Length entry, a reader finds the end of the data even
			// where the data contains an EI marker.
			if err := ValidateInlineImageLength(dict, len(data)); err != nil {
				return err
			}
			if _, ok := imagedict.InlineKey(dict, "Length"); !ok {
				dict = maps.Clone(dict)
				if dict == nil {
					dict = pdf.Dict{}
				}
				dict["L"] = pdf.Integer(len(data))
			}

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
				if err := pdf.Format(out, opt, key); err != nil {
					return err
				}
				if _, err := out.Write([]byte(" ")); err != nil {
					return err
				}
				if natVal, ok := val.(pdf.Native); ok {
					if err := pdf.Format(out, opt, natVal); err != nil {
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
		if err := pdf.Format(out, opt, arg); err != nil {
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
