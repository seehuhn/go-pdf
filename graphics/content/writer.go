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
	"fmt"
	"io"
	"slices"

	"seehuhn.de/go/pdf"
)

// Writer outputs content streams with version checking.
type Writer struct {
	*State
	version pdf.Version
	res     *Resources
}

// NewWriter creates a Writer for the given content type and PDF version.
func NewWriter(v pdf.Version, ct Type, res *Resources) *Writer {
	return &Writer{
		State:   NewState(ct),
		version: v,
		res:     res,
	}
}

// Write outputs a stream, validating and checking version compatibility.
func (w *Writer) Write(out io.Writer, s Stream) error {
	for i, op := range s {
		// Check version compatibility.
		// Unknown operators are allowed inside BX/EX compatibility sections.
		if err := op.isValidName(w.version); err != nil {
			if !((err == ErrUnknown || err == ErrVersion) && w.InCompatibilitySection()) {
				return fmt.Errorf("operator %d (%s): %w", i, op.Name, err)
			}
		}

		// Update state based on operator
		if err := w.applyOperator(op); err != nil {
			return fmt.Errorf("operator %d (%s): %w", i, op.Name, err)
		}

		// Write the operator
		if err := writeOperator(out, op); err != nil {
			return err
		}
	}
	return nil
}

// Close checks for balanced operators and version-specific stack depth limits.
func (w *Writer) Close() error {
	if err := w.State.CanClose(); err != nil {
		return err
	}

	// PDF 1.7 and earlier: max stack depth is 28
	if w.version < pdf.V2_0 && w.MaxStackDepth > 28 {
		return fmt.Errorf("stack depth %d exceeds limit of 28 for PDF %s",
			w.MaxStackDepth, w.version)
	}

	return nil
}

// applyOperator updates Writer state for the given operator.
func (w *Writer) applyOperator(op Operator) error {
	if err := w.State.CheckOperatorAllowed(op.Name); err != nil {
		return err
	}

	// handle state-modifying operators
	var err error
	switch op.Name {
	case OpPushGraphicsState:
		err = w.Push()
	case OpPopGraphicsState:
		err = w.Pop()
	case OpTextBegin:
		err = w.TextBegin()
	case OpTextEnd:
		err = w.TextEnd()
	case OpBeginMarkedContent, OpBeginMarkedContentWithProperties:
		w.MarkedContentBegin()
	case OpEndMarkedContent:
		err = w.MarkedContentEnd()
	case OpBeginCompatibility:
		w.CompatibilityBegin()
	case OpEndCompatibility:
		err = w.CompatibilityEnd()
	case OpType3ColoredGlyph:
		err = w.State.GlyphColored()
	case OpType3UncoloredGlyph:
		err = w.State.GlyphUncolored()
	}
	if err != nil {
		return err
	}

	// apply object state transitions for path/clipping operators
	w.State.ApplyTransition(op.Name)

	// validate resource references
	if w.res != nil {
		if err := w.validateResources(op); err != nil {
			return err
		}
	}

	return nil
}

// validateResources checks that resources referenced by the operator exist.
func (w *Writer) validateResources(op Operator) error {
	switch op.Name {
	case OpTextSetFont: // Tf name size
		if len(op.Args) >= 1 {
			if name, ok := op.Args[0].(pdf.Name); ok {
				if _, exists := w.res.Font[name]; !exists {
					return fmt.Errorf("font %q not in resources", name)
				}
			}
		}

	case OpXObject: // Do name
		if len(op.Args) >= 1 {
			if name, ok := op.Args[0].(pdf.Name); ok {
				if _, exists := w.res.XObject[name]; !exists {
					return fmt.Errorf("XObject %q not in resources", name)
				}
			}
		}

	case OpSetExtGState: // gs name
		if len(op.Args) >= 1 {
			if name, ok := op.Args[0].(pdf.Name); ok {
				if _, exists := w.res.ExtGState[name]; !exists {
					return fmt.Errorf("ExtGState %q not in resources", name)
				}
			}
		}

	case OpShading: // sh name
		if len(op.Args) >= 1 {
			if name, ok := op.Args[0].(pdf.Name); ok {
				if _, exists := w.res.Shading[name]; !exists {
					return fmt.Errorf("shading %q not in resources", name)
				}
			}
		}

	case OpSetStrokeColorSpace, OpSetFillColorSpace: // CS/cs name
		if len(op.Args) >= 1 {
			if name, ok := op.Args[0].(pdf.Name); ok {
				// device color spaces are built-in, not in resources
				if !isDeviceColorSpace(name) {
					if _, exists := w.res.ColorSpace[name]; !exists {
						return fmt.Errorf("color space %q not in resources", name)
					}
				}
			}
		}

	case OpSetStrokeColorN, OpSetFillColorN: // SCN/scn c1...cn [name]
		// pattern names are pdf.Name, color components are numbers
		if len(op.Args) > 0 {
			if name, ok := op.Args[len(op.Args)-1].(pdf.Name); ok {
				if _, exists := w.res.Pattern[name]; !exists {
					return fmt.Errorf("pattern %q not in resources", name)
				}
			}
		}
	}
	return nil
}

// isDeviceColorSpace returns true for built-in device color spaces.
func isDeviceColorSpace(name pdf.Name) bool {
	switch name {
	case "DeviceGray", "DeviceRGB", "DeviceCMYK":
		return true
	}
	return false
}

// writeOperator writes a single operator to the output.
func writeOperator(out io.Writer, op Operator) error {
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
