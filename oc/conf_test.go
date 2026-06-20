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
	confGroup1 = &Group{Name: "Layer 1", Intent: []pdf.Name{"View"}}
	confGroup2 = &Group{Name: "Layer 2", Intent: []pdf.Name{"View"}}
	confGroup3 = &Group{Name: "Layer 3", Intent: []pdf.Name{"Design"}}

	confTestCases = []struct {
		name    string
		version pdf.Version
		data    *Configuration
	}{
		{
			name:    "minimal",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "with_name_and_creator",
			version: pdf.V1_5,
			data: &Configuration{
				Name:      "Default",
				Creator:   "TestApp",
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "base_state_off_with_on",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateOFF,
				ON:        []*Group{confGroup1},
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "base_state_on_with_off",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				OFF:       []*Group{confGroup2, confGroup3},
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "with_intent",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View", "Design"},
			},
		},
		{
			name:    "with_single_intent",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"Design"},
			},
		},
		{
			name:    "with_as",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				AS: []*UsageApplication{
					{
						SingleUse: true,
						Event:     EventView,
						OCGs:      []*Group{confGroup1},
						Category:  []Category{CategoryZoom},
					},
				},
			},
		},
		{
			name:    "with_order_flat",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				Order:     []OrderItem{confGroup1, confGroup2},
			},
		},
		{
			name:    "with_order_nested",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				Order: []OrderItem{
					&OrderGroup{
						Label:    "Anatomy",
						Children: []OrderItem{confGroup1, confGroup2},
					},
					confGroup3,
				},
			},
		},
		{
			name:    "with_order_nested_no_label",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				Order: []OrderItem{
					confGroup1,
					&OrderGroup{
						Children: []OrderItem{confGroup2, confGroup3},
					},
				},
			},
		},
		{
			name:    "list_mode_visible_pages",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				ListMode:  ListModeVisiblePages,
			},
		},
		{
			name:    "with_rbgroups",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				RBGroups:  [][]*Group{{confGroup1, confGroup2}},
			},
		},
		{
			name:    "with_locked",
			version: pdf.V1_6,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
				Locked:    []*Group{confGroup3},
			},
		},
		{
			name:    "direct",
			version: pdf.V1_5,
			data: &Configuration{
				SingleUse: true,
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "minimal_v17",
			version: pdf.V1_7,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "minimal_v20",
			version: pdf.V2_0,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{"View"},
			},
		},
		{
			name:    "complex_v17",
			version: pdf.V1_7,
			data: &Configuration{
				Name:      "Full Config",
				Creator:   "TestApp",
				BaseState: BaseStateOFF,
				ON:        []*Group{confGroup1},
				OFF:       []*Group{confGroup2},
				Intent:    []pdf.Name{"View", "Design"},
				Order: []OrderItem{
					confGroup1,
					&OrderGroup{
						Label:    "Others",
						Children: []OrderItem{confGroup2, confGroup3},
					},
				},
				ListMode: ListModeVisiblePages,
				RBGroups: [][]*Group{{confGroup1, confGroup2}},
				Locked:   []*Group{confGroup3},
			},
		},
		{
			name:    "complex_v20",
			version: pdf.V2_0,
			data: &Configuration{
				Name:      "Full Config",
				Creator:   "TestApp",
				BaseState: BaseStateUnchanged,
				ON:        []*Group{confGroup1, confGroup3},
				OFF:       []*Group{confGroup2},
				Intent:    []pdf.Name{"Design"},
				AS: []*UsageApplication{
					{
						SingleUse: true,
						Event:     EventView,
						OCGs:      []*Group{confGroup1},
						Category:  []Category{CategoryView},
					},
				},
				Order:    []OrderItem{confGroup1, confGroup2, confGroup3},
				RBGroups: [][]*Group{{confGroup2, confGroup3}},
				Locked:   []*Group{confGroup1},
			},
		},
		{
			name:    "complex",
			version: pdf.V1_6,
			data: &Configuration{
				Name:      "Full Config",
				Creator:   "TestApp",
				BaseState: BaseStateOFF,
				ON:        []*Group{confGroup1},
				OFF:       []*Group{confGroup2},
				Intent:    []pdf.Name{"View"},
				AS: []*UsageApplication{
					{
						SingleUse: true,
						Event:     EventPrint,
						OCGs:      []*Group{confGroup3},
						Category:  []Category{CategoryPrint},
					},
				},
				Order: []OrderItem{
					confGroup1,
					&OrderGroup{
						Label:    "Others",
						Children: []OrderItem{confGroup2, confGroup3},
					},
				},
				ListMode: ListModeAllPages,
				RBGroups: [][]*Group{{confGroup1, confGroup2}},
				Locked:   []*Group{confGroup3},
			},
		},
	}
)

func TestConfigurationRoundTrip(t *testing.T) {
	for _, tc := range confTestCases {
		t.Run(tc.name, func(t *testing.T) {
			testConfigurationRoundTrip(t, tc.version, tc.data)
		})
	}
}

func testConfigurationRoundTrip(t *testing.T, version pdf.Version, data *Configuration) {
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
	extracted, err := pdf.Decode(pdf.CursorAt(extractor, nil), obj, ExtractConfiguration)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// normalize both for comparison
	normalizeConfiguration(data)
	normalizeConfiguration(extracted)

	opts := []cmp.Option{
		cmp.AllowUnexported(Configuration{}, UsageApplication{}),
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

func normalizeConfiguration(c *Configuration) {
	if c.BaseState == "" {
		c.BaseState = BaseStateON
	}
	if c.ListMode == "" {
		c.ListMode = ListModeAllPages
	}
	if len(c.ON) == 0 {
		c.ON = nil
	}
	if len(c.OFF) == 0 {
		c.OFF = nil
	}
	if len(c.Intent) == 0 {
		c.Intent = nil
	}
	if len(c.AS) == 0 {
		c.AS = nil
	}
	if len(c.Order) == 0 {
		c.Order = nil
	}
	if len(c.RBGroups) == 0 {
		c.RBGroups = nil
	}
	if len(c.Locked) == 0 {
		c.Locked = nil
	}
	// normalize sub-objects
	for _, ua := range c.AS {
		if len(ua.OCGs) == 0 {
			ua.OCGs = nil
		}
	}
	normalizeOrderItems(c.Order)
}

func normalizeOrderItems(items []OrderItem) {
	for _, item := range items {
		if og, ok := item.(*OrderGroup); ok {
			normalizeOrderItems(og.Children)
			if len(og.Children) == 0 {
				og.Children = nil
			}
		}
	}
}

func TestConfigurationValidation(t *testing.T) {
	w14, _ := memfile.NewPDFWriter(pdf.V1_4, nil)
	rm14 := pdf.NewResourceManager(w14)

	// version check (PDF 1.4 should fail)
	c := &Configuration{BaseState: BaseStateON}
	_, err := rm14.Embed(c)
	if err == nil {
		t.Error("expected version error for PDF 1.4, but got none")
	}

	// invalid BaseState
	w, _ := memfile.NewPDFWriter(pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)
	c = &Configuration{BaseState: BaseState("Invalid")}
	_, err = rm.Embed(c)
	if err == nil {
		t.Error("expected error for invalid BaseState, but got none")
	}
}

func FuzzConfigurationRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range confTestCases {
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
		data, err := pdf.Decode(pdf.CursorAt(x, nil), obj, ExtractConfiguration)
		if err != nil {
			t.Skip("malformed object")
		}

		testConfigurationRoundTrip(t, pdf.GetVersion(r), data)
	})
}
