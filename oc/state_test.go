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
	"testing"

	"seehuhn.de/go/pdf"
)

func TestDefaultState(t *testing.T) {
	g1 := &Group{Name: "Group 1"}
	g2 := &Group{Name: "Group 2"}
	g3 := &Group{Name: "Group 3"}
	allGroups := []*Group{g1, g2, g3}

	t.Run("base_state_on", func(t *testing.T) {
		c := &Configuration{BaseState: BaseStateON}
		state := c.DefaultState(allGroups, EventView)
		for _, g := range allGroups {
			if !state.IsOn(g) {
				t.Errorf("expected %s to be ON", g.Name)
			}
		}
	})

	t.Run("base_state_off", func(t *testing.T) {
		c := &Configuration{BaseState: BaseStateOFF}
		state := c.DefaultState(allGroups, EventView)
		for _, g := range allGroups {
			if state.IsOn(g) {
				t.Errorf("expected %s to be OFF", g.Name)
			}
		}
	})

	t.Run("default_base_state", func(t *testing.T) {
		c := &Configuration{}
		state := c.DefaultState(allGroups, EventView)
		for _, g := range allGroups {
			if !state.IsOn(g) {
				t.Errorf("expected %s to be ON (default)", g.Name)
			}
		}
	})

	t.Run("off_with_on_override", func(t *testing.T) {
		c := &Configuration{
			BaseState: BaseStateOFF,
			ON:        []*Group{g1, g3},
		}
		state := c.DefaultState(allGroups, EventView)
		if !state.IsOn(g1) {
			t.Error("expected g1 to be ON")
		}
		if state.IsOn(g2) {
			t.Error("expected g2 to be OFF")
		}
		if !state.IsOn(g3) {
			t.Error("expected g3 to be ON")
		}
	})

	t.Run("on_with_off_override", func(t *testing.T) {
		c := &Configuration{
			BaseState: BaseStateON,
			OFF:       []*Group{g2},
		}
		state := c.DefaultState(allGroups, EventView)
		if !state.IsOn(g1) {
			t.Error("expected g1 to be ON")
		}
		if state.IsOn(g2) {
			t.Error("expected g2 to be OFF")
		}
		if !state.IsOn(g3) {
			t.Error("expected g3 to be ON")
		}
	})

	t.Run("with_as_view_state", func(t *testing.T) {
		g := &Group{
			Name: "Viewable",
			Usage: &Usage{
				View: &UsageView{ViewState: false},
			},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{
				{
					Event:    EventView,
					OCGs:     []*Group{g},
					Category: []Category{CategoryView},
				},
			},
		}
		state := c.DefaultState([]*Group{g}, EventView)
		if state.IsOn(g) {
			t.Error("expected group to be OFF due to ViewState=false")
		}
	})

	t.Run("with_as_print_state", func(t *testing.T) {
		g := &Group{
			Name: "Printable",
			Usage: &Usage{
				Print: &UsagePrint{PrintState: true},
			},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{
				{
					Event:    EventPrint,
					OCGs:     []*Group{g},
					Category: []Category{CategoryPrint},
				},
			},
		}
		state := c.DefaultState([]*Group{g}, EventPrint)
		if !state.IsOn(g) {
			t.Error("expected group to be ON due to PrintState=true")
		}
	})

	t.Run("as_wrong_event_ignored", func(t *testing.T) {
		g := &Group{
			Name: "Printable",
			Usage: &Usage{
				Print: &UsagePrint{PrintState: true},
			},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{
				{
					Event:    EventPrint,
					OCGs:     []*Group{g},
					Category: []Category{CategoryPrint},
				},
			},
		}
		// ask for View event, should not match Print AS entry
		state := c.DefaultState([]*Group{g}, EventView)
		if state.IsOn(g) {
			t.Error("expected group to remain OFF (wrong event)")
		}
	})

	t.Run("as_no_usage", func(t *testing.T) {
		// group without usage should not be affected by AS
		g := &Group{Name: "NoUsage"}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{
				{
					Event:    EventView,
					OCGs:     []*Group{g},
					Category: []Category{CategoryView},
				},
			},
		}
		state := c.DefaultState([]*Group{g}, EventView)
		if !state.IsOn(g) {
			t.Error("expected group to remain ON (no usage)")
		}
	})

	t.Run("as_multiple_categories_all_on", func(t *testing.T) {
		g := &Group{
			Name: "Both",
			Usage: &Usage{
				View:   &UsageView{ViewState: true},
				Export: &UsageExport{ExportState: true},
			},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{
				{
					Event:    EventView,
					OCGs:     []*Group{g},
					Category: []Category{CategoryView, CategoryExport},
				},
			},
		}
		state := c.DefaultState([]*Group{g}, EventView)
		if !state.IsOn(g) {
			t.Error("expected group to be ON (both categories ON)")
		}
	})

	t.Run("as_multiple_categories_one_off", func(t *testing.T) {
		g := &Group{
			Name: "Mixed",
			Usage: &Usage{
				View:   &UsageView{ViewState: true},
				Export: &UsageExport{ExportState: false},
			},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{
				{
					Event:    EventView,
					OCGs:     []*Group{g},
					Category: []Category{CategoryView, CategoryExport},
				},
			},
		}
		state := c.DefaultState([]*Group{g}, EventView)
		if state.IsOn(g) {
			t.Error("expected group to be OFF (one category OFF)")
		}
	})
}

func TestIntentOverlaps(t *testing.T) {
	tests := []struct {
		name         string
		groupIntent  []pdf.Name
		configIntent []pdf.Name
		want         bool
	}{
		{
			name: "both default to View",
			want: true,
		},
		{
			name:        "group View, config default View",
			groupIntent: []pdf.Name{"View"},
			want:        true,
		},
		{
			name:         "group View, config Design",
			groupIntent:  []pdf.Name{"View"},
			configIntent: []pdf.Name{"Design"},
			want:         false,
		},
		{
			name:         "group Design, config View",
			groupIntent:  []pdf.Name{"Design"},
			configIntent: []pdf.Name{"View"},
			want:         false,
		},
		{
			name:         "config All matches everything",
			groupIntent:  []pdf.Name{"Design"},
			configIntent: []pdf.Name{"All"},
			want:         true,
		},
		{
			name:         "config All matches default View",
			configIntent: []pdf.Name{"All"},
			want:         true,
		},
		{
			name:         "multi-intent group, one matches",
			groupIntent:  []pdf.Name{"View", "Design"},
			configIntent: []pdf.Name{"Design"},
			want:         true,
		},
		{
			name:         "multi-intent config, one matches",
			groupIntent:  []pdf.Name{"Design"},
			configIntent: []pdf.Name{"View", "Design"},
			want:         true,
		},
		{
			name:         "no overlap",
			groupIntent:  []pdf.Name{"Design"},
			configIntent: []pdf.Name{"Print"},
			want:         false,
		},
		{
			name:         "explicit empty config intent",
			groupIntent:  []pdf.Name{"View"},
			configIntent: []pdf.Name{},
			want:         false,
		},
		{
			name:         "explicit empty config, default group",
			configIntent: []pdf.Name{},
			want:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := intentOverlaps(tc.groupIntent, tc.configIntent)
			if got != tc.want {
				t.Errorf("intentOverlaps(%v, %v) = %v, want %v",
					tc.groupIntent, tc.configIntent, got, tc.want)
			}
		})
	}
}

func TestIntentFiltering(t *testing.T) {
	viewGroup := &Group{Name: "ViewLayer", Intent: []pdf.Name{"View"}}
	designGroup := &Group{Name: "DesignLayer", Intent: []pdf.Name{"Design"}}
	allGroups := []*Group{viewGroup, designGroup}

	t.Run("view_config_includes_view_group", func(t *testing.T) {
		c := &Configuration{Intent: []pdf.Name{"View"}}
		state := c.DefaultState(allGroups, EventView)
		if !state.Participates(viewGroup) {
			t.Error("expected View group to participate in View config")
		}
		if state.Participates(designGroup) {
			t.Error("expected Design group to not participate in View config")
		}
	})

	t.Run("design_group_excluded_from_view_config", func(t *testing.T) {
		c := &Configuration{Intent: []pdf.Name{"View"}}
		state := c.DefaultState(allGroups, EventView)
		// non-participating groups are always visible
		if !state.IsOn(designGroup) {
			t.Error("expected non-participating Design group to be visible")
		}
	})

	t.Run("all_config_includes_all_groups", func(t *testing.T) {
		c := &Configuration{Intent: []pdf.Name{"All"}}
		state := c.DefaultState(allGroups, EventView)
		if !state.Participates(viewGroup) {
			t.Error("expected View group to participate in All config")
		}
		if !state.Participates(designGroup) {
			t.Error("expected Design group to participate in All config")
		}
	})

	t.Run("default_config_intent", func(t *testing.T) {
		// empty config intent defaults to ["View"]
		c := &Configuration{}
		state := c.DefaultState(allGroups, EventView)
		if !state.Participates(viewGroup) {
			t.Error("expected View group to participate in default config")
		}
		if state.Participates(designGroup) {
			t.Error("expected Design group to not participate in default config")
		}
	})

	t.Run("mixed_intent_groups", func(t *testing.T) {
		mixedGroup := &Group{Name: "Mixed", Intent: []pdf.Name{"View", "Design"}}
		c := &Configuration{Intent: []pdf.Name{"Design"}}
		state := c.DefaultState([]*Group{viewGroup, designGroup, mixedGroup}, EventView)
		if state.Participates(viewGroup) {
			t.Error("expected View-only group to not participate in Design config")
		}
		if !state.Participates(designGroup) {
			t.Error("expected Design group to participate in Design config")
		}
		if !state.Participates(mixedGroup) {
			t.Error("expected mixed-intent group to participate in Design config")
		}
	})

	t.Run("explicit_empty_intent_all_visible", func(t *testing.T) {
		// per spec 8.11.2.3: explicit empty Intent array means no groups
		// participate, so all content is visible
		c := &Configuration{Intent: []pdf.Name{}}
		state := c.DefaultState(allGroups, EventView)
		for _, g := range allGroups {
			if state.Participates(g) {
				t.Errorf("expected %s to not participate with empty Intent", g.Name)
			}
			if !state.IsOn(g) {
				t.Errorf("expected %s to be visible with empty Intent", g.Name)
			}
		}
	})

	t.Run("nil_intent_defaults_to_view", func(t *testing.T) {
		// nil Intent (absent from PDF) defaults to "View"
		c := &Configuration{Intent: nil}
		state := c.DefaultState(allGroups, EventView)
		if !state.Participates(viewGroup) {
			t.Error("expected View group to participate with nil Intent")
		}
		if state.Participates(designGroup) {
			t.Error("expected Design group to not participate with nil Intent")
		}
	})
}

func TestDefaultStateASAndSemantics(t *testing.T) {
	// per spec 8.11.4.4: "If a given optional content group appears in more
	// than one OCGs array, its state shall be ON only if all categories in
	// all the usage application dictionaries it appears in have a state of ON."
	g := &Group{
		Name: "Multi-AS",
		Usage: &Usage{
			View:   &UsageView{ViewState: true},
			Export: &UsageExport{ExportState: false},
		},
	}

	t.Run("and_semantics_mixed", func(t *testing.T) {
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{
				{
					Event:    EventView,
					OCGs:     []*Group{g},
					Category: []Category{CategoryView},
				},
				{
					Event:    EventView,
					OCGs:     []*Group{g},
					Category: []Category{CategoryExport},
				},
			},
		}
		state := c.DefaultState([]*Group{g}, EventView)
		// View says ON, Export says OFF → AND → OFF
		if state.IsOn(g) {
			t.Error("expected group to be OFF (AND of ON and OFF)")
		}
	})

	t.Run("and_semantics_all_on", func(t *testing.T) {
		gBoth := &Group{
			Name: "BothOn",
			Usage: &Usage{
				View:   &UsageView{ViewState: true},
				Export: &UsageExport{ExportState: true},
			},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{
				{
					Event:    EventView,
					OCGs:     []*Group{gBoth},
					Category: []Category{CategoryView},
				},
				{
					Event:    EventView,
					OCGs:     []*Group{gBoth},
					Category: []Category{CategoryExport},
				},
			},
		}
		state := c.DefaultState([]*Group{gBoth}, EventView)
		// both say ON → AND → ON
		if !state.IsOn(gBoth) {
			t.Error("expected group to be ON (AND of ON and ON)")
		}
	})
}

func TestVEIntentNotVisible(t *testing.T) {
	// A Design group under a View config should not participate.
	// VE = Not(Design group) should have no opinion, so content is visible.
	designGroup := &Group{Name: "DesignLayer", Intent: []pdf.Name{"Design"}}
	allGroups := []*Group{designGroup}

	c := &Configuration{Intent: []pdf.Name{"View"}}
	state := c.DefaultState(allGroups, EventView)

	m := &Membership{
		VE: &VisibilityExpressionNot{
			Arg: &VisibilityExpressionGroup{Group: designGroup},
		},
	}

	if !m.IsVisible(state) {
		t.Error("expected Not(non-participating Design group) to be visible")
	}
}

func TestDefaultStateBaseStateUnchanged(t *testing.T) {
	g1 := &Group{Name: "Group 1"}
	g2 := &Group{Name: "Group 2"}
	allGroups := []*Group{g1, g2}

	t.Run("unchanged_groups_participate", func(t *testing.T) {
		c := &Configuration{
			BaseState: BaseStateUnchanged,
			OFF:       []*Group{g1},
		}
		state := c.DefaultState(allGroups, EventView)
		// g1 explicitly OFF
		if state.IsOn(g1) {
			t.Error("expected g1 to be OFF")
		}
		// g2 should participate and be ON (Unchanged treats all as ON)
		if !state.Participates(g2) {
			t.Error("expected g2 to participate")
		}
		if !state.IsOn(g2) {
			t.Error("expected g2 to be ON")
		}
	})
}
