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
	"fmt"
	"io"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// Type6 represents a Type 6 halftone that uses threshold arrays with zero screen angle.
type Type6 struct {
	// HalftoneName (optional) is the name of the halftone dictionary.
	HalftoneName string

	// Width is the threshold array width in device pixels.
	Width int

	// Height is the threshold array height in device pixels.
	Height int

	// ThresholdData contains Width × Height bytes, each with an 8-bit threshold value (0-255).
	// Values are stored in row-major order with horizontal coordinates changing faster than vertical.
	// The first value corresponds to device coordinates (0, 0).
	ThresholdData []byte

	// TransferFunction (optional) overrides the current transfer function.
	// Use pdf.Name("Identity") for the identity function.
	TransferFunction pdf.Object
}

var _ graphics.Halftone = (*Type6)(nil)

// HalftoneType returns 6.
// This implements the [graphics.Halftone] interface.
func (h *Type6) HalftoneType() int {
	return 6
}

func (h *Type6) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "halftone screening", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	if h.HalftoneName == "" {
		if h.Width <= 0 || h.Height <= 0 {
			return nil, zero, fmt.Errorf("invalid threshold array dimensions %d×%d", h.Width, h.Height)
		}
		expectedSize := h.Width * h.Height
		if len(h.ThresholdData) != expectedSize {
			return nil, zero, fmt.Errorf("threshold data size mismatch: expected %d bytes, got %d", expectedSize, len(h.ThresholdData))
		}
	} else {
		// If HalftoneName is provided, all other fields become optional.
		if h.Width < 0 || h.Height < 0 {
			return nil, zero, fmt.Errorf("invalid threshold array dimensions %d×%d", h.Width, h.Height)
		}
		if h.Width > 0 && h.Height > 0 {
			expectedSize := h.Width * h.Height
			if len(h.ThresholdData) != 0 && len(h.ThresholdData) != expectedSize {
				return nil, zero, fmt.Errorf("threshold data size mismatch: expected %d bytes, got %d", expectedSize, len(h.ThresholdData))
			}
		}
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(6),
	}

	if h.Width > 0 {
		dict["Width"] = pdf.Integer(h.Width)
	}
	if h.Height > 0 {
		dict["Height"] = pdf.Integer(h.Height)
	}

	// Add optional fields
	opt := rm.Out.GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
	}

	if h.HalftoneName != "" {
		dict["HalftoneName"] = pdf.String(h.HalftoneName)
	}

	if h.TransferFunction != nil {
		dict["TransferFunction"] = h.TransferFunction
	}

	// Create the stream with threshold data
	ref := rm.Out.Alloc()
	stm, err := rm.Out.OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, zero, err
	}

	if len(h.ThresholdData) > 0 {
		_, err = stm.Write(h.ThresholdData)
		if err != nil {
			return nil, zero, err
		}
	}

	err = stm.Close()
	if err != nil {
		return nil, zero, err
	}

	return ref, zero, nil
}

// readType6 reads a Type 6 halftone from a PDF stream.
func readType6(r pdf.Getter, stream *pdf.Stream) (*Type6, error) {
	h := &Type6{}

	if name, ok := stream.Dict["HalftoneName"]; ok {
		halftoneName, err := pdf.GetString(r, name)
		if err != nil {
			return nil, err
		}
		h.HalftoneName = string(halftoneName)
	}

	if width, ok := stream.Dict["Width"]; ok {
		widthVal, err := pdf.GetInteger(r, width)
		if err != nil {
			return nil, err
		}
		h.Width = int(widthVal)
	}

	if height, ok := stream.Dict["Height"]; ok {
		heightVal, err := pdf.GetInteger(r, height)
		if err != nil {
			return nil, err
		}
		h.Height = int(heightVal)
	}

	if transferFunc, ok := stream.Dict["TransferFunction"]; ok {
		h.TransferFunction = transferFunc
	}

	// Validate dimensions
	if h.HalftoneName == "" {
		if h.Width <= 0 || h.Height <= 0 {
			return nil, fmt.Errorf("invalid threshold array dimensions %d×%d", h.Width, h.Height)
		}
	} else {
		if h.Width < 0 || h.Height < 0 {
			return nil, fmt.Errorf("invalid threshold array dimensions %d×%d", h.Width, h.Height)
		}
	}

	// Read threshold data if dimensions are provided
	if h.Width > 0 && h.Height > 0 {
		expectedSize := h.Width * h.Height
		stmReader, err := pdf.DecodeStream(r, stream, 0)
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
