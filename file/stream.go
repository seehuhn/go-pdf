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

package file

import (
	"bytes"
	"io"
	"time"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 7.11.4

// Stream represents an embedded file stream dictionary.
// Embedded file streams allow external files to be embedded directly
// within the PDF document body, making it self-contained.
type Stream struct {
	// MimeType (optional) specifies the MIME media type of the embedded file.
	//
	// This corresponds to the /Subtype entry in the PDF dictionary.
	MimeType string

	// Size (optional) specifies the uncompressed size of the embedded file in
	// bytes. A zero value means the Size entry is omitted from the PDF
	// dictionary.
	Size int64

	// CreationDate (optional) specifies when the embedded file was created.
	CreationDate time.Time

	// ModDate (optional) specifies when the embedded file was last modified.
	ModDate time.Time

	// CheckSum (optional) contains the MD5 checksum of the uncompressed
	// embedded file. Must be exactly 16 bytes when present.
	CheckSum []byte

	// WriteData writes the embedded file data to the provided writer.
	WriteData func(io.Writer) error

	// SingleUse determines if Embed returns a stream dictionary (true) or
	// a reference (false).
	SingleUse bool
}

// ExtractStream extracts an embedded file stream from a PDF stream object.
func ExtractStream(x *pdf.Extractor, obj pdf.Object) (*Stream, error) {
	stream, err := pdf.GetStream(x.R, obj)
	if err != nil {
		return nil, err
	} else if stream == nil {
		return nil, pdf.Errorf("missing embedded file stream")
	}

	// Check Type field - optional for embedded file streams
	if err := pdf.CheckDictType(x.R, stream.Dict, "EmbeddedFile"); err != nil {
		return nil, err
	}

	result := &Stream{}

	// Extract Subtype (MimeType) - optional
	if subtype, err := pdf.Optional(pdf.GetName(x.R, stream.Dict["Subtype"])); err != nil {
		return nil, err
	} else {
		result.MimeType = string(subtype)
	}

	// Extract Params dictionary - optional
	if paramsDict, err := pdf.Optional(pdf.GetDict(x.R, stream.Dict["Params"])); err != nil {
		return nil, err
	} else if paramsDict != nil {
		// Extract Size
		if size, err := pdf.Optional(pdf.GetInteger(x.R, paramsDict["Size"])); err != nil {
			return nil, err
		} else if size >= 0 {
			result.Size = int64(size)
		}

		// Extract CreationDate
		if creationDate, err := pdf.Optional(pdf.GetDate(x.R, paramsDict["CreationDate"])); err != nil {
			return nil, err
		} else {
			result.CreationDate = time.Time(creationDate)
		}

		// Extract ModDate
		if modDate, err := pdf.Optional(pdf.GetDate(x.R, paramsDict["ModDate"])); err != nil {
			return nil, err
		} else {
			result.ModDate = time.Time(modDate)
		}

		// Extract CheckSum
		if checkSum, err := pdf.Optional(pdf.GetString(x.R, paramsDict["CheckSum"])); err != nil {
			return nil, err
		} else if len(checkSum) == 16 { // MD5 is exactly 16 bytes
			result.CheckSum = []byte(checkSum)
		}

		// Mac entry is ignored
	}

	// Set up WriteData to read from the stream
	result.WriteData = func(w io.Writer) error {
		r, err := pdf.GetStreamReader(x.R, stream)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = io.Copy(w, r)
		return err
	}

	return result, nil
}

// Embed converts the Stream to a PDF stream object.
func (s *Stream) Embed(rm *pdf.ResourceManager) (pdf.Native, pdf.Unused, error) {
	var zero pdf.Unused

	// Check PDF version requirement for embedded file streams (PDF 1.3)
	if err := pdf.CheckVersion(rm.Out, "embedded file streams", pdf.V1_3); err != nil {
		return nil, zero, err
	}

	// Validate WriteData is provided
	if s.WriteData == nil {
		return nil, zero, pdf.Errorf("WriteData function is required for embedded file streams")
	}

	// Validate CheckSum length if present
	if len(s.CheckSum) != 0 && len(s.CheckSum) != 16 {
		return nil, zero, pdf.Errorf("CheckSum must be exactly 16 bytes when present")
	}

	// Create stream dictionary
	dict := pdf.Dict{}
	if rm.Out.GetOptions().HasAny(pdf.OptDictTypes) {
		dict["Type"] = pdf.Name("EmbeddedFile")
	}

	// Add Subtype (MimeType) if present
	if s.MimeType != "" {
		dict["Subtype"] = pdf.Name(s.MimeType)
	}

	// Create Params dictionary only if any fields have data
	var paramsDict pdf.Dict
	if s.Size > 0 || !s.CreationDate.IsZero() || !s.ModDate.IsZero() || len(s.CheckSum) > 0 {
		paramsDict = pdf.Dict{}

		// Add Size if greater than 0
		if s.Size > 0 {
			paramsDict["Size"] = pdf.Integer(s.Size)
		}

		// Add CreationDate if not zero
		if !s.CreationDate.IsZero() {
			paramsDict["CreationDate"] = pdf.Date(s.CreationDate)
		}

		// Add ModDate if not zero
		if !s.ModDate.IsZero() {
			paramsDict["ModDate"] = pdf.Date(s.ModDate)
		}

		// Add CheckSum if present
		if len(s.CheckSum) == 16 {
			paramsDict["CheckSum"] = pdf.String(s.CheckSum)
		}

		dict["Params"] = paramsDict
	}

	// Create the stream object
	ref := rm.Out.Alloc()

	// Open stream for writing
	w, err := rm.Out.OpenStream(ref, dict)
	if err != nil {
		return nil, zero, err
	}

	// Write data using the WriteData function
	err = s.WriteData(w)
	if err != nil {
		return nil, zero, err
	}

	// Close the stream
	err = w.Close()
	if err != nil {
		return nil, zero, err
	}

	if s.SingleUse {
		// For SingleUse, we still need to return the reference since
		// embedded file streams are always indirect objects
		return ref, zero, nil
	}

	return ref, zero, nil
}

// Equal compares two Stream objects for equality.
// It compares all fields and the output of WriteData functions.
func (s *Stream) Equal(other *Stream) bool {
	if s == nil && other == nil {
		return true
	}
	if s == nil || other == nil {
		return false
	}

	// Compare basic fields
	if s.MimeType != other.MimeType ||
		s.Size != other.Size ||
		!s.CreationDate.Equal(other.CreationDate) ||
		!s.ModDate.Equal(other.ModDate) ||
		s.SingleUse != other.SingleUse {
		return false
	}

	// Compare CheckSum
	if len(s.CheckSum) != len(other.CheckSum) {
		return false
	}
	for i, b := range s.CheckSum {
		if b != other.CheckSum[i] {
			return false
		}
	}

	// Compare WriteData function outputs
	if s.WriteData == nil && other.WriteData == nil {
		return true
	}
	if s.WriteData == nil || other.WriteData == nil {
		return false
	}

	var sBuf, otherBuf bytes.Buffer
	if err := s.WriteData(&sBuf); err != nil {
		return false
	}
	if err := other.WriteData(&otherBuf); err != nil {
		return false
	}

	return bytes.Equal(sBuf.Bytes(), otherBuf.Bytes())
}
