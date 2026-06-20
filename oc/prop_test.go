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

package oc

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var (
	propGroup1 = &Group{Name: "Layer 1", Intent: []pdf.Name{"View"}}
	propGroup2 = &Group{Name: "Layer 2", Intent: []pdf.Name{"View"}}
	propGroup3 = &Group{Name: "Layer 3", Intent: []pdf.Name{"Design"}}

	propTestCases = []struct {
		name    string
		version pdf.Version
		data    *Properties
	}{
		{
			name:    "minimal",
			version: pdf.V1_5,
			data: &Properties{
				OCGs: []*Group{propGroup1},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
				},
			},
		},
		{
			name:    "multiple_groups",
			version: pdf.V1_5,
			data: &Properties{
				OCGs: []*Group{propGroup1, propGroup2, propGroup3},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
					OFF:       []*Group{propGroup2, propGroup3},
				},
			},
		},
		{
			name:    "with_off_list",
			version: pdf.V1_5,
			data: &Properties{
				OCGs: []*Group{propGroup1, propGroup2},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
					OFF:       []*Group{propGroup2},
				},
			},
		},
		{
			name:    "with_order",
			version: pdf.V1_5,
			data: &Properties{
				OCGs: []*Group{propGroup1, propGroup2, propGroup3},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
					Order: []OrderItem{
						propGroup1,
						&OrderGroup{
							Label:    "Background",
							Children: []OrderItem{propGroup2, propGroup3},
						},
					},
				},
			},
		},
		{
			name:    "with_configs",
			version: pdf.V1_5,
			data: &Properties{
				OCGs: []*Group{propGroup1, propGroup2},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
				},
				Configs: []*Configuration{
					{
						Name:      "Print",
						BaseState: BaseStateOFF,
						Intent:    []pdf.Name{"View"},
						ON:        []*Group{propGroup1},
					},
					{
						Name:      "Export",
						BaseState: BaseStateON,
						Intent:    []pdf.Name{"View"},
						OFF:       []*Group{propGroup2},
					},
				},
			},
		},
		{
			name:    "with_rbgroups",
			version: pdf.V1_5,
			data: &Properties{
				OCGs: []*Group{propGroup1, propGroup2, propGroup3},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
					RBGroups:  [][]*Group{{propGroup1, propGroup2}},
				},
			},
		},
		{
			name:    "minimal_v17",
			version: pdf.V1_7,
			data: &Properties{
				OCGs: []*Group{propGroup1},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
				},
			},
		},
		{
			name:    "minimal_v20",
			version: pdf.V2_0,
			data: &Properties{
				OCGs: []*Group{propGroup1},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
				},
			},
		},
		{
			name:    "complex_v17",
			version: pdf.V1_7,
			data: &Properties{
				OCGs: []*Group{propGroup1, propGroup2, propGroup3},
				D: &Configuration{
					BaseState: BaseStateON,
					Intent:    []pdf.Name{"View"},
					OFF:       []*Group{propGroup2, propGroup3},
					Order: []OrderItem{
						propGroup1,
						&OrderGroup{
							Label:    "Background",
							Children: []OrderItem{propGroup2, propGroup3},
						},
					},
					RBGroups: [][]*Group{{propGroup1, propGroup2}},
					Locked:   []*Group{propGroup3},
				},
				Configs: []*Configuration{
					{
						Name:      "Print",
						BaseState: BaseStateOFF,
						Intent:    []pdf.Name{"View"},
						ON:        []*Group{propGroup1},
						Order: []OrderItem{
							propGroup1,
							&OrderGroup{
								Label:    "Background",
								Children: []OrderItem{propGroup2, propGroup3},
							},
						},
						RBGroups: [][]*Group{{propGroup1, propGroup2}},
					},
				},
			},
		},
	}
)

func TestPropertiesRoundTrip(t *testing.T) {
	for _, tc := range propTestCases {
		t.Run(tc.name, func(t *testing.T) {
			testPropertiesRoundTrip(t, tc.version, tc.data)
		})
	}
}

func testPropertiesRoundTrip(t *testing.T, version pdf.Version, data *Properties) {
	t.Helper()

	w, _ := memfile.NewPDFWriter(version, nil)
	rm := pdf.NewResourceManager(w)

	obj, err := rm.Embed(data)
	if err != nil {
		if pdf.IsWrongVersion(err) {
			t.Skip("version not supported")
		}
		t.Fatalf("embed: %v", err)
	}

	err = rm.Close()
	if err != nil {
		t.Fatalf("rm.Close: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	extractor := pdf.NewExtractor(w)
	extracted, err := pdf.Decode(pdf.CursorAt(extractor, nil), obj, ExtractProperties)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// normalize for comparison
	normalizeProperties(data)
	normalizeProperties(extracted)

	opts := []cmp.Option{
		cmp.AllowUnexported(Properties{}, Configuration{}, UsageApplication{}),
		cmp.Comparer(func(a, b *Group) bool {
			if a == nil && b == nil {
				return true
			}
			if a == nil || b == nil {
				return false
			}
			return a.Name == b.Name
		}),
	}
	if diff := cmp.Diff(data, extracted, opts...); diff != "" {
		t.Errorf("round trip failed (-want +got):\n%s", diff)
	}
}

func normalizeProperties(p *Properties) {
	if len(p.OCGs) == 0 {
		p.OCGs = nil
	}
	if p.D != nil {
		normalizeConfiguration(p.D)
	}
	for _, c := range p.Configs {
		normalizeConfiguration(c)
	}
	if len(p.Configs) == 0 {
		p.Configs = nil
	}
}

func TestPropertiesValidation(t *testing.T) {
	// missing OCGs
	w, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)
	p := &Properties{D: &Configuration{BaseState: BaseStateON}}
	_, err := rm.Embed(p)
	if err == nil {
		t.Error("expected error for missing OCGs, but got none")
	}

	// missing D
	w2, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm2 := pdf.NewResourceManager(w2)
	p2 := &Properties{OCGs: []*Group{propGroup1}}
	_, err = rm2.Embed(p2)
	if err == nil {
		t.Error("expected error for missing D, but got none")
	}

	// version check
	w14, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm14 := pdf.NewResourceManager(w14)
	p3 := &Properties{
		OCGs: []*Group{propGroup1},
		D:    &Configuration{BaseState: BaseStateON},
	}
	_, err = rm14.Embed(p3)
	if err == nil {
		t.Error("expected version error for PDF 1.4, but got none")
	}
}

func FuzzPropertiesRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range propTestCases {
		w, buf := memfile.NewPDFWriter(tc.version, opt)

		err := memfile.AddBlankPage(w)
		if err != nil {
			continue
		}

		rm := pdf.NewResourceManager(w)
		obj, err := rm.Embed(tc.data)
		if err != nil {
			continue
		}

		err = rm.Close()
		if err != nil {
			continue
		}

		w.GetMeta().Trailer["Quir:E"] = obj
		err = w.Close()
		if err != nil {
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
			t.Skip("missing test object")
		}

		x := pdf.NewExtractor(r)
		data, err := pdf.Decode(pdf.CursorAt(x, nil), obj, ExtractProperties)
		if err != nil {
			t.Skip("malformed object")
		}

		testPropertiesRoundTrip(t, pdf.GetVersion(r), data)
	})
}
