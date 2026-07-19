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

package trapnet

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
	"seehuhn.de/go/pdf/opaque"
)

var testCases = []struct {
	name  string
	attrs *Attributes
}{
	{
		name:  "PCM only",
		attrs: &Attributes{PCM: "DeviceCMYK"},
	},
	{
		name:  "DeviceN",
		attrs: &Attributes{PCM: "DeviceN"},
	},
	{
		name: "separation colorant names",
		attrs: &Attributes{
			PCM:                  "DeviceCMYK",
			SeparationColorNames: []pdf.Name{"Spot1", "Spot2"},
		},
	},
	{
		name: "trap styles",
		attrs: &Attributes{
			PCM:        "DeviceGray",
			TrapStyles: "default trapping",
		},
	},
	{
		name: "trap regions",
		attrs: &Attributes{
			PCM: "DeviceCMYK",
			TrapRegions: []*opaque.Object{
				opaque.Direct(pdf.Dict{"Foo": pdf.Integer(1)}),
			},
		},
	},
	{
		name: "all entries",
		attrs: &Attributes{
			PCM:                  "DeviceRGBK",
			SeparationColorNames: []pdf.Name{"Varnish"},
			TrapRegions: []*opaque.Object{
				opaque.Direct(pdf.Dict{"Bar": pdf.Name("baz")}),
			},
			TrapStyles: "wide traps",
		},
	},
}

// carrier writes Attributes into a dictionary of its own, so that the entries
// can be round-tripped without building a complete form XObject.
type carrier struct {
	attrs *Attributes
}

func (c *carrier) Embed(e *pdf.EmbedHelper) (pdf.Native, error) {
	dict := pdf.Dict{}
	if err := c.attrs.FillDict(e, dict); err != nil {
		return nil, err
	}
	ref := e.Alloc()
	if err := e.Out().Put(ref, dict); err != nil {
		return nil, err
	}
	return ref, nil
}

func TestRoundTrip(t *testing.T) {
	for _, v := range []pdf.Version{pdf.V1_3, pdf.V1_7, pdf.V2_0} {
		for _, tc := range testCases {
			t.Run(tc.name+"-"+v.String(), func(t *testing.T) {
				roundTripTest(t, v, tc.attrs)
			})
		}
	}
}

func roundTripTest(t *testing.T, version pdf.Version, original *Attributes) {
	t.Helper()

	buf, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(buf)

	embedded, err := rm.Embed(&carrier{attrs: original})
	if pdf.IsWrongVersion(err) {
		t.Skip()
	} else if err != nil {
		t.Fatal(err)
	}

	if err := rm.Close(); err != nil {
		t.Fatal(err)
	}
	if err := buf.Close(); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(buf)
	c := pdf.CursorAt(x, nil)
	dict, err := c.Dict(embedded)
	if err != nil {
		t.Fatal(err)
	}
	readAttrs, err := ExtractAttributes(c, dict)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(original, readAttrs); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func TestExtractAbsent(t *testing.T) {
	x := pdf.NewExtractor(nil)
	c := pdf.CursorAt(x, nil)

	attrs, err := ExtractAttributes(c, pdf.Dict{"BBox": pdf.Array{}})
	if err != nil {
		t.Fatal(err)
	}
	if attrs != nil {
		t.Errorf("expected nil attributes, got %+v", attrs)
	}
}

// TestExtractRepairsPCM checks that a dictionary which uses the trap network
// entries but gives no usable process colour model is repaired, so that the
// result can be written back.
func TestExtractRepairsPCM(t *testing.T) {
	for _, tc := range []struct {
		name string
		dict pdf.Dict
	}{
		{
			name: "missing PCM",
			dict: pdf.Dict{"TrapStyles": pdf.TextString("some traps")},
		},
		{
			name: "unknown PCM",
			dict: pdf.Dict{"PCM": pdf.Name("DeviceWibble")},
		},
		{
			name: "PCM of wrong type",
			dict: pdf.Dict{"PCM": pdf.Integer(3)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x := pdf.NewExtractor(nil)
			attrs, err := ExtractAttributes(pdf.CursorAt(x, nil), tc.dict)
			if err != nil {
				t.Fatal(err)
			}
			if attrs == nil {
				t.Fatal("expected attributes")
			}
			if attrs.PCM != DefaultPCM {
				t.Errorf("expected PCM %q, got %q", DefaultPCM, attrs.PCM)
			}
			roundTripTest(t, pdf.V2_0, attrs)
		})
	}
}

// TestExtractSkipsInvalidColorantNames checks that entries which are not names
// are dropped from SeparationColorNames.
func TestExtractSkipsInvalidColorantNames(t *testing.T) {
	dict := pdf.Dict{
		"PCM": pdf.Name("DeviceCMYK"),
		"SeparationColorNames": pdf.Array{
			pdf.Name("Spot1"),
			pdf.Integer(7),
			pdf.Name("Spot2"),
		},
	}

	x := pdf.NewExtractor(nil)
	attrs, err := ExtractAttributes(pdf.CursorAt(x, nil), dict)
	if err != nil {
		t.Fatal(err)
	}

	want := []pdf.Name{"Spot1", "Spot2"}
	if diff := cmp.Diff(want, attrs.SeparationColorNames); diff != "" {
		t.Errorf("unexpected colorant names (-want +got):\n%s", diff)
	}
}

func TestFillDictRejectsInvalidPCM(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	_, err := rm.Embed(&carrier{attrs: &Attributes{PCM: "DeviceWibble"}})
	if err == nil {
		t.Error("expected an error for an invalid process colour model")
	}
}

// TestFillDictRejectsNilTrapRegion checks that a nil trap region is refused.
// Such an entry has no PDF representation, and silently skipping it would
// drop data the caller asked to write.
func TestFillDictRejectsNilTrapRegion(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V2_0, nil)
	rm := pdf.NewResourceManager(buf)

	attrs := &Attributes{
		PCM:         "DeviceCMYK",
		TrapRegions: []*opaque.Object{nil},
	}
	if _, err := rm.Embed(&carrier{attrs: attrs}); err == nil {
		t.Error("expected an error for a nil trap region")
	}
}

func TestFillDictVersion(t *testing.T) {
	buf, _ := memfile.NewPDFWriter(pdf.V1_2, nil)
	rm := pdf.NewResourceManager(buf)

	_, err := rm.Embed(&carrier{attrs: &Attributes{PCM: "DeviceCMYK"}})
	if !pdf.IsWrongVersion(err) {
		t.Errorf("expected a version error, got %v", err)
	}
}

func FuzzRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{
		HumanReadable: true,
	}
	for _, tc := range testCases {
		w, buf := memfile.NewPDFWriter(pdf.V2_0, opt)

		if err := memfile.AddBlankPage(w); err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)

		embedded, err := rm.Embed(&carrier{attrs: tc.attrs})
		if err != nil {
			continue
		}
		if err := rm.Close(); err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = embedded

		if err := w.Close(); err != nil {
			continue
		}

		f.Add(buf.Data)
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
		c := pdf.CursorAt(x, nil)
		dict, err := c.Dict(obj)
		if err != nil || dict == nil {
			t.Skip("missing dictionary")
		}

		attrs, err := ExtractAttributes(c, dict)
		if err != nil {
			t.Skip("malformed trap network entries")
		}
		if attrs == nil {
			t.Skip("no trap network entries")
		}

		roundTripTest(t, pdf.GetVersion(r), attrs)
	})
}
