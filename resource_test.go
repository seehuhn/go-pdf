// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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
	"bytes"
	"testing"
)

// reserveOnlyEncoder reserves a reference for itself when encoded but is never
// stored, modelling an object referenced by another but never written.
type reserveOnlyEncoder struct{ ref Reference }

func (e *reserveOnlyEncoder) Encode(rm *ResourceManager) (Native, error) {
	return Dict{"Other": rm.GetReference(e)}, nil
}

func newTestResourceManager(t *testing.T) *ResourceManager {
	t.Helper()
	w, err := NewWriter(&bytes.Buffer{}, V2_0, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewResourceManager(w)
}

// a reference reserved via GetReference but never written leaves a dangling
// reference; Close must report it rather than produce an invalid file.
func TestResourceManagerCloseReservedNotWritten(t *testing.T) {
	rm := newTestResourceManager(t)

	enc := &reserveOnlyEncoder{}
	enc.ref = rm.GetReference(enc) // reserve, but never Store

	if err := rm.Close(); err == nil {
		t.Error("expected Close to report the unwritten reservation")
	}
}

// when the reserved encoder is also stored, the reservation is cleared and
// Close succeeds.
func TestResourceManagerCloseReservedThenStored(t *testing.T) {
	rm := newTestResourceManager(t)

	enc := &reserveOnlyEncoder{}
	_ = rm.GetReference(enc)
	if _, err := rm.Store(enc); err != nil {
		t.Fatal(err)
	}

	if err := rm.Close(); err != nil {
		t.Errorf("unexpected error from Close: %v", err)
	}
}
