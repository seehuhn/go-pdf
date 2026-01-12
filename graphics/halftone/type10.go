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

package halftone

import (
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics"
)

// PDF 2.0 sections: 10.6.4 10.6.5.1 10.6.5.4

// Type10 represents a Type 10 halftone that uses angled threshold arrays
// supporting non-zero screen angles through two-square decomposition.
type Type10 struct {
	// Size1 is the side of square X in device pixels.
	Size1 int

	// Size2 is the side of square Y in device pixels.
	Size2 int

	// ThresholdData contains the threshold values for both squares.
	// The first Size1*Size1 bytes represent the first square, followed by
	// Size2*Size2 bytes for the second square, both in row-major order.
	ThresholdData []uint8

	// TransferFunction (optional) overrides the current transfer function for
	// this component. Use [function.Identity] for the identity function.
	TransferFunction pdf.Function
}

var _ graphics.Halftone = (*Type10)(nil)

// extractType10 reads a Type 10 halftone from a PDF stream.
func extractType10(x *pdf.Extractor, stream *pdf.Stream) (*Type10, error) {
	h := &Type10{}

	if xsquare, ok := stream.Dict["Xsquare"]; ok {
		xsquareVal, err := x.GetInteger(xsquare)
		if err != nil {
			return nil, err
		}
		h.Size1 = int(xsquareVal)
	}

	if ysquare, ok := stream.Dict["Ysquare"]; ok {
		ysquareVal, err := x.GetInteger(ysquare)
		if err != nil {
			return nil, err
		}
		h.Size2 = int(ysquareVal)
	}

	if tf, err := pdf.Resolve(x.R, stream.Dict["TransferFunction"]); err != nil {
		return nil, err
	} else if tf == pdf.Name("Identity") {
		h.TransferFunction = function.Identity
	} else {
		if F, err := pdf.Optional(function.Extract(x, tf)); err != nil {
			return nil, err
		} else if isValidTransferFunction(F) {
			h.TransferFunction = F
		}
	}

	// Validate dimensions
	if h.Size1 <= 0 || h.Size2 <= 0 {
		return nil, fmt.Errorf("invalid square dimensions %dx%d", h.Size1, h.Size2)
	}

	// Read threshold data if dimensions are provided
	if h.Size1 > 0 && h.Size2 > 0 {
		expectedSize := h.Size1*h.Size1 + h.Size2*h.Size2
		stmReader, err := pdf.DecodeStream(x.R, stream, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to decode stream: %w", err)
		}
		defer stmReader.Close()

		data := make([]byte, expectedSize)
		n, err := io.ReadFull(stmReader, data)
		if err != nil {
			return nil, fmt.Errorf("failed to read threshold data: %w", err)
		}
		if n != expectedSize {
			return nil, fmt.Errorf("incomplete threshold data: expected %d bytes, got %d", expectedSize, n)
		}
		h.ThresholdData = data
	}

	return h, nil
}

func (h *Type10) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "halftone screening", pdf.V1_2); err != nil {
		return nil, err
	}

	if h.Size1 <= 0 || h.Size2 <= 0 {
		return nil, fmt.Errorf("invalid square dimensions %dx%d", h.Size1, h.Size2)
	}
	expectedSize := h.Size1*h.Size1 + h.Size2*h.Size2
	if len(h.ThresholdData) != expectedSize {
		return nil, fmt.Errorf("threshold data size mismatch: expected %d bytes, got %d", expectedSize, len(h.ThresholdData))
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(10),
	}

	if h.Size1 > 0 {
		dict["Xsquare"] = pdf.Integer(h.Size1)
	}
	if h.Size2 > 0 {
		dict["Ysquare"] = pdf.Integer(h.Size2)
	}

	// Add optional fields
	opt := rm.Out().GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if h.TransferFunction == function.Identity {
		dict["TransferFunction"] = pdf.Name("Identity")
	} else if h.TransferFunction != nil {
		if !isValidTransferFunction(h.TransferFunction) {
			return nil, errors.New("invalid transfer function shape")
		}
		ref, err := rm.Embed(h.TransferFunction)
		if err != nil {
			return nil, err
		}
		dict["TransferFunction"] = ref
	}

	// Create the stream with threshold data
	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}

	if len(h.ThresholdData) > 0 {
		_, err = stm.Write(h.ThresholdData)
		if err != nil {
			stm.Close()
			return nil, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// HalftoneType returns 10.
// This implements the [graphics.Halftone] interface.
func (h *Type10) HalftoneType() int {
	return 10
}

// GetTransferFunction returns the transfer function given in the halftone.
// This implements the [graphics.Halftone] interface.
func (h *Type10) GetTransferFunction() pdf.Function {
	return h.TransferFunction
}
