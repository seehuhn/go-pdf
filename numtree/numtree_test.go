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

package numtree

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

func TestInMemoryBasic(t *testing.T) {
	tree := &InMemory{
		Data: map[pdf.Integer]pdf.Object{
			1:  pdf.Name("one"),
			5:  pdf.Name("five"),
			10: pdf.Name("ten"),
		},
	}

	// test Lookup
	tests := []struct {
		key     pdf.Integer
		want    pdf.Object
		wantErr bool
	}{
		{1, pdf.Name("one"), false},
		{5, pdf.Name("five"), false},
		{10, pdf.Name("ten"), false},
		{3, nil, true},
		{0, nil, true},
		{15, nil, true},
	}

	for _, tt := range tests {
		got, err := tree.Lookup(tt.key)
		if (err != nil) != tt.wantErr {
			t.Errorf("Lookup(%d) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("Lookup(%d) = %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestInMemoryAll(t *testing.T) {
	tree := &InMemory{
		Data: map[pdf.Integer]pdf.Object{
			100: pdf.Name("hundred"),
			1:   pdf.Name("one"),
			50:  pdf.Name("fifty"),
		},
	}

	var keys []pdf.Integer
	var values []pdf.Object
	for key, value := range tree.All() {
		keys = append(keys, key)
		values = append(values, value)
	}

	expectedKeys := []pdf.Integer{1, 50, 100}
	expectedValues := []pdf.Object{pdf.Name("one"), pdf.Name("fifty"), pdf.Name("hundred")}

	if !slices.Equal(keys, expectedKeys) {
		t.Errorf("All() keys = %v, want %v", keys, expectedKeys)
	}
	if !slices.Equal(values, expectedValues) {
		t.Errorf("All() values = %v, want %v", values, expectedValues)
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	original := &InMemory{
		Data: map[pdf.Integer]pdf.Object{
			-10: pdf.Name("negative"),
			0:   pdf.Name("zero"),
			5:   pdf.Name("five"),
			100: pdf.Name("hundred"),
			999: pdf.Name("nines"),
		},
	}

	// write to PDF
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write(w, original.All())
	if err != nil {
		t.Fatal(err)
	}

	// read back using ExtractInMemory
	extracted, err := ExtractInMemory(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(original.Data, extracted.Data); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestWriteReadRoundTripFromFile(t *testing.T) {
	originalData := map[pdf.Integer]pdf.Object{
		1:   pdf.Name("first"),
		2:   pdf.Name("second"),
		100: pdf.Name("hundredth"),
	}

	original := &InMemory{Data: originalData}

	// write to PDF
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write(w, original.All())
	if err != nil {
		t.Fatal(err)
	}

	// read back using ExtractFromFile
	fromFile, err := ExtractFromFile(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	// test lookup
	for key, expectedValue := range originalData {
		got, err := fromFile.Lookup(key)
		if err != nil {
			t.Errorf("FromFile.Lookup(%d) error = %v", key, err)
			continue
		}
		if got != expectedValue {
			t.Errorf("FromFile.Lookup(%d) = %v, want %v", key, got, expectedValue)
		}
	}

	// test missing key
	_, err = fromFile.Lookup(999)
	if err != ErrKeyNotFound {
		t.Errorf("FromFile.Lookup(999) error = %v, want %v", err, ErrKeyNotFound)
	}

	// test All iterator
	var keys []pdf.Integer
	var values []pdf.Object
	for key, value := range fromFile.All() {
		keys = append(keys, key)
		values = append(values, value)
	}

	expectedKeys := []pdf.Integer{1, 2, 100}
	expectedValues := []pdf.Object{pdf.Name("first"), pdf.Name("second"), pdf.Name("hundredth")}

	if !slices.Equal(keys, expectedKeys) {
		t.Errorf("FromFile.All() keys = %v, want %v", keys, expectedKeys)
	}
	if !slices.Equal(values, expectedValues) {
		t.Errorf("FromFile.All() values = %v, want %v", values, expectedValues)
	}
}

func TestSizeFunction(t *testing.T) {
	tree := &InMemory{
		Data: map[pdf.Integer]pdf.Object{
			1: pdf.Integer(10),
			2: pdf.Integer(20),
			3: pdf.Integer(30),
		},
	}

	// write to PDF
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write(w, tree.All())
	if err != nil {
		t.Fatal(err)
	}

	// test Size function
	size, err := Size(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if size != 3 {
		t.Errorf("Size() = %d, want %d", size, 3)
	}
}

func TestLargeTree(t *testing.T) {
	// create a large tree to test multi-level structure
	tree := &InMemory{
		Data: make(map[pdf.Integer]pdf.Object),
	}

	// add many entries to force multi-level tree
	for i := 0; i < 200; i++ {
		tree.Data[pdf.Integer(i)] = pdf.Integer(i * i)
	}

	// write to PDF
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write(w, tree.All())
	if err != nil {
		t.Fatal(err)
	}

	// test Size function
	size, err := Size(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if size != 200 {
		t.Errorf("Size() = %d, want %d", size, 200)
	}

	// test FromFile lookup
	fromFile, err := ExtractFromFile(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	// test a few lookups
	testKey := pdf.Integer(42)
	got, err := fromFile.Lookup(testKey)
	if err != nil {
		t.Errorf("FromFile.Lookup(%d) error = %v", testKey, err)
	}
	if got != pdf.Integer(42*42) {
		t.Errorf("FromFile.Lookup(%d) = %v, want %v", testKey, got, pdf.Integer(42*42))
	}
}

func TestEmptyTree(t *testing.T) {
	tree := &InMemory{
		Data: map[pdf.Integer]pdf.Object{},
	}

	// write empty tree
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write(w, tree.All())
	if err != nil {
		t.Fatal(err)
	}

	// ref should be 0 for empty tree
	if ref != 0 {
		t.Errorf("Write(empty) = %v, want 0", ref)
	}
}

func TestNilTree(t *testing.T) {
	var tree *InMemory

	// test nil tree lookup
	_, err := tree.Lookup(1)
	if err != ErrKeyNotFound {
		t.Errorf("nil tree Lookup error = %v, want %v", err, ErrKeyNotFound)
	}

	// test nil tree All
	count := 0
	for range tree.All() {
		count++
	}
	if count != 0 {
		t.Errorf("nil tree All() yielded %d items, want 0", count)
	}
}

func TestStreamingUnsortedKeys(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// create unsorted iterator
	data := func(yield func(pdf.Integer, pdf.Object) bool) {
		if !yield(100, pdf.Integer(1)) {
			return
		}
		yield(5, pdf.Integer(2)) // out of order!
	}

	_, err := Write(w, data)
	if err == nil {
		t.Error("Write() should return error for unsorted keys")
	}
	if err.Error() != "keys must be in sorted order" {
		t.Errorf("Write() error = %q, want %q", err.Error(), "keys must be in sorted order")
	}
}

func TestStreamingDuplicateKeys(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// create iterator with duplicate keys
	data := func(yield func(pdf.Integer, pdf.Object) bool) {
		if !yield(1, pdf.Integer(10)) {
			return
		}
		yield(1, pdf.Integer(20)) // duplicate!
	}

	_, err := Write(w, data)
	if err == nil {
		t.Error("Write() should return error for duplicate keys")
	}
	if err.Error() != "keys must be in sorted order" {
		t.Errorf("Write() error = %q, want %q", err.Error(), "keys must be in sorted order")
	}
}

func TestStreamingVeryLargeTree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large tree test in short mode")
	}

	// create a very large tree to test streaming behavior
	const numEntries = 1000

	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)

	// streaming iterator that generates entries on demand
	data := func(yield func(pdf.Integer, pdf.Object) bool) {
		for i := 0; i < numEntries; i++ {
			key := pdf.Integer(i)
			value := pdf.Integer(i * 2)
			if !yield(key, value) {
				return
			}
		}
	}

	ref, err := Write(w, data)
	if err != nil {
		t.Fatal(err)
	}

	// verify size
	size, err := Size(w, ref)
	if err != nil {
		t.Fatal(err)
	}
	if size != numEntries {
		t.Errorf("Size() = %d, want %d", size, numEntries)
	}

	// test a few lookups with FromFile
	fromFile, err := ExtractFromFile(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	// test first, middle, and last entries
	testCases := []struct {
		key   pdf.Integer
		value pdf.Integer
	}{
		{0, 0},
		{numEntries / 2, numEntries},
		{numEntries - 1, (numEntries - 1) * 2},
	}

	for _, tc := range testCases {
		got, err := fromFile.Lookup(tc.key)
		if err != nil {
			t.Errorf("Lookup(%d) error = %v", tc.key, err)
			continue
		}
		if got != tc.value {
			t.Errorf("Lookup(%d) = %v, want %v", tc.key, got, tc.value)
		}
	}
}

func TestNegativeKeys(t *testing.T) {
	tree := &InMemory{
		Data: map[pdf.Integer]pdf.Object{
			-100: pdf.Name("negative hundred"),
			-1:   pdf.Name("negative one"),
			0:    pdf.Name("zero"),
			1:    pdf.Name("one"),
			100:  pdf.Name("hundred"),
		},
	}

	// write to PDF
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref, err := Write(w, tree.All())
	if err != nil {
		t.Fatal(err)
	}

	// read back and test
	fromFile, err := ExtractFromFile(w, ref)
	if err != nil {
		t.Fatal(err)
	}

	// verify all keys work, including negatives
	for key, expectedValue := range tree.Data {
		got, err := fromFile.Lookup(key)
		if err != nil {
			t.Errorf("Lookup(%d) error = %v", key, err)
			continue
		}
		if got != expectedValue {
			t.Errorf("Lookup(%d) = %v, want %v", key, got, expectedValue)
		}
	}

	// test All iterator maintains numerical order
	var keys []pdf.Integer
	for key := range fromFile.All() {
		keys = append(keys, key)
	}

	expectedKeys := []pdf.Integer{-100, -1, 0, 1, 100}
	if !slices.Equal(keys, expectedKeys) {
		t.Errorf("All() keys = %v, want %v", keys, expectedKeys)
	}
}
