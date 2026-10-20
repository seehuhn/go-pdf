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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/internal/debug/memfile"
)

var (
	confGroup1 = &Group{Name: pdf.Text("Layer 1"), Intent: []pdf.Name{"View"}}
	confGroup2 = &Group{Name: pdf.Text("Layer 2"), Intent: []pdf.Name{"View"}}
	confGroup3 = &Group{Name: pdf.Text("Layer 3"), Intent: []pdf.Name{"Design"}}

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
				Name:      pdf.Text("Default"),
				Creator:   pdf.Text("TestApp"),
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
						Label:    pdf.Text("Anatomy"),
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
				Name:      pdf.Text("Full Config"),
				Creator:   pdf.Text("TestApp"),
				BaseState: BaseStateOFF,
				ON:        []*Group{confGroup1},
				OFF:       []*Group{confGroup2},
				Intent:    []pdf.Name{"View", "Design"},
				Order: []OrderItem{
					confGroup1,
					&OrderGroup{
						Label:    pdf.Text("Others"),
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
				Name:      pdf.Text("Full Config"),
				Creator:   pdf.Text("TestApp"),
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
			// absent Intent, Order and RBGroups
			name:    "absent_arrays",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
			},
		},
		{
			// empty Intent, Order and RBGroups arrays are explicit and must
			// stay distinct from absent ones
			name:    "explicitly_empty_arrays",
			version: pdf.V1_5,
			data: &Configuration{
				BaseState: BaseStateON,
				Intent:    []pdf.Name{},
				Order:     []OrderItem{},
				RBGroups:  [][]*Group{},
			},
		},
		{
			name:    "complex",
			version: pdf.V1_6,
			data: &Configuration{
				Name:      pdf.Text("Full Config"),
				Creator:   pdf.Text("TestApp"),
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
						Label:    pdf.Text("Others"),
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

	w, _ := memfile.NewPDFWriter(t, version, nil)
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
	extracted, err := pdf.CursorAt(extractor, nil).Decode(obj, ExtractConfiguration)
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
	// nil Intent is a shorthand for the value the reader always produces;
	// an empty Intent is meaningful and must not be collapsed.  The same
	// goes for Order and RBGroups, which are left untouched here.
	if c.Intent == nil {
		c.Intent = []pdf.Name{"View"}
	}
	if len(c.AS) == 0 {
		c.AS = nil
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

// TestConfigurationONOFFOverlap checks that a group listed in both the ON and
// OFF arrays, which Table 99 does not allow, is read back in OFF alone.  The
// writer rejects the overlap, so the repair is what keeps such a file
// writable again.
func TestConfigurationONOFFOverlap(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	shared := w.Alloc()
	err := w.Put(shared, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("shared")})
	if err != nil {
		t.Fatal(err)
	}
	onlyOn := w.Alloc()
	err = w.Put(onlyOn, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("only on")})
	if err != nil {
		t.Fatal(err)
	}

	ref := w.Alloc()
	err = w.Put(ref, pdf.Dict{
		"BaseState": pdf.Name("Unchanged"),
		"ON":        pdf.Array{shared, onlyOn},
		"OFF":       pdf.Array{shared},
	})
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	cfg, err := pdf.CursorAt(x, nil).Decode(ref, ExtractConfiguration)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(cfg.ON) != 1 || cfg.ON[0].Name.String() != "only on" {
		t.Errorf("ON = %v, want the group listed in ON alone", cfg.ON)
	}
	if len(cfg.OFF) != 1 || cfg.OFF[0].Name.String() != "shared" {
		t.Errorf("OFF = %v, want the shared group", cfg.OFF)
	}

	// the repaired value must be writable again
	w2, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)
	rm := pdf.NewResourceManager(w2)
	_, err = rm.Embed(cfg)
	if err != nil {
		t.Errorf("re-embed failed: %v", err)
	}
}

// a configuration whose ON array holds nothing but groups repeated in OFF
func TestConfigurationONOFFOverlapAll(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	shared := w.Alloc()
	err := w.Put(shared, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("shared")})
	if err != nil {
		t.Fatal(err)
	}

	ref := w.Alloc()
	err = w.Put(ref, pdf.Dict{
		"ON":  pdf.Array{shared},
		"OFF": pdf.Array{shared},
	})
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	cfg, err := pdf.CursorAt(x, nil).Decode(ref, ExtractConfiguration)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	// an emptied ON array reads as absent, matching an absent entry
	if cfg.ON != nil {
		t.Errorf("ON = %v, want nil", cfg.ON)
	}
	if len(cfg.OFF) != 1 {
		t.Errorf("OFF = %v, want the shared group", cfg.OFF)
	}
}

// TestConfigurationRBGroupsCap checks that no more than maxRBGroupsItems
// groups are read into RBGroups, so that a malicious file cannot make later
// scans over the collections arbitrarily expensive.
func TestConfigurationRBGroupsCap(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V2_0, nil)

	inner := make(pdf.Array, maxRBGroupsItems+5)
	for i := range inner {
		inner[i] = pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("g")}
	}

	ref := w.Alloc()
	err := w.Put(ref, pdf.Dict{
		"RBGroups": pdf.Array{
			inner,
			pdf.Array{pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("late")}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(w)
	cfg, err := pdf.CursorAt(x, nil).Decode(ref, ExtractConfiguration)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	total := 0
	for _, rb := range cfg.RBGroups {
		total += len(rb)
	}
	if total != maxRBGroupsItems {
		t.Errorf("got %d groups in RBGroups, want %d", total, maxRBGroupsItems)
	}
	if len(cfg.RBGroups) != 1 {
		t.Errorf("got %d collections, want the truncated first one alone", len(cfg.RBGroups))
	}
}

func TestConfigurationValidation(t *testing.T) {
	w14, _ := memfile.NewPDFWriter(t, pdf.V1_4, nil)
	rm14 := pdf.NewResourceManager(w14)

	// version check (PDF 1.4 should fail)
	c := &Configuration{BaseState: BaseStateON}
	_, err := rm14.Embed(c)
	if err == nil {
		t.Error("expected version error for PDF 1.4, but got none")
	}

	// invalid BaseState
	w, _ := memfile.NewPDFWriter(t, pdf.V1_5, nil)
	rm := pdf.NewResourceManager(w)
	c = &Configuration{BaseState: BaseState("Invalid")}
	_, err = rm.Embed(c)
	if err == nil {
		t.Error("expected error for invalid BaseState, but got none")
	}

	// empty intent name, alone and among valid ones
	for _, intent := range [][]pdf.Name{{""}, {"View", ""}} {
		c = &Configuration{Intent: intent}
		_, err = rm.Embed(c)
		if err == nil {
			t.Errorf("expected error for intent %q, but got none", intent)
		}
	}
}

func FuzzConfigurationRoundTrip(f *testing.F) {
	opt := &pdf.WriterOptions{HumanReadable: true}

	for _, tc := range confTestCases {
		w, buf := memfile.NewPDFWriter(f, tc.version, opt)

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
		data, err := pdf.CursorAt(x, nil).Decode(obj, ExtractConfiguration)
		if err != nil {
			t.Skip("malformed object")
		}

		// reading accepts versions the writer cannot reproduce
		version := pdf.GetVersion(r)
		if !version.IsSupported() {
			t.Skip("version cannot be written")
		}

		testConfigurationRoundTrip(t, version, data)
	})
}

// failingGetter wraps a [pdf.Getter] and fails to read the object bad with
// an I/O error, which is not a sign of a malformed file.
type failingGetter struct {
	pdf.Getter
	bad pdf.Reference
}

func (g failingGetter) Get(ref pdf.Reference, canObjStm bool) (pdf.Native, error) {
	if ref == g.bad {
		return nil, errors.New("I/O error")
	}
	return g.Getter.Get(ref, canObjStm)
}

// TestGroupArrayReadError checks that an I/O error while reading a group
// array element is reported, while a malformed element is skipped.
func TestGroupArrayReadError(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	good := w.Alloc()
	if err := w.Put(good, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("Layer")}); err != nil {
		t.Fatal(err)
	}
	broken := w.Alloc()
	if err := w.Put(broken, pdf.Integer(1)); err != nil {
		t.Fatal(err)
	}
	unreadable := w.Alloc()
	if err := w.Put(unreadable, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("Other")}); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(failingGetter{Getter: w, bad: unreadable})
	c := pdf.CursorAt(x, nil)

	groups, err := extractGroupArray(c, pdf.Array{broken, good})
	if err != nil {
		t.Fatalf("malformed element: unexpected error %v", err)
	}
	if len(groups) != 1 || groups[0].Name.String() != "Layer" {
		t.Errorf("malformed element: got %v, want only the good group", groups)
	}

	if _, err := extractGroupArray(c, pdf.Array{good, unreadable}); err == nil {
		t.Error("I/O error was not reported")
	}
}

// TestConfigurationReadErrors checks that I/O errors in array elements of a
// configuration are reported, while malformed elements are skipped.
func TestConfigurationReadErrors(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	put := func(obj pdf.Object) pdf.Reference {
		ref := w.Alloc()
		if err := w.Put(ref, obj); err != nil {
			t.Fatal(err)
		}
		return ref
	}
	good := put(pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("Layer")})
	unreadable := put(pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("Other")})
	malformed := pdf.Integer(1)

	x := pdf.NewExtractor(failingGetter{Getter: w, bad: unreadable})
	c := pdf.CursorAt(x, nil)

	cases := []struct {
		name  string
		key   pdf.Name
		bad   []pdf.Array // arrays which must cause a read error
		skip  pdf.Array   // malformed elements, to be skipped
		check func(*Configuration) bool
	}{
		{
			name: "RBGroups",
			key:  "RBGroups",
			bad: []pdf.Array{
				{pdf.Array{good, unreadable}},
				{unreadable},
			},
			skip: pdf.Array{malformed, pdf.Array{malformed, good}},
			check: func(cfg *Configuration) bool {
				return len(cfg.RBGroups) == 1 && len(cfg.RBGroups[0]) == 1
			},
		},
		{
			name: "AS",
			key:  "AS",
			bad:  []pdf.Array{{unreadable}},
			skip: pdf.Array{malformed},
			check: func(cfg *Configuration) bool {
				return len(cfg.AS) == 0
			},
		},
		{
			name: "Order",
			key:  "Order",
			bad: []pdf.Array{
				{unreadable},
				{pdf.Array{pdf.Text("label"), unreadable}},
				{pdf.Array{unreadable}},
			},
			skip: pdf.Array{malformed, pdf.Array{malformed, good}, good},
			check: func(cfg *Configuration) bool {
				return len(cfg.Order) == 2
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for i, arr := range tc.bad {
				dict := pdf.Dict{tc.key: arr}
				if _, err := ExtractConfiguration(c, dict, true); err == nil || pdf.IsMalformed(err) {
					t.Errorf("case %d: I/O error not reported: %v", i, err)
				}
			}

			dict := pdf.Dict{tc.key: tc.skip}
			cfg, err := ExtractConfiguration(c, dict, true)
			if err != nil {
				t.Fatalf("malformed elements: unexpected error %v", err)
			}
			if !tc.check(cfg) {
				t.Errorf("malformed elements: unexpected result %+v", cfg)
			}
		})
	}
}

// TestPropertiesConfigsReadError checks that an I/O error while reading an
// alternate configuration is reported, while a malformed entry is skipped.
func TestPropertiesConfigsReadError(t *testing.T) {
	w, _ := memfile.NewPDFWriter(t, pdf.V1_7, nil)
	group := w.Alloc()
	if err := w.Put(group, pdf.Dict{"Type": pdf.Name("OCG"), "Name": pdf.Text("Layer")}); err != nil {
		t.Fatal(err)
	}
	unreadable := w.Alloc()
	if err := w.Put(unreadable, pdf.Dict{"Name": pdf.Text("Alt")}); err != nil {
		t.Fatal(err)
	}

	x := pdf.NewExtractor(failingGetter{Getter: w, bad: unreadable})
	c := pdf.CursorAt(x, nil)

	props := func(configs pdf.Array) pdf.Dict {
		return pdf.Dict{
			"OCGs":    pdf.Array{group},
			"D":       pdf.Dict{},
			"Configs": configs,
		}
	}

	p, err := ExtractProperties(c, props(pdf.Array{pdf.Integer(1), pdf.Dict{"Name": pdf.Text("Alt")}}), true)
	if err != nil {
		t.Fatalf("malformed entry: unexpected error %v", err)
	}
	if len(p.Configs) != 1 {
		t.Errorf("malformed entry: got %d configs, want 1", len(p.Configs))
	}

	_, err = ExtractProperties(c, props(pdf.Array{unreadable}), true)
	if err == nil || pdf.IsMalformed(err) {
		t.Errorf("I/O error not reported: %v", err)
	}
}
