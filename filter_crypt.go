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

package pdf

import (
	"io"
)

// filterCrypt is a special filter that handles stream encryption/decryption.
// Unlike regular filters which are stored in the PDF file's stream dictionary,
// this filter is applied transparently based on the document's encryption settings.
//
// This filter allows Stream.R to remain the raw (possibly encrypted) seekable
// stream data from the file. Decryption happens on-the-fly when the stream is
// read through DecodeStream, allowing streams to be:
//   - Read multiple times (by seeking back to the beginning)
//   - Properly decoded even after seeking
type filterCrypt struct {
	enc *encryptInfo
	ref Reference
}

// Info implements the [Filter] interface.
// The crypt filter is transparent and does not appear in the stream dictionary.
func (f *filterCrypt) Info(Version) (Name, Dict, error) {
	// The Crypt filter should not appear in the stream dictionary -
	// it's handled separately via the document's encryption dictionary.
	return "", nil, nil
}

// Encode implements the [Filter] interface.
func (f *filterCrypt) Encode(_ Version, w io.WriteCloser) (io.WriteCloser, error) {
	return f.enc.EncryptStream(f.ref, w)
}

// Decode implements the [Filter] interface.
func (f *filterCrypt) Decode(_ Version, r io.Reader) (io.ReadCloser, error) {
	decrypted, err := f.enc.DecryptStream(f.ref, r)
	if err != nil {
		return nil, err
	}
	// Wrap in a ReadCloser since DecryptStream returns io.Reader
	if rc, ok := decrypted.(io.ReadCloser); ok {
		return rc, nil
	}
	return io.NopCloser(decrypted), nil
}
