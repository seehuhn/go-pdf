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

package reader

import (
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

type actualTextEvent struct {
	event ActualTextEvent
	text  string
}

func collectActualText(t *testing.T, src string) ([]actualTextEvent, *Reader) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))

	var events []actualTextEvent
	r.ActualText = func(event ActualTextEvent, text string) error {
		events = append(events, actualTextEvent{event: event, text: text})
		return nil
	}

	if err := r.ProcessIter(stringStream(src).NewIter()); err != nil {
		t.Fatalf("ProcessIter failed: %v", err)
	}
	return events, r
}

func TestActualTextBeginEnd(t *testing.T) {
	events, r := collectActualText(t,
		"/Span <</ActualText (REPLACEMENT)>> BDC\nEMC\n")

	want := []actualTextEvent{
		{ActualTextBegin, "REPLACEMENT"},
		{ActualTextEnd, "REPLACEMENT"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d", len(events), len(want))
	}
	for i, w := range want {
		if events[i] != w {
			t.Errorf("event %d: got %+v, want %+v", i, events[i], w)
		}
	}
	if r.InActualText() {
		t.Error("InActualText() should be false after region ends")
	}
}

func TestActualTextNoCallback(t *testing.T) {
	// BDC without ActualText property must not fire the callback.
	events, _ := collectActualText(t,
		"/Span <</Lang (en-US)>> BDC\nEMC\n")
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestActualTextNested(t *testing.T) {
	// Inner ActualText is suppressed: only outer Begin/End fires.
	events, _ := collectActualText(t, `/Span <</ActualText (OUTER)>> BDC
/Span <</ActualText (INNER)>> BDC
EMC
EMC
`)

	want := []actualTextEvent{
		{ActualTextBegin, "OUTER"},
		{ActualTextEnd, "OUTER"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, w := range want {
		if events[i] != w {
			t.Errorf("event %d: got %+v, want %+v", i, events[i], w)
		}
	}
}

func TestActualTextInsideRegular(t *testing.T) {
	// ActualText nested inside a non-ActualText span still fires.
	events, _ := collectActualText(t, `/Span <</Lang (en-US)>> BDC
/Span <</ActualText (INNER)>> BDC
EMC
EMC
`)

	want := []actualTextEvent{
		{ActualTextBegin, "INNER"},
		{ActualTextEnd, "INNER"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, w := range want {
		if events[i] != w {
			t.Errorf("event %d: got %+v, want %+v", i, events[i], w)
		}
	}
}

func TestActualTextUnclosedAutoEnd(t *testing.T) {
	// Unclosed BDC at EOF: ProcessIter synthesises EMC, which must
	// surface as an ActualTextEnd.
	events, r := collectActualText(t,
		"/Span <</ActualText (HI)>> BDC")

	want := []actualTextEvent{
		{ActualTextBegin, "HI"},
		{ActualTextEnd, "HI"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, w := range want {
		if events[i] != w {
			t.Errorf("event %d: got %+v, want %+v", i, events[i], w)
		}
	}
	if r.InActualText() {
		t.Error("InActualText() should be false after auto-close")
	}
}

func TestInActualText(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))

	if r.InActualText() {
		t.Error("InActualText() should be false initially")
	}

	// Sample InActualText() at three positions: before BDC, between BDC and
	// EMC, and after EMC.  We use UnknownOp on a sentinel operator name to
	// surface the snapshots in order.
	var samples []bool
	r.UnknownOp = func(op string, args []pdf.Object) error {
		if op == "Probe" {
			samples = append(samples, r.InActualText())
		}
		return nil
	}

	src := "Probe\n/Span <</ActualText (X)>> BDC\nProbe\nEMC\nProbe\n"
	if err := r.ProcessIter(stringStream(src).NewIter()); err != nil {
		t.Fatalf("ProcessIter failed: %v", err)
	}

	want := []bool{false, true, false}
	if len(samples) != len(want) {
		t.Fatalf("got %d samples, want %d", len(samples), len(want))
	}
	for i, w := range want {
		if samples[i] != w {
			t.Errorf("sample %d: got %v, want %v", i, samples[i], w)
		}
	}
}

func TestActualTextResetClears(t *testing.T) {
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))

	r.actualTextStartDepth = 3
	r.actualTextValue = "stale"

	r.Reset()

	if r.actualTextStartDepth != -1 {
		t.Errorf("after Reset, actualTextStartDepth = %d, want -1", r.actualTextStartDepth)
	}
	if r.actualTextValue != "" {
		t.Errorf("after Reset, actualTextValue = %q, want \"\"", r.actualTextValue)
	}
}

func TestActualTextProcessIterResets(t *testing.T) {
	// ProcessIter must reset stale ActualText state from a prior run.
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))

	r.actualTextStartDepth = 7
	r.actualTextValue = "stale"

	if err := r.ProcessIter(stringStream("").NewIter()); err != nil {
		t.Fatalf("ProcessIter failed: %v", err)
	}

	if r.actualTextStartDepth != -1 {
		t.Errorf("after ProcessIter, actualTextStartDepth = %d, want -1", r.actualTextStartDepth)
	}
	if r.actualTextValue != "" {
		t.Errorf("after ProcessIter, actualTextValue = %q, want \"\"", r.actualTextValue)
	}
}
