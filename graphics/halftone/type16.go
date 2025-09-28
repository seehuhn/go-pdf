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
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/function"
	"seehuhn.de/go/pdf/graphics/transfer"
)

// PDF 2.0 sections: 10.6.4 10.6.5.1 10.6.5.5

// Type16 represents a Type 16 halftone that uses high-precision threshold
// arrays with 16-bit threshold values.
type Type16 struct {
	// Width is the width of the first (or only) rectangle in device pixels.
	Width int

	// Height is the height of the first (or only) rectangle in device pixels.
	Height int

	// Width2 (optional) is the width of the second rectangle.
	// If present, Height2 must also be present.
	Width2 int

	// Height2 (optional) is the height of the second rectangle.
	// If present, Width2 must also be present.
	Height2 int

	// ThresholdData contains the 16-bit threshold values.
	// For one rectangle: Width*Height values.
	// For two rectangles: (Width*Height + Width2*Height2) values.
	ThresholdData []uint16

	// TransferFunction (optional) overrides the current transfer function for
	// this component. Use [transfer.Identity] for the identity function.
	TransferFunction pdf.Function
}

var _ Halftone = (*Type16)(nil)

// extractType16 reads a Type 16 halftone from a PDF stream.
func extractType16(x *pdf.Extractor, stream *pdf.Stream) (*Type16, error) {
	h := &Type16{}

	if width, ok := stream.Dict["Width"]; ok {
		widthVal, err := pdf.GetInteger(x.R, width)
		if err != nil {
			return nil, err
		}
		h.Width = int(widthVal)
	}

	if height, ok := stream.Dict["Height"]; ok {
		heightVal, err := pdf.GetInteger(x.R, height)
		if err != nil {
			return nil, err
		}
		h.Height = int(heightVal)
	}

	if width2, ok := stream.Dict["Width2"]; ok {
		width2Val, err := pdf.GetInteger(x.R, width2)
		if err != nil {
			return nil, err
		}
		h.Width2 = int(width2Val)
	}

	if height2, ok := stream.Dict["Height2"]; ok {
		height2Val, err := pdf.GetInteger(x.R, height2)
		if err != nil {
			return nil, err
		}
		h.Height2 = int(height2Val)
	}

	if tf, err := pdf.Resolve(x.R, stream.Dict["TransferFunction"]); err != nil {
		return nil, err
	} else if tf == pdf.Name("Identity") {
		h.TransferFunction = transfer.Identity
	} else {
		if F, err := pdf.Optional(function.Extract(x, tf)); err != nil {
			return nil, err
		} else if isValidTransferFunction(F) {
			h.TransferFunction = F
		}
	}

	// Validate dimensions
	if h.Width <= 0 || h.Height <= 0 {
		return nil, fmt.Errorf("invalid threshold array dimensions %dx%d", h.Width, h.Height)
	}

	hasSecondRect := h.Width2 > 0 || h.Height2 > 0
	if hasSecondRect && (h.Width2 <= 0 || h.Height2 <= 0) {
		return nil, fmt.Errorf("if Width2 or Height2 is specified, both must be positive, got %dx%d", h.Width2, h.Height2)
	}

	// Read threshold data if dimensions are provided
	if h.Width > 0 && h.Height > 0 {
		expectedValues := h.Width * h.Height
		hasSecondRect := h.Width2 > 0 && h.Height2 > 0
		if hasSecondRect {
			expectedValues += h.Width2 * h.Height2
		}
		expectedBytes := expectedValues * 2

		stmReader, err := pdf.DecodeStream(x.R, stream, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to decode stream: %w", err)
		}
		defer stmReader.Close()

		data := make([]byte, expectedBytes)
		n, err := io.ReadFull(stmReader, data)
		if err != nil {
			return nil, fmt.Errorf("failed to read threshold data: %w", err)
		}
		if n != expectedBytes {
			return nil, fmt.Errorf("incomplete threshold data: expected %d bytes, got %d", expectedBytes, n)
		}

		// Convert big-endian bytes to uint16 values
		h.ThresholdData = make([]uint16, expectedValues)
		for i := 0; i < expectedValues; i++ {
			h.ThresholdData[i] = binary.BigEndian.Uint16(data[i*2:])
		}
	}

	return h, nil
}

func (h *Type16) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "Type 16 halftone screening", pdf.V1_3); err != nil {
		return nil, err
	}

	if h.Width <= 0 || h.Height <= 0 {
		return nil, fmt.Errorf("invalid threshold array dimensions %dx%d", h.Width, h.Height)
	}

	hasSecondRect := h.Width2 > 0 || h.Height2 > 0
	if hasSecondRect && (h.Width2 <= 0 || h.Height2 <= 0) {
		return nil, fmt.Errorf("if Width2 or Height2 is specified, both must be positive, got %dx%d", h.Width2, h.Height2)
	}

	// Calculate expected data size (uint16 values)
	expectedValues := h.Width * h.Height
	if hasSecondRect {
		expectedValues += h.Width2 * h.Height2
	}

	if len(h.ThresholdData) != expectedValues {
		return nil, fmt.Errorf("threshold data size mismatch: expected %d values, got %d", expectedValues, len(h.ThresholdData))
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(16),
	}

	if h.Width > 0 {
		dict["Width"] = pdf.Integer(h.Width)
	}
	if h.Height > 0 {
		dict["Height"] = pdf.Integer(h.Height)
	}

	// Add optional fields
	opt := rm.Out().GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if hasSecondRect {
		dict["Width2"] = pdf.Integer(h.Width2)
		dict["Height2"] = pdf.Integer(h.Height2)
	}

	if h.TransferFunction == transfer.Identity {
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
		// Convert uint16 values to big-endian bytes
		data := make([]byte, len(h.ThresholdData)*2)
		for i, val := range h.ThresholdData {
			binary.BigEndian.PutUint16(data[i*2:], val)
		}
		_, err = stm.Write(data)
		if err != nil {
			return nil, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// HalftoneType returns 16.
// This implements the [Halftone] interface.
func (h *Type16) HalftoneType() int {
	return 16
}

// GetTransferFunction returns the transfer function given in the halftone.
// This implements the [Halftone] interface.
func (h *Type16) GetTransferFunction() pdf.Function {
	return h.TransferFunction
}
