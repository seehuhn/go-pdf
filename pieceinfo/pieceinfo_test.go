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

package pieceinfo

import (
	"testing"
	"time"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
)

func TestExtract(t *testing.T) {
	// test with nil object
	info, err := Extract(nil, nil)
	if err != nil {
		t.Errorf("Extract(nil, nil) returned error: %v", err)
	}
	if info != nil {
		t.Errorf("Extract(nil, nil) returned non-nil info: %v", info)
	}

	// test with empty dictionary
	emptyDict := pdf.Dict{}
	info, err = Extract(nil, emptyDict)
	if err != nil {
		t.Errorf("Extract with empty dict returned error: %v", err)
	}
	if info == nil {
		t.Errorf("Extract with empty dict returned nil info")
	} else if len(info.Entries) != 0 {
		t.Errorf("Extract with empty dict returned %d entries, want 0", len(info.Entries))
	}
}

func TestUnknownLastModified(t *testing.T) {
	now := time.Now()
	u := &unknown{
		lastModified: now,
	}

	if !u.LastModified().Equal(now) {
		t.Errorf("LastModified() = %v, want %v", u.LastModified(), now)
	}
}

func TestRegister(t *testing.T) {
	// clear registry for test
	registryMu.Lock()
	registry = make(map[pdf.Name]func(r pdf.Getter, private pdf.Object) (Data, error))
	registryMu.Unlock()

	testName := pdf.Name("TestHandler")
	testHandler := func(r pdf.Getter, private pdf.Object) (Data, error) {
		return &unknown{lastModified: time.Now()}, nil
	}

	Register(testName, testHandler)

	registryMu.RLock()
	_, exists := registry[testName]
	registryMu.RUnlock()

	if !exists {
		t.Errorf("Register did not store handler for %v", testName)
	}
}

func TestExtractWithValidData(t *testing.T) {
	// create a mock piece info dictionary
	now := pdf.Date(time.Now())
	privateData := pdf.String("some private data")

	dataDict := pdf.Dict{
		"LastModified": now,
		"Private":      privateData,
	}

	pieceDict := pdf.Dict{
		"TestApp": dataDict,
	}

	info, err := Extract(mock.Getter, pieceDict)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if info == nil {
		t.Fatal("Extract returned nil info")
	}

	if len(info.Entries) != 1 {
		t.Errorf("Extract returned %d entries, want 1", len(info.Entries))
	}

	data, exists := info.Entries["TestApp"]
	if !exists {
		t.Error("Extract did not find TestApp entry")
	}

	unknown, ok := data.(*unknown)
	if !ok {
		t.Errorf("Entry is not *unknown type: %T", data)
	}

	if string(unknown.Private.(pdf.String)) != string(privateData) {
		t.Errorf("Private data mismatch: got %v, want %v", unknown.Private, privateData)
	}
}

func TestPieceInfoEmbed(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	// test with nil PieceInfo
	var nilInfo *PieceInfo
	result, _, err := pdf.ResourceManagerEmbed(rm, nilInfo)
	if err != nil {
		t.Errorf("Embed(nil) returned error: %v", err)
	}
	if result != nil {
		t.Errorf("Embed(nil) returned non-nil result: %v", result)
	}

	// test with empty PieceInfo
	emptyInfo := &PieceInfo{Entries: make(map[pdf.Name]Data)}
	result, _, err = pdf.ResourceManagerEmbed(rm, emptyInfo)
	if err != nil {
		t.Errorf("Embed(empty) returned error: %v", err)
	}
	if result != nil {
		t.Errorf("Embed(empty) returned non-nil result: %v", result)
	}
}

func TestErrDiscard(t *testing.T) {
	// clear registry for test
	registryMu.Lock()
	registry = make(map[pdf.Name]func(r pdf.Getter, private pdf.Object) (Data, error))
	registryMu.Unlock()

	// register a handler that discards entries
	testName := pdf.Name("DiscardHandler")
	Register(testName, func(r pdf.Getter, private pdf.Object) (Data, error) {
		return nil, ErrDiscard
	})

	// create test data
	now := pdf.Date(time.Now())
	dataDict := pdf.Dict{
		"LastModified": now,
		"Private":      pdf.String("test data"),
	}
	pieceDict := pdf.Dict{
		testName: dataDict,
	}

	info, err := Extract(mock.Getter, pieceDict)
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	if info == nil {
		t.Fatal("Extract returned nil info")
	}

	// the entry should be discarded, so no entries should remain
	if len(info.Entries) != 0 {
		t.Errorf("Extract returned %d entries, want 0 (entry should be discarded)", len(info.Entries))
	}
}

func TestSingleUse(t *testing.T) {
	// test direct dictionary (SingleUse = true)
	now := pdf.Date(time.Now())
	dataDict := pdf.Dict{
		"LastModified": now,
		"Private":      pdf.String("test data"),
	}
	pieceDict := pdf.Dict{
		"TestApp": dataDict,
	}

	info, err := Extract(mock.Getter, pieceDict)
	if err != nil {
		t.Fatalf("Extract with direct dict returned error: %v", err)
	}
	if !info.SingleUse {
		t.Error("Extract with direct dict should set SingleUse to true")
	}

	// test indirect reference (SingleUse = false)
	// create a writer with the piece dict stored as an indirect object
	w2, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	ref := w2.Alloc()
	err = w2.Put(ref, pieceDict)
	if err != nil {
		t.Fatalf("Failed to write indirect object: %v", err)
	}

	info2, err := Extract(w2, ref)
	if err != nil {
		t.Fatalf("Extract with reference returned error: %v", err)
	}
	if info2.SingleUse {
		t.Error("Extract with reference should set SingleUse to false")
	}

	// test Embed with SingleUse = true (returns direct dict)
	w, _ := memfile.NewPDFWriter(pdf.V1_7, nil)
	rm := pdf.NewResourceManager(w)

	info.SingleUse = true
	result, _, err := pdf.ResourceManagerEmbed(rm, info)
	if err != nil {
		t.Fatalf("Embed with SingleUse=true returned error: %v", err)
	}
	if _, isDict := result.(pdf.Dict); !isDict {
		t.Errorf("Embed with SingleUse=true should return pdf.Dict, got %T", result)
	}

	infoCopy := &PieceInfo{
		Entries:   info.Entries,
		SingleUse: false,
	}
	result2, _, err := pdf.ResourceManagerEmbed(rm, infoCopy)
	if err != nil {
		t.Fatalf("Embed with SingleUse=false returned error: %v", err)
	}
	if _, isRef := result2.(pdf.Reference); !isRef {
		t.Errorf("Embed with SingleUse=false should return pdf.Reference, got %T", result2)
	}
}
