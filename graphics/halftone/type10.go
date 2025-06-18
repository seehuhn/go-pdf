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

// Type10 represents a Type 10 halftone that uses angled threshold arrays
// supporting non-zero screen angles through two-square decomposition.
type Type10 struct {
	// HalftoneName (optional) is the name of the halftone dictionary.
	HalftoneName string

	// Xsquare is the side of square X in device pixels.
	// Horizontal displacement between corresponding points in adjacent halftone cells.
	Xsquare int

	// Ysquare is the side of square Y in device pixels.
	// Vertical displacement between corresponding points in adjacent halftone cells.
	Ysquare int

	// ThresholdData contains the threshold values for both squares.
	// The first Xsquare² bytes represent the first square, followed by
	// Ysquare² bytes for the second square, both in row-major order.
	ThresholdData []uint8

	// TransferFunction (optional) overrides the current transfer function.
	// Use pdf.Name("Identity") for the identity function.
	TransferFunction pdf.Object
}

var _ graphics.Halftone = (*Type10)(nil)

// HalftoneType returns 10.
// This implements the [graphics.Halftone] interface.
func (h *Type10) HalftoneType() int {
	return 10
}

func (h *Type10) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	if err := pdf.CheckVersion(rm.Out, "halftone screening", pdf.V1_2); err != nil {
		return nil, zero, err
	}

	if h.HalftoneName == "" {
		if h.Xsquare <= 0 || h.Ysquare <= 0 {
			return nil, zero, fmt.Errorf("invalid square dimensions %d×%d", h.Xsquare, h.Ysquare)
		}
		expectedSize := h.Xsquare*h.Xsquare + h.Ysquare*h.Ysquare
		if len(h.ThresholdData) != expectedSize {
			return nil, zero, fmt.Errorf("threshold data size mismatch: expected %d bytes, got %d", expectedSize, len(h.ThresholdData))
		}
	} else {
		// If HalftoneName is provided, all other fields become optional.
		if h.Xsquare < 0 || h.Ysquare < 0 {
			return nil, zero, fmt.Errorf("invalid square dimensions %d×%d", h.Xsquare, h.Ysquare)
		}
		if h.Xsquare > 0 && h.Ysquare > 0 {
			expectedSize := h.Xsquare*h.Xsquare + h.Ysquare*h.Ysquare
			if len(h.ThresholdData) != 0 && len(h.ThresholdData) != expectedSize {
				return nil, zero, fmt.Errorf("threshold data size mismatch: expected %d bytes, got %d", expectedSize, len(h.ThresholdData))
			}
		}
	}

	dict := pdf.Dict{
		"HalftoneType": pdf.Integer(10),
	}

	if h.Xsquare > 0 {
		dict["Xsquare"] = pdf.Integer(h.Xsquare)
	}
	if h.Ysquare > 0 {
		dict["Ysquare"] = pdf.Integer(h.Ysquare)
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

// readType10 reads a Type 10 halftone from a PDF stream.
func readType10(r pdf.Getter, stream *pdf.Stream) (*Type10, error) {
	h := &Type10{}

	if name, ok := stream.Dict["HalftoneName"]; ok {
		halftoneName, err := pdf.GetString(r, name)
		if err != nil {
			return nil, err
		}
		h.HalftoneName = string(halftoneName)
	}

	if xsquare, ok := stream.Dict["Xsquare"]; ok {
		xsquareVal, err := pdf.GetInteger(r, xsquare)
		if err != nil {
			return nil, err
		}
		h.Xsquare = int(xsquareVal)
	}

	if ysquare, ok := stream.Dict["Ysquare"]; ok {
		ysquareVal, err := pdf.GetInteger(r, ysquare)
		if err != nil {
			return nil, err
		}
		h.Ysquare = int(ysquareVal)
	}

	if transferFunc, ok := stream.Dict["TransferFunction"]; ok {
		h.TransferFunction = transferFunc
	}

	// Validate dimensions
	if h.HalftoneName == "" {
		if h.Xsquare <= 0 || h.Ysquare <= 0 {
			return nil, fmt.Errorf("invalid square dimensions %d×%d", h.Xsquare, h.Ysquare)
		}
	} else {
		if h.Xsquare < 0 || h.Ysquare < 0 {
			return nil, fmt.Errorf("invalid square dimensions %d×%d", h.Xsquare, h.Ysquare)
		}
	}

	// Read threshold data if dimensions are provided
	if h.Xsquare > 0 && h.Ysquare > 0 {
		expectedSize := h.Xsquare*h.Xsquare + h.Ysquare*h.Ysquare
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
