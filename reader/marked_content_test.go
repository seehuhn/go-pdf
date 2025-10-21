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
	"strings"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

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
	r := New(nil, nil)

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
	r := New(nil, nil)

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
	r := New(w, nil)
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
	contentStream := strings.NewReader("/Caption MP")
	err := r.ParseContentStream(contentStream)
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
	r := New(w, nil)
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

	// Parse: BMC /Artifact
	contentStream := strings.NewReader("/Artifact BMC")
	err := r.ParseContentStream(contentStream)
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	if !called {
		t.Fatal("MarkedContent callback was not called")
	}

	if gotEvent != MarkedContentBegin {
		t.Errorf("event = %v, want MarkedContentBegin", gotEvent)
	}

	if gotMC.Tag != "Artifact" {
		t.Errorf("mc.Tag = %q, want \"Artifact\"", gotMC.Tag)
	}

	if gotMC.Properties != nil {
		t.Errorf("mc.Properties = %v, want nil", gotMC.Properties)
	}

	// Stack should have one element (BMC pushes)
	if len(r.MarkedContentStack) != 1 {
		t.Fatalf("MarkedContentStack length = %d, want 1", len(r.MarkedContentStack))
	}

	if r.MarkedContentStack[0] != gotMC {
		t.Error("Stack element should be same object passed to callback")
	}
}

func TestDPOperatorInline(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(w, nil)
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
	contentStream := strings.NewReader("/Artifact <</Type /Pagination>> DP")
	err := r.ParseContentStream(contentStream)
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

	// Verify we can get the Type key
	val, err := gotMC.Properties.Get("Type")
	if err != nil {
		t.Fatalf("Get(Type) failed: %v", err)
	}

	if val.AsPDF(0) != pdf.Name("Pagination") {
		t.Errorf("Type = %v, want Pagination", val.AsPDF(0))
	}
}

func TestDPOperatorResourceReference(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(w, nil)
	r.Reset()

	// Set up Resources with a property list
	propDict := pdf.Dict{"MCID": pdf.Integer(42)}
	r.Resources = &pdf.Resources{
		Properties: map[pdf.Name]pdf.Object{
			"P1": propDict,
		},
	}

	var called bool
	var gotMC *graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		called = true
		gotMC = mc
		return nil
	}

	// Parse: DP /Span /P1
	contentStream := strings.NewReader("/Span /P1 DP")
	err := r.ParseContentStream(contentStream)
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	if !called {
		t.Fatal("MarkedContent callback was not called")
	}

	if gotMC.Tag != "Span" {
		t.Errorf("mc.Tag = %q, want \"Span\"", gotMC.Tag)
	}

	if gotMC.Properties == nil {
		t.Fatal("mc.Properties should not be nil")
	}

	if gotMC.Inline {
		t.Error("mc.Inline should be false for resource reference")
	}

	// Verify MCID value
	val, err := gotMC.Properties.Get("MCID")
	if err != nil {
		t.Fatalf("Get(MCID) failed: %v", err)
	}

	if val.AsPDF(0) != pdf.Integer(42) {
		t.Errorf("MCID = %v, want 42", val.AsPDF(0))
	}
}

func TestBDCOperator(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(w, nil)
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

	// Parse: BDC /Span <</Lang (en-US)>>
	contentStream := strings.NewReader("/Span <</Lang (en-US)>> BDC")
	err := r.ParseContentStream(contentStream)
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	if !called {
		t.Fatal("MarkedContent callback was not called")
	}

	if gotEvent != MarkedContentBegin {
		t.Errorf("event = %v, want MarkedContentBegin", gotEvent)
	}

	if gotMC.Tag != "Span" {
		t.Errorf("mc.Tag = %q, want \"Span\"", gotMC.Tag)
	}

	if gotMC.Properties == nil {
		t.Fatal("mc.Properties should not be nil")
	}

	if !gotMC.Inline {
		t.Error("mc.Inline should be true for inline dict")
	}

	// Stack should have one element
	if len(r.MarkedContentStack) != 1 {
		t.Fatalf("MarkedContentStack length = %d, want 1", len(r.MarkedContentStack))
	}

	// Verify Lang value
	val, err := gotMC.Properties.Get("Lang")
	if err != nil {
		t.Fatalf("Get(Lang) failed: %v", err)
	}

	if string(val.AsPDF(0).(pdf.String)) != "en-US" {
		t.Errorf("Lang = %v, want en-US", val.AsPDF(0))
	}
}

func TestEMCOperator(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(w, nil)
	r.Reset()

	var events []MarkedContentEvent
	var mcs []*graphics.MarkedContent

	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		events = append(events, event)
		mcs = append(mcs, mc)
		return nil
	}

	// Parse: BMC /Artifact EMC
	contentStream := strings.NewReader("/Artifact BMC\nEMC")
	err := r.ParseContentStream(contentStream)
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
	r := New(w, nil)
	r.Reset()

	var callCount int
	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		callCount++
		return nil
	}

	// Parse: EMC without BMC/BDC
	contentStream := strings.NewReader("EMC")
	err := r.ParseContentStream(contentStream)
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
	r := New(w, nil)
	r.Reset()

	var beginCount int
	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		if event == MarkedContentBegin {
			beginCount++
		}
		return nil
	}

	// Try to push 100 levels (should stop at maxMarkedContentDepth = 64)
	var content strings.Builder
	for i := 0; i < 100; i++ {
		content.WriteString("/Test BMC\n")
	}

	err := r.ParseContentStream(strings.NewReader(content.String()))
	if err != nil {
		t.Fatalf("ParseContentStream failed: %v", err)
	}

	// Should only have called callback 64 times
	if beginCount != 64 {
		t.Errorf("Begin callback called %d times, want 64", beginCount)
	}

	// Stack should have exactly 64 elements
	if len(r.MarkedContentStack) != 64 {
		t.Errorf("MarkedContentStack length = %d, want 64", len(r.MarkedContentStack))
	}
}

func TestNestedMarkedContent(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(w, nil)
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
	contentStream := strings.NewReader("/Outer BMC\n/Inner BMC\nEMC\nEMC")
	err := r.ParseContentStream(contentStream)
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
	r := New(w, nil)
	r.Reset()

	var callCount int
	r.MarkedContent = func(event MarkedContentEvent, mc *graphics.MarkedContent) error {
		callCount++
		return nil
	}

	// Try to parse BDC with non-dict, non-name operand (should be skipped)
	// This is malformed according to PDF spec
	contentStream := strings.NewReader("/Test 123 BDC")
	err := r.ParseContentStream(contentStream)
	if err != nil {
		t.Fatalf("ParseContentStream should not fail on malformed properties: %v", err)
	}

	// The BDC should still process (with properties as the integer)
	// ExtractList will handle it or fail gracefully
	// This test documents current behavior

	// Stack should be empty (malformed BDC was skipped or had nil properties)
	t.Logf("Stack depth: %d, callback count: %d", len(r.MarkedContentStack), callCount)
}
