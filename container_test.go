// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2024  Jochen Voss <voss@seehuhn.de>
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

import "testing"

func TestGetDictTyped_NilObject(t *testing.T) {
	mockGetter := new(MockGetter)
	dict, err := GetDictTyped(mockGetter, nil, "test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if dict != nil {
		t.Fatalf("expected nil, got %v", dict)
	}
}

// MockGetter is a mock implementation of pdf.Getter for testing
type MockGetter struct{}

func (m *MockGetter) Get(ref Reference, canObjStm bool) (Native, error) {
	return nil, nil
}

func (m *MockGetter) GetMeta() *MetaInfo {
	return nil
}
