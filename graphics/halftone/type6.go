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
	"seehuhn.de/go/pdf/graphics/transfer"
)

// PDF 2.0 sections: 10.6.4 10.6.5.1 10.6.5.3

// Type6 represents a Type 6 halftone that uses threshold arrays with zero screen angle.
type Type6 struct {
	// Width is the threshold array width in device pixels.
	Width int

	// Height is the threshold array height in device pixels.
	Height int

	// ThresholdData contains Width * Height bytes, each with an 8-bit
	// threshold value (0-255). Values are stored in row-major order with
	// horizontal coordinates changing faster than vertical. The first value
	// corresponds to device coordinates (0, 0).
	ThresholdData []byte

	// TransferFunction (optional) overrides the current transfer function for
	// this component. Use [transfer.Identity] for the identity function.
	TransferFunction pdf.Function
}

var _ Halftone = (*Type6)(nil)

// extractType6 reads a Type 6 halftone from a PDF stream.
func extractType6(x *pdf.Extractor, stream *pdf.Stream) (*Type6, error) {
	dict := stream.Dict

	h := &Type6{}

	if width, err := pdf.GetInteger(x.R, dict["Width"]); err != nil {
		return nil, err
	} else if width > 0 && width <= 1024 {
		h.Width = int(width)
	} else {
		return nil, pdf.Error("invalid Type 6 halftone Width")
	}

	if height, err := pdf.GetInteger(x.R, dict["Height"]); err != nil {
		return nil, err
	} else if height > 0 && height <= 1024 {
		h.Height = int(height)
	} else {
		return nil, pdf.Error("invalid Type 6 halftone Height")
	}

	if tf, err := pdf.Resolve(x.R, dict["TransferFunction"]); err != nil {
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

	// Read threshold data if dimensions are provided
	if h.Width > 0 && h.Height > 0 {
		r, err := pdf.GetStreamReader(x.R, stream)
		if err != nil {
			return nil, err
		}
		defer r.Close()

		expectedSize := h.Width * h.Height
		data := make([]byte, expectedSize)
		_, err = io.ReadFull(r, data)
		if err != nil {
			return nil, err
		}
		h.ThresholdData = data
	}

	return h, nil
}

func (h *Type6) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {

	if err := pdf.CheckVersion(rm.Out(), "halftone screening", pdf.V1_2); err != nil {
		return nil, err
	}

	if h.Width <= 0 || h.Height <= 0 {
		return nil, fmt.Errorf("invalid threshold array dimensions %dx%d", h.Width, h.Height)
	}
	expectedSize := h.Width * h.Height
	if len(h.ThresholdData) != expectedSize {
		return nil, fmt.Errorf("threshold data size mismatch: expected %d bytes, got %d", expectedSize, len(h.ThresholdData))
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
	opt := rm.Out().GetOptions()
	if opt.HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("Halftone")
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

	// create the stream
	ref := rm.Alloc()
	stm, err := rm.Out().OpenStream(ref, dict, pdf.FilterCompress{})
	if err != nil {
		return nil, err
	}
	_, err = stm.Write(h.ThresholdData)
	if err != nil {
		stm.Close()
		return nil, err
	}
	err = stm.Close()
	if err != nil {
		return nil, err
	}

	return ref, nil
}

// HalftoneType returns 6.
// This implements the [Halftone] interface.
func (h *Type6) HalftoneType() int {
	return 6
}

// GetTransferFunction returns the transfer function given in the halftone.
// This implements the [Halftone] interface.
func (h *Type6) GetTransferFunction() pdf.Function {
	return h.TransferFunction
}
