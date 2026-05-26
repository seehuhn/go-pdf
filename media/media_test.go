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
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/geom/matrix"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/file"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
	"seehuhn.de/go/pdf/graphics/content/builder"
	"seehuhn.de/go/pdf/graphics/form"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/optional"
)

func spec() *file.Specification {
	return &file.Specification{
		FileName:       "movie.mp4",
		AFRelationship: file.RelationshipUnspecified,
	}
}

func clipForm() *form.Form {
	b := builder.New(content.Form, nil, pdf.V2_0)
	b.SetFillColor(color.DeviceGray(0.5))
	b.Rectangle(0, 0, 100, 100)
	b.Fill()
	return &form.Form{
		Content: &content.Operators{Ops: b.Stream},
		Res:     b.Resources,
		BBox:    pdf.Rectangle{URx: 100, URy: 100},
		Matrix:  matrix.Identity,
	}
}

func swid() *SoftwareIdentifier {
	return &SoftwareIdentifier{URI: "vnd.adobe.swname:ADBE_Acrobat", SingleUse: true}
}

// decodeFunc adapts a typed Extract function to one returning any.
type decodeFunc func(*pdf.Extractor, *pdf.CycleCheck, pdf.Object, bool) (any, error)

func adapt[T any](f func(*pdf.Extractor, *pdf.CycleCheck, pdf.Object, bool) (T, error)) decodeFunc {
	return func(x *pdf.Extractor, p *pdf.CycleCheck, o pdf.Object, d bool) (any, error) {
		return f(x, p, o, d)
	}
}

type rtCase struct {
	name   string
	obj    pdf.Embedder
	decode decodeFunc
}

var rtCases = []rtCase{
	{
		name:   "software-identifier-minimal",
		obj:    &SoftwareIdentifier{URI: "vnd.adobe.swname:ADBE_Acrobat"},
		decode: adapt(ExtractSoftwareIdentifier),
	},
	{
		name: "software-identifier-full",
		obj: &SoftwareIdentifier{
			URI:           "vnd.adobe.swname:ADBE_Acrobat",
			Low:           []int{1, 2},
			High:          []int{3},
			LowExclusive:  true,
			HighExclusive: true,
			OS:            []string{"Windows", "Mac OS"},
			SingleUse:     true,
		},
		decode: adapt(ExtractSoftwareIdentifier),
	},
	{
		name:   "timespan",
		obj:    &Timespan{Seconds: 1.5},
		decode: adapt(ExtractTimespan),
	},
	{
		name:   "offset-time",
		obj:    &MediaOffsetTime{Time: &Timespan{Seconds: 10, SingleUse: true}, SingleUse: true},
		decode: adapt(ExtractMediaOffset),
	},
	{
		name:   "offset-frame",
		obj:    &MediaOffsetFrame{Frame: 20},
		decode: adapt(ExtractMediaOffset),
	},
	{
		name:   "offset-marker",
		obj:    &MediaOffsetMarker{Marker: "Chapter One"},
		decode: adapt(ExtractMediaOffset),
	},
	{
		name:   "permissions",
		obj:    &MediaPermissions{TempFile: TempAlways},
		decode: adapt(ExtractMediaPermissions),
	},
	{
		name:   "min-bit-depth",
		obj:    &MinBitDepth{Depth: 24, Monitor: MonitorPrimary},
		decode: adapt(ExtractMinBitDepth),
	},
	{
		name:   "min-screen-size",
		obj:    &MinScreenSize{Width: 640, Height: 480, Monitor: MonitorGreatestWidth},
		decode: adapt(ExtractMinScreenSize),
	},
	{
		name: "criteria",
		obj: &MediaCriteria{
			AudioDescriptions: optional.NewBool(true),
			Captions:          optional.NewBool(false),
			Bandwidth:         optional.NewUInt(56000),
			MinBitDepth:       &MinBitDepth{Depth: 16, SingleUse: true},
			MinScreenSize:     &MinScreenSize{Width: 800, Height: 600, SingleUse: true},
			Software:          []*SoftwareIdentifier{swid()},
			Version:           []pdf.Name{"1.5", "2.0"},
			Languages:         []string{"en-US", "de"},
			SingleUse:         true,
		},
		decode: adapt(ExtractMediaCriteria),
	},
	{
		name: "players",
		obj: &MediaPlayers{
			MustUse: []*MediaPlayerInfo{{PID: swid(), SingleUse: true}},
			NotUsed: []*MediaPlayerInfo{{PID: swid(), SingleUse: true}},
		},
		decode: adapt(ExtractMediaPlayers),
	},
	{
		name:   "player-info",
		obj:    &MediaPlayerInfo{PID: swid()},
		decode: adapt(ExtractMediaPlayerInfo),
	},
	{
		name:   "duration-intrinsic",
		obj:    &MediaDuration{Kind: DurationIntrinsic},
		decode: adapt(ExtractMediaDuration),
	},
	{
		name:   "duration-infinity",
		obj:    &MediaDuration{Kind: DurationInfinity},
		decode: adapt(ExtractMediaDuration),
	},
	{
		name:   "duration-explicit",
		obj:    &MediaDuration{Kind: DurationExplicit, Time: &Timespan{Seconds: 30, SingleUse: true}},
		decode: adapt(ExtractMediaDuration),
	},
	{
		name: "play-parameters",
		obj: &MediaPlayParameters{
			Players: &MediaPlayers{Allowed: []*MediaPlayerInfo{{PID: swid(), SingleUse: true}}, SingleUse: true},
			MustHonour: &MediaPlayEntries{
				Volume:   optional.NewUInt(50),
				Fit:      FitMeet,
				Duration: &MediaDuration{Kind: DurationInfinity, SingleUse: true},
			},
			BestEffort: &MediaPlayEntries{
				Controller:  optional.NewBool(true),
				AutoPlay:    optional.NewBool(false),
				RepeatCount: optional.NewFloat64(0),
			},
		},
		decode: adapt(ExtractMediaPlayParameters),
	},
	{
		name: "floating-window",
		obj: &FloatingWindowParameters{
			Width:        320,
			Height:       240,
			RelativeTo:   RelativeToMonitor,
			Position:     PositionUpperLeft,
			Offscreen:    OffscreenNoAction,
			TitleBar:     optional.NewBool(true),
			UserCanClose: optional.NewBool(false),
			Resizable:    ResizeKeepAspect,
			Title:        MultiLangText{{Lang: "en-US", Text: "Player"}},
		},
		decode: adapt(ExtractFloatingWindowParameters),
	},
	{
		name: "screen-parameters",
		obj: &MediaScreenParameters{
			MustHonour: &MediaScreenEntries{
				Window:         WindowFloating,
				FloatingWindow: &FloatingWindowParameters{Width: 100, Height: 100, SingleUse: true},
			},
			BestEffort: &MediaScreenEntries{
				Background: []float64{0.5, 0.5, 0.5},
				Opacity:    optional.NewFloat64(0.75),
				Monitor:    MonitorPrimary,
			},
		},
		decode: adapt(ExtractMediaScreenParameters),
	},
	{
		name: "clip-data-file",
		obj: &MediaClipData{
			Name:              "clip",
			DataFile:          spec(),
			ContentType:       "video/mp4",
			Permissions:       &MediaPermissions{TempFile: TempExtract, SingleUse: true},
			Alt:               MultiLangText{{Lang: "en", Text: "a movie"}},
			Players:           &MediaPlayers{MustUse: []*MediaPlayerInfo{{PID: swid(), SingleUse: true}}, SingleUse: true},
			MustHonourBaseURL: "https://example.com/",
			BestEffortBaseURL: "https://example.org/",
		},
		decode: adapt(ExtractMediaClip),
	},
	{
		name: "clip-data-form",
		obj: &MediaClipData{
			Name:     "form-clip",
			DataForm: clipForm(),
			Alt:      MultiLangText{{Lang: "en", Text: "a form"}},
		},
		decode: adapt(ExtractMediaClip),
	},
	{
		name: "clip-section",
		obj: &MediaClipSection{
			Name:            "section",
			Next:            &MediaClipData{DataFile: spec(), SingleUse: true},
			Alt:             MultiLangText{{Lang: "en", Text: "part"}},
			MustHonourBegin: &MediaOffsetTime{Time: &Timespan{Seconds: 5, SingleUse: true}, SingleUse: true},
			BestEffortEnd:   &MediaOffsetFrame{Frame: 100, SingleUse: true},
		},
		decode: adapt(ExtractMediaClip),
	},
	{
		name: "media-rendition",
		obj: &MediaRendition{
			RenditionCommon: RenditionCommon{
				Name:               "video",
				MustHonourCriteria: &MediaCriteria{Bandwidth: optional.NewUInt(1000000), SingleUse: true},
			},
			Clip: &MediaClipData{DataFile: spec(), SingleUse: true},
			Play: &MediaPlayParameters{
				MustHonour: &MediaPlayEntries{Volume: optional.NewUInt(80)},
				SingleUse:  true,
			},
			Screen: &MediaScreenParameters{
				MustHonour: &MediaScreenEntries{Window: WindowFullScreen},
				SingleUse:  true,
			},
		},
		decode: adapt(ExtractRendition),
	},
	{
		name: "selector-rendition",
		obj: &SelectorRendition{
			RenditionCommon: RenditionCommon{Name: "choices"},
			Renditions: []Rendition{
				&MediaRendition{Clip: &MediaClipData{DataFile: spec(), SingleUse: true}, SingleUse: true},
				&SelectorRendition{SingleUse: true},
			},
		},
		decode: adapt(ExtractRendition),
	},
	{
		name:   "rendition-single-use",
		obj:    &MediaRendition{Clip: &MediaClipData{DataFile: spec(), SingleUse: true}, SingleUse: true},
		decode: adapt(ExtractRendition),
	},
}

func roundTrip(t *testing.T, version pdf.Version, obj pdf.Embedder, decode decodeFunc) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)
	stored, err := rm.Embed(obj)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed: %v", err)
	}
	if err := rm.Close(); err != nil {
		t.Fatalf("rm.Close: %v", err)
	}
	w.GetMeta().Trailer["Quir:E"] = stored
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	x := pdf.NewExtractor(w)
	s := w.GetMeta().Trailer["Quir:E"]
	_, isRef := s.(pdf.Reference)
	got, err := decode(x, nil, s, !isRef)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// form XObjects carry resource references and SingleUse flags that differ
	// after a round trip, so compare them with form.Equal.
	formCmp := cmp.Comparer(func(a, b *form.Form) bool {
		if a == nil || b == nil {
			return a == b
		}
		return a.Equal(b)
	})
	if diff := cmp.Diff(obj, got, formCmp); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		t.Run(version.String(), func(t *testing.T) {
			for _, tc := range rtCases {
				t.Run(tc.name, func(t *testing.T) {
					roundTrip(t, version, tc.obj, tc.decode)
				})
			}
		})
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}
	for _, version := range []pdf.Version{pdf.V1_7, pdf.V2_0} {
		for _, tc := range rtCases {
			r, ok := tc.obj.(Rendition)
			if !ok {
				continue
			}
			w, buf := memfile.NewPDFWriter(version, opt)
			if err := memfile.AddBlankPage(w); err != nil {
				continue
			}
			rm := pdf.NewResourceManager(w)
			stored, err := rm.Embed(r)
			if err != nil {
				continue
			}
			if err := rm.Close(); err != nil {
				continue
			}
			w.GetMeta().Trailer["Quir:E"] = stored
			if err := w.Close(); err != nil {
				continue
			}
			f.Add(buf.Data)
		}
	}

	f.Fuzz(func(t *testing.T, fileData []byte) {
		r, err := pdf.NewReader(bytes.NewReader(fileData), int64(len(fileData)), nil)
		if err != nil {
			t.Skip("invalid PDF")
		}
		obj := r.GetMeta().Trailer["Quir:E"]
		if obj == nil {
			t.Skip("missing object")
		}
		x := pdf.NewExtractor(r)
		_, isRef := obj.(pdf.Reference)
		first, err := ExtractRendition(x, nil, obj, !isRef)
		if err != nil {
			t.Skip("malformed rendition")
		}
		roundTrip(t, pdf.GetVersion(r), first, adapt(ExtractRendition))
	})
}
