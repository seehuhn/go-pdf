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

package reader

import (
	"io"
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// stringOpener returns a reader factory for a string content stream.
func stringOpener(s string) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(s)), nil
	}
}

func TestMarkedContentEventConstants(t *testing.T) {
	// Verify event constants exist and have expected values
	var point MarkedContentEvent = MarkedContentPoint
	var begin MarkedContentEvent = MarkedContentBegin
	var end MarkedContentEvent = MarkedContentEnd

	if point != 0 {
		t.Errorf("MarkedContentPoint = %d, want 0", point)
	}
	if begin != 1 {
		t.Errorf("MarkedContentBegin = %d, want 1", begin)
	}
	if end != 2 {
		t.Errorf("MarkedContentEnd = %d, want 2", end)
	}
}

func TestReaderMarkedContentFields(t *testing.T) {
	r := New(nil)

	// Verify callback field exists
	if r.MarkedContent != nil {
		t.Error("MarkedContent callback should be nil by default")
	}

	// Verify stack field exists and is empty
	if r.MarkedContentStack == nil {
		t.Fatal("MarkedContentStack should be initialized (empty slice, not nil)")
	}
	if len(r.MarkedContentStack) != 0 {
		t.Errorf("MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}
}

func TestResetClearsMarkedContentStack(t *testing.T) {
	r := New(nil)

	// Add something to the stack
	r.MarkedContentStack = append(r.MarkedContentStack, &graphics.MarkedContent{
		Tag: "Test",
	})

	if len(r.MarkedContentStack) != 1 {
		t.Fatal("Setup failed: stack should have 1 element")
	}

	// Reset should clear the stack
	r.Reset()

	if len(r.MarkedContentStack) != 0 {
		t.Errorf("After Reset(), MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}
}

func TestMaxMarkedContentDepthConstant(t *testing.T) {
	if maxMarkedContentDepth != 64 {
		t.Errorf("maxMarkedContentDepth = %d, want 64", maxMarkedContentDepth)
	}
}

func TestMPOperator(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var called bool
	var gotEvent MarkedContentEvent
	var gotMC *graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		called = true
		gotEvent = event
		gotMC = mc
		return nil
	}

	// Parse: MP /Caption
	err := r.ParseContentStream(stringOpener("/Caption MP"))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	if !called {
		t.Fatal("MarkedContent callback was not called")
	}

	if gotEvent != MarkedContentPoint {
		t.Errorf("event = %v, want MarkedContentPoint", gotEvent)
	}

	if gotMC.Tag != "Caption" {
		t.Errorf("mc.Tag = %q, want \"Caption\"", gotMC.Tag)
	}

	if gotMC.Properties != nil {
		t.Errorf("mc.Properties = %v, want nil", gotMC.Properties)
	}

	// Stack should still be empty (MP doesn't push)
	if len(r.MarkedContentStack) != 0 {
		t.Errorf("MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}
}

func TestBMCOperator(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var events []MarkedContentEvent
	var mcs []*graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		events = append(events, event)
		mcs = append(mcs, mc)
		return nil
	}

	// Parse: BMC /Artifact
	// Note: ParseContentStream auto-closes unclosed BMC with EMC at EOF
	err := r.ParseContentStream(stringOpener("/Artifact BMC"))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	// Should get Begin and End events (due to auto-close)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	if events[0] != MarkedContentBegin {
		t.Errorf("events[0] = %v, want MarkedContentBegin", events[0])
	}

	if events[1] != MarkedContentEnd {
		t.Errorf("events[1] = %v, want MarkedContentEnd", events[1])
	}

	if mcs[0].Tag != "Artifact" {
		t.Errorf("mc.Tag = %q, want \"Artifact\"", mcs[0].Tag)
	}

	if mcs[0].Properties != nil {
		t.Errorf("mc.Properties = %v, want nil", mcs[0].Properties)
	}

	// Same object should be passed to both callbacks
	if mcs[0] != mcs[1] {
		t.Error("Begin and End should receive same MarkedContent object")
	}

	// Stack should be empty after auto-close
	if len(r.MarkedContentStack) != 0 {
		t.Errorf("MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}
}

func TestDPOperatorInline(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var called bool
	var gotEvent MarkedContentEvent
	var gotMC *graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		called = true
		gotEvent = event
		gotMC = mc
		return nil
	}

	// Parse: DP /Artifact <</Type /Pagination>>
	err := r.ParseContentStream(stringOpener("/Artifact <</Type /Pagination>> DP"))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	if !called {
		t.Fatal("MarkedContent callback was not called")
	}

	if gotEvent != MarkedContentPoint {
		t.Errorf("event = %v, want MarkedContentPoint", gotEvent)
	}

	if gotMC.Tag != "Artifact" {
		t.Errorf("mc.Tag = %q, want \"Artifact\"", gotMC.Tag)
	}

	if gotMC.Properties == nil {
		t.Fatal("mc.Properties should not be nil")
	}

	if !gotMC.Inline {
		t.Error("mc.Inline should be true for inline dict")
	}

	// verify via AsDirectDict
	dict := gotMC.Properties.AsDirectDict()
	if dict == nil {
		t.Fatal("AsDirectDict() returned nil for inline dict")
	}
	if dict["Type"] != pdf.Name("Pagination") {
		t.Errorf("Type = %v, want Pagination", dict["Type"])
	}
}

func TestBDCOperator(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var events []MarkedContentEvent
	var mcs []*graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		events = append(events, event)
		mcs = append(mcs, mc)
		return nil
	}

	// Parse: BDC /Span <</Lang (en-US)>>
	// Note: ParseContentStream auto-closes unclosed BDC with EMC at EOF
	err := r.ParseContentStream(stringOpener("/Span <</Lang (en-US)>> BDC"))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	// Should get Begin and End events (due to auto-close)
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	if events[0] != MarkedContentBegin {
		t.Errorf("events[0] = %v, want MarkedContentBegin", events[0])
	}

	if events[1] != MarkedContentEnd {
		t.Errorf("events[1] = %v, want MarkedContentEnd", events[1])
	}

	if mcs[0].Tag != "Span" {
		t.Errorf("mc.Tag = %q, want \"Span\"", mcs[0].Tag)
	}

	if mcs[0].Properties == nil {
		t.Fatal("mc.Properties should not be nil")
	}

	if !mcs[0].Inline {
		t.Error("mc.Inline should be true for inline dict")
	}

	// Same object should be passed to both callbacks
	if mcs[0] != mcs[1] {
		t.Error("Begin and End should receive same MarkedContent object")
	}

	// Stack should be empty after auto-close
	if len(r.MarkedContentStack) != 0 {
		t.Errorf("MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}

	// verify Lang value via AsDirectDict
	dict := mcs[0].Properties.AsDirectDict()
	if dict == nil {
		t.Fatal("AsDirectDict() returned nil for inline dict")
	}
	if string(dict["Lang"].(pdf.String)) != "en-US" {
		t.Errorf("Lang = %v, want en-US", dict["Lang"])
	}
}

func TestEMCOperator(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var events []MarkedContentEvent
	var mcs []*graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		events = append(events, event)
		mcs = append(mcs, mc)
		return nil
	}

	// Parse: BMC /Artifact EMC
	err := r.ParseContentStream(stringOpener("/Artifact BMC\nEMC"))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("Got %d events, want 2", len(events))
	}

	if events[0] != MarkedContentBegin {
		t.Errorf("events[0] = %v, want MarkedContentBegin", events[0])
	}

	if events[1] != MarkedContentEnd {
		t.Errorf("events[1] = %v, want MarkedContentEnd", events[1])
	}

	// Same object should be passed to both callbacks
	if mcs[0] != mcs[1] {
		t.Error("Begin and End should receive same MarkedContent object")
	}

	// Stack should be empty after EMC
	if len(r.MarkedContentStack) != 0 {
		t.Errorf("MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}
}

func TestUnmatchedEMC(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var callCount int
	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		callCount++
		return nil
	}

	// Parse: EMC without BMC/BDC
	err := r.ParseContentStream(stringOpener("EMC"))
	if err != nil {
		t.Fatalf("ParseContentStream should not fail on unmatched EMC: %v", err)
	}

	// Callback should not be called (stack was empty)
	if callCount != 0 {
		t.Errorf("Callback called %d times, want 0 (should silently ignore unmatched EMC)", callCount)
	}
}

func TestMarkedContentStackOverflow(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var beginCount, endCount int
	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		switch event {
		case MarkedContentBegin:
			beginCount++
		case MarkedContentEnd:
			endCount++
		}
		return nil
	}

	// Try to push 100 levels (should stop at maxMarkedContentDepth = 64)
	// Note: ReadStream auto-closes at EOF, so all pushed items will be popped
	var content strings.Builder
	for range 100 {
		content.WriteString("/Test BMC\n")
	}

	err := r.ParseContentStream(stringOpener(content.String()))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	// Should only have called begin callback 64 times (depth limit)
	if beginCount != 64 {
		t.Errorf("Begin callback called %d times, want 64", beginCount)
	}

	// Auto-close should have called end callback 64 times
	if endCount != 64 {
		t.Errorf("End callback called %d times, want 64", endCount)
	}

	// Stack should be empty after auto-close
	if len(r.MarkedContentStack) != 0 {
		t.Errorf("MarkedContentStack length = %d, want 0", len(r.MarkedContentStack))
	}
}

func TestNestedMarkedContent(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	type eventRecord struct {
		event MarkedContentEvent
		tag   pdf.Name
		depth int
	}

	var records []eventRecord

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		records = append(records, eventRecord{
			event: event,
			tag:   mc.Tag,
			depth: len(r.MarkedContentStack),
		})
		return nil
	}

	// Parse nested structure:
	// /Outer BMC
	//   /Inner BMC
	//   EMC
	// EMC
	err := r.ParseContentStream(stringOpener("/Outer BMC\n/Inner BMC\nEMC\nEMC"))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	expected := []eventRecord{
		{MarkedContentBegin, "Outer", 1}, // depth after push
		{MarkedContentBegin, "Inner", 2}, // depth after push
		{MarkedContentEnd, "Inner", 1},   // depth after pop
		{MarkedContentEnd, "Outer", 0},   // depth after pop
	}

	if len(records) != len(expected) {
		t.Fatalf("Got %d events, want %d", len(records), len(expected))
	}

	for i, exp := range expected {
		got := records[i]
		if got.event != exp.event || got.tag != exp.tag || got.depth != exp.depth {
			t.Errorf("Event %d: got {%v, %q, depth=%d}, want {%v, %q, depth=%d}",
				i, got.event, got.tag, got.depth, exp.event, exp.tag, exp.depth)
		}
	}
}

func TestMalformedPropertyExtraction(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.Reset()

	var callCount int
	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		callCount++
		return nil
	}

	// Try to parse BDC with non-dict, non-name operand (should be skipped)
	// This is malformed according to PDF spec
	err := r.ParseContentStream(stringOpener("/Test 123 BDC"))
	if err != nil {
		t.Fatalf("ParseContentStream should not fail on malformed properties: %v", err)
	}

	// The BDC should still process (with properties as the integer)
	// ExtractList will handle it or fail gracefully
	// This test documents current behavior

	// Stack should be empty (malformed BDC was skipped or had nil properties)
	t.Logf("Stack depth: %d, callback count: %d", len(r.MarkedContentStack), callCount)
}
