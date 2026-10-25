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

package media

import (
	"maps"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/internal/debug/mock"
	"seehuhn.de/go/pdf/optional"
)

// embed is a test helper that embeds obj into a fresh writer and returns the
// resulting native value or error.
func embed(t *testing.T, version pdf.Version, obj pdf.Embedder) (pdf.Native, error) {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, version, nil)
	rm := pdf.NewResourceManager(w)
	return rm.Embed(obj)
}

// TestEnumRoundTrip exercises every enumerated value so the PDF<->Go
// converters are fully covered.
func TestEnumRoundTrip(t *testing.T) {
	var cases []pdf.Embedder

	for _, fit := range []FitMode{FitUnspecified, FitMeet, FitSlice, FitFill, FitScroll, FitHidden} {
		cases = append(cases, &MediaPlayParameters{
			MustHonour: &MediaPlayEntries{Fit: fit, Volume: optional.NewUInt(1)},
			SingleUse:  true,
		})
	}
	for _, w := range []WindowType{WindowAnnotation, WindowFloating, WindowFullScreen, WindowHidden} {
		entry := &MediaScreenEntries{Window: w, Opacity: optional.NewFloat64(1)}
		if w == WindowFloating {
			entry.FloatingWindow = &FloatingWindowParameters{Width: 10, Height: 10, SingleUse: true}
		}
		cases = append(cases, &MediaScreenParameters{MustHonour: entry, SingleUse: true})
	}
	positions := []WindowPosition{
		PositionCentre, PositionUpperLeft, PositionUpperCentre, PositionUpperRight,
		PositionCentreLeft, PositionCentreRight, PositionLowerLeft, PositionLowerCentre,
		PositionLowerRight,
	}
	for _, p := range positions {
		for _, o := range []OffscreenAction{OffscreenMoveOnscreen, OffscreenNoAction, OffscreenNonViable} {
			for _, rt := range []WindowRelativeTo{RelativeToDocument, RelativeToApplication, RelativeToDesktop, RelativeToMonitor} {
				cases = append(cases, &FloatingWindowParameters{
					Width: 1, Height: 1, Position: p, Offscreen: o, RelativeTo: rt,
					Resizable: ResizeFree, SingleUse: true,
				})
			}
		}
	}
	for _, tf := range []TempFilePermission{TempNever, TempExtract, TempAccess, TempAlways} {
		cases = append(cases, &MediaPermissions{TempFile: tf, SingleUse: true})
	}
	for _, m := range []MonitorSpecifier{
		MonitorLargestDocument, MonitorSmallestDocument, MonitorPrimary,
		MonitorGreatestDepth, MonitorGreatestArea, MonitorGreatestHeight, MonitorGreatestWidth,
	} {
		cases = append(cases, &MinBitDepth{Depth: 8, Monitor: m, SingleUse: true})
	}

	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for i, obj := range cases {
			t.Run(version.String(), func(t *testing.T) {
				var dec decodeFunc
				switch obj.(type) {
				case *MediaPlayParameters:
					dec = adapt(ExtractMediaPlayParameters)
				case *MediaScreenParameters:
					dec = adapt(ExtractMediaScreenParameters)
				case *FloatingWindowParameters:
					dec = adapt(ExtractFloatingWindowParameters)
				case *MediaPermissions:
					dec = adapt(ExtractMediaPermissions)
				case *MinBitDepth:
					dec = adapt(ExtractMinBitDepth)
				default:
					t.Fatalf("case %d: unexpected type %T", i, obj)
				}
				roundTrip(t, version, obj, dec)
			})
		}
	}
}

// TestEmbedErrors checks that invalid values are rejected when writing.
func TestEmbedErrors(t *testing.T) {
	cases := []struct {
		name string
		obj  pdf.Embedder
	}{
		{"empty software URI", &SoftwareIdentifier{}},
		{"negative version", &SoftwareIdentifier{URI: "x", Low: []int{-1}}},
		{"negative high version", &SoftwareIdentifier{URI: "x", High: []int{-1}}},
		{"negative timespan", &Timespan{Seconds: -1}},
		{"negative frame", &MediaOffsetFrame{Frame: -1}},
		{"invalid permission", &MediaPermissions{TempFile: "bogus"}},
		{"zero bit depth", &MinBitDepth{Depth: 0}},
		{"negative screen size", &MinScreenSize{Width: -1, Height: 1}},
		{"missing offset time", &MediaOffsetTime{}},
		{"missing player PID", &MediaPlayerInfo{}},
		{"duration explicit without time", &MediaDuration{Kind: DurationExplicit}},
		{"duration invalid kind", &MediaDuration{Kind: "X"}},
		{"clip neither data", &MediaClipData{}},
		{"section without next", &MediaClipSection{}},
		{"floating negative size", &FloatingWindowParameters{Width: -1, Height: 1}},
		{"screen opacity out of range", &MediaScreenParameters{
			MustHonour: &MediaScreenEntries{Opacity: optional.NewFloat64(2)},
			SingleUse:  true,
		}},
		{"screen floating required", &MediaScreenParameters{
			MustHonour: &MediaScreenEntries{Window: WindowFloating},
			SingleUse:  true,
		}},
		{"negative repeat count", &MediaPlayParameters{
			MustHonour: &MediaPlayEntries{RepeatCount: optional.NewFloat64(-1)},
			SingleUse:  true,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := embed(t, pdf.V2_0, tc.obj); err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestVersionCheck verifies that media objects are rejected before PDF 1.5.
func TestVersionCheck(t *testing.T) {
	objs := []pdf.Embedder{
		&Timespan{Seconds: 1},
		&SoftwareIdentifier{URI: "x"},
		&MediaPermissions{},
		&MediaOffsetFrame{Frame: 1},
		&MinBitDepth{Depth: 8},
		&MinScreenSize{Width: 1, Height: 1},
		&MediaCriteria{},
		&MediaPlayers{},
		&MediaPlayerInfo{PID: &SoftwareIdentifier{URI: "x", SingleUse: true}},
		&MediaDuration{Kind: DurationIntrinsic},
		&MediaPlayParameters{},
		&FloatingWindowParameters{Width: 1, Height: 1},
		&MediaScreenParameters{},
		&MediaClipData{DataFile: spec()},
		&MediaRendition{},
		&SelectorRendition{},
	}
	for _, obj := range objs {
		if _, err := embed(t, pdf.V1_0, obj); !pdf.IsWrongVersion(err) {
			t.Errorf("%T: expected version error, got %v", obj, err)
		}
	}
}

// TestReadMalformed checks that malformed or unknown input is handled
// gracefully (an error that callers treat as "absent", never a panic).
func TestReadMalformed(t *testing.T) {
	x := pdf.NewExtractor(memfileGetter(t))

	mustErr := func(name string, err error) {
		if err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	_, err := ExtractRendition(pdf.CursorAt(x, nil), pdf.Dict{"S": pdf.Name("ZZ")}, true)
	mustErr("rendition unknown subtype", err)
	_, err = ExtractMediaClip(pdf.CursorAt(x, nil), pdf.Dict{"S": pdf.Name("ZZ")}, true)
	mustErr("clip unknown subtype", err)
	_, err = ExtractMediaOffset(pdf.CursorAt(x, nil), pdf.Dict{"S": pdf.Name("ZZ")}, true)
	mustErr("offset unknown subtype", err)
	_, err = ExtractMediaDuration(pdf.CursorAt(x, nil), pdf.Dict{"S": pdf.Name("ZZ")}, true)
	mustErr("duration unknown subtype", err)
	_, err = ExtractSoftwareIdentifier(pdf.CursorAt(x, nil), pdf.Dict{}, true)
	mustErr("software identifier missing U", err)
	_, err = ExtractTimespan(pdf.CursorAt(x, nil), pdf.Dict{"V": pdf.Number(-1)}, true)
	mustErr("timespan negative", err)
	_, err = ExtractMinBitDepth(pdf.CursorAt(x, nil), pdf.Dict{"V": pdf.Integer(0)}, true)
	mustErr("min bit depth zero", err)
	_, err = ExtractMinScreenSize(pdf.CursorAt(x, nil), pdf.Dict{"V": pdf.Array{pdf.Integer(1)}}, true)
	mustErr("min screen size short", err)

	// a version array containing a negative number is dropped (treated as absent)
	s, err := ExtractSoftwareIdentifier(pdf.CursorAt(x, nil), pdf.Dict{
		"U": pdf.String("x"),
		"L": pdf.Array{pdf.Integer(-1)},
	}, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Low != nil {
		t.Errorf("expected negative version array to be dropped, got %v", s.Low)
	}
}

// TestMarkers calls the unexported interface marker methods.
func TestMarkers(t *testing.T) {
	(&MediaRendition{}).isRendition()
	(&SelectorRendition{}).isRendition()
	(&MediaClipData{}).isMediaClip()
	(&MediaClipSection{}).isMediaClip()
	(&MediaOffsetTime{}).isMediaOffset()
	(&MediaOffsetFrame{}).isMediaOffset()
	(&MediaOffsetMarker{}).isMediaOffset()
}

func memfileGetter(t *testing.T) pdf.Getter {
	t.Helper()
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	return w
}

// TestInvalidEntryIsAbsent checks that a reference to a nonexistent object,
// and a value of the wrong type, are read like an absent entry.
func TestInvalidEntryIsAbsent(t *testing.T) {
	invalid := map[string]pdf.Object{
		"dangling": pdf.NewReference(99, 0),
		"name":     pdf.Name("x"),
		"integer":  pdf.Integer(5),
	}
	c := pdf.NewCursor(mock.Getter)

	fwBase := pdf.Dict{"D": pdf.Array{pdf.Integer(100), pdf.Integer(200)}}
	wrapMH := func(key pdf.Name, entries pdf.Dict) pdf.Dict {
		return pdf.Dict{key: entries}
	}

	// boolEntry marks boolean entries, for which an integer is a wrong type
	type entryCase struct {
		name      string
		key       pdf.Name
		base      pdf.Dict
		boolEntry bool
		decode    func(pdf.Dict) (any, error)
	}
	decodeSoftware := func(d pdf.Dict) (any, error) { return ExtractSoftwareIdentifier(c, d, true) }
	decodeCriteria := func(d pdf.Dict) (any, error) { return ExtractMediaCriteria(c, d, true) }
	decodeFW := func(d pdf.Dict) (any, error) { return ExtractFloatingWindowParameters(c, d, true) }
	decodePlay := func(d pdf.Dict) (any, error) { return ExtractMediaPlayParameters(c, wrapMH("MH", d), true) }
	decodeScreen := func(d pdf.Dict) (any, error) { return ExtractMediaScreenParameters(c, wrapMH("MH", d), true) }

	swBase := pdf.Dict{"U": pdf.String("app")}
	playBase := pdf.Dict{"M": pdf.Integer(1)}
	cases := []entryCase{
		{"softwareid LI", "LI", swBase, true, decodeSoftware},
		{"softwareid HI", "HI", swBase, true, decodeSoftware},
		{"criteria A", "A", pdf.Dict{}, true, decodeCriteria},
		{"criteria C", "C", pdf.Dict{}, true, decodeCriteria},
		{"criteria O", "O", pdf.Dict{}, true, decodeCriteria},
		{"criteria S", "S", pdf.Dict{}, true, decodeCriteria},
		{"criteria R", "R", pdf.Dict{}, false, decodeCriteria},
		{"floating window P", "P", fwBase, false, decodeFW},
		{"floating window O", "O", fwBase, false, decodeFW},
		{"floating window T", "T", fwBase, true, decodeFW},
		{"floating window UC", "UC", fwBase, true, decodeFW},
		{"play entries V", "V", playBase, false, decodePlay},
		{"play entries C", "C", playBase, true, decodePlay},
		{"play entries F", "F", playBase, false, decodePlay},
		{"play entries A", "A", playBase, true, decodePlay},
		{"play entries RC", "RC", playBase, false, decodePlay},
		{"screen entries W", "W", playBase, false, decodeScreen},
		{"screen entries O", "O", playBase, false, decodeScreen},
	}

	for _, tc := range cases {
		for variant, value := range invalid {
			if variant == "integer" && !tc.boolEntry {
				continue
			}
			t.Run(tc.name+" "+variant, func(t *testing.T) {
				absent := maps.Clone(tc.base)
				withInvalid := maps.Clone(tc.base)
				withInvalid[tc.key] = value

				want, err := tc.decode(absent)
				if err != nil {
					t.Fatal(err)
				}
				got, err := tc.decode(withInvalid)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("%s value read differently from absent entry (-absent +got):\n%s", variant, diff)
				}
			})
		}
	}
}

// TestRequiredEntryMissing checks that a missing, dangling or non-numeric
// required value fails the read rather than reading as zero, and that an
// optional parent drops the failed offset.
func TestRequiredEntryMissing(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	bad := map[string]pdf.Object{
		"missing":  nil,
		"dangling": pdf.NewReference(99, 0),
		"name":     pdf.Name("x"),
	}
	for name, v := range bad {
		timespan := pdf.Dict{"S": pdf.Name("S")}
		frame := pdf.Dict{"S": pdf.Name("F")}
		if v != nil {
			timespan["V"] = v
			frame["F"] = v
		}

		t.Run("timespan "+name, func(t *testing.T) {
			if _, err := ExtractTimespan(c, timespan, true); !pdf.IsMalformed(err) {
				t.Errorf("got error %v, want MalformedFileError", err)
			}
		})
		t.Run("time offset "+name, func(t *testing.T) {
			dict := pdf.Dict{"S": pdf.Name("T"), "T": timespan}
			if _, err := ExtractMediaOffset(c, dict, true); !pdf.IsMalformed(err) {
				t.Errorf("got error %v, want MalformedFileError", err)
			}
		})
		t.Run("frame offset "+name, func(t *testing.T) {
			if _, err := ExtractMediaOffset(c, frame, true); !pdf.IsMalformed(err) {
				t.Errorf("got error %v, want MalformedFileError", err)
			}
		})
		t.Run("section "+name, func(t *testing.T) {
			// the invalid begin offset is dropped, the valid end offset is kept
			sec := pdf.Dict{
				"B": frame,
				"E": pdf.Dict{"S": pdf.Name("F"), "F": pdf.Integer(7)},
			}
			begin, end, err := extractSectionOffsets(c, sec)
			if err != nil {
				t.Fatal(err)
			}
			if begin != nil {
				t.Errorf("got begin offset %v, want nil", begin)
			}
			if f, ok := end.(*MediaOffsetFrame); !ok || f.Frame != 7 {
				t.Errorf("got end offset %v, want frame 7", end)
			}
		})
	}

	// zero is a valid value
	ts, err := ExtractTimespan(c, pdf.Dict{"V": pdf.Integer(0)}, true)
	if err != nil || ts.Seconds != 0 {
		t.Errorf("got %v, %v, want zero timespan", ts, err)
	}
	of, err := ExtractMediaOffset(c, pdf.Dict{"S": pdf.Name("F"), "F": pdf.Integer(0)}, true)
	if err != nil {
		t.Fatal(err)
	}
	if f, ok := of.(*MediaOffsetFrame); !ok || f.Frame != 0 {
		t.Errorf("got %v, want frame 0", of)
	}
}

// TestVersionArrayInvalidElement checks that a version array with a
// non-integer or negative element is dropped as a whole.
func TestVersionArrayInvalidElement(t *testing.T) {
	c := pdf.NewCursor(mock.Getter)
	cases := map[string]pdf.Array{
		"name":     {pdf.Integer(1), pdf.Name("x"), pdf.Integer(3)},
		"string":   {pdf.String("1")},
		"null":     {pdf.Integer(1), nil},
		"dangling": {pdf.Integer(1), pdf.NewReference(99, 0)},
		"negative": {pdf.Integer(1), pdf.Integer(-2)},
	}
	for name, arr := range cases {
		t.Run(name, func(t *testing.T) {
			s, err := ExtractSoftwareIdentifier(c, pdf.Dict{
				"U": pdf.String("app"),
				"L": arr,
				"H": arr,
			}, true)
			if err != nil {
				t.Fatal(err)
			}
			if s.Low != nil || s.High != nil {
				t.Errorf("got Low=%v High=%v, want both dropped", s.Low, s.High)
			}
		})
	}

	s, err := ExtractSoftwareIdentifier(c, pdf.Dict{
		"U": pdf.String("app"),
		"L": pdf.Array{pdf.Integer(1), pdf.Integer(0), pdf.Integer(3)},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Low, []int{1, 0, 3}) {
		t.Errorf("got Low=%v, want [1 0 3]", s.Low)
	}
}
