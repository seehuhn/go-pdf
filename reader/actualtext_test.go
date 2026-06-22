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
	"errors"
	"testing"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

// errAbortActualText is returned by a test ActualText callback to abort a run
// mid-region, leaving the reader's ActualText state dirty.
var errAbortActualText = errors.New("abort")

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
	// EMC, and after EMC.  We use EveryOp on a sentinel operator name to
	// surface the snapshots in order.
	var samples []bool
	r.EveryOp = func(op string, args []pdf.Object) error {
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
	// Reset must clear ActualText state left dirty by an aborted run.  A
	// callback that fails on the Begin event aborts ProcessIter before the
	// region's auto-close runs, leaving it open; Reset then returns the reader
	// to a clean state.
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))
	r.ActualText = func(ActualTextEvent, string) error {
		return errAbortActualText
	}

	if err := r.ProcessIter(stringStream("/Span <</ActualText (STALE)>> BDC\nEMC\n").NewIter()); err == nil {
		t.Fatal("expected the callback error to abort ProcessIter")
	}
	if !r.InActualText() {
		t.Fatal("test setup: an aborted run should leave the region open")
	}

	r.Reset()

	if r.InActualText() {
		t.Error("InActualText() true after Reset, want false")
	}
}

func TestActualTextProcessIterResets(t *testing.T) {
	// A new ProcessIter must not inherit ActualText state left dirty by an
	// aborted run.  The first run's callback fails mid-region, leaving it open;
	// without the reset at the start of ProcessIter the second run's region
	// would be treated as nested and suppressed instead of firing its own
	// Begin/End events.
	w, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	r := New(pdf.NewExtractor(w))

	var events []actualTextEvent
	fail := true
	r.ActualText = func(event ActualTextEvent, text string) error {
		if fail {
			return errAbortActualText
		}
		events = append(events, actualTextEvent{event: event, text: text})
		return nil
	}

	if err := r.ProcessIter(stringStream("/Span <</ActualText (STALE)>> BDC\nEMC\n").NewIter()); err == nil {
		t.Fatal("expected the callback error to abort the first run")
	}

	fail = false
	if err := r.ProcessIter(stringStream("/Span <</ActualText (FRESH)>> BDC\nEMC\n").NewIter()); err != nil {
		t.Fatalf("second ProcessIter failed: %v", err)
	}

	want := []actualTextEvent{
		{ActualTextBegin, "FRESH"},
		{ActualTextEnd, "FRESH"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i, wnt := range want {
		if events[i] != wnt {
			t.Errorf("event %d: got %+v, want %+v", i, events[i], wnt)
		}
	}
}
