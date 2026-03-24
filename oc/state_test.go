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

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
)

func TestDefaultState(t *testing.T) {
	g1 := &Group{Name: "Group 1"}
	g2 := &Group{Name: "Group 2"}
	g3 := &Group{Name: "Group 3"}
	allGroups := []*Group{g1, g2, g3}

	t.Run("base_state_on", func(t *testing.T) {
		c := &Configuration{BaseState: BaseStateON}
		state := c.DefaultState(allGroups, EventView, nil)
		for _, g := range allGroups {
			if !state.IsOn(g) {
				t.Errorf("expected %s to be ON", g.Name)
			}
		}
	})

	t.Run("base_state_off", func(t *testing.T) {
		c := &Configuration{BaseState: BaseStateOFF}
		state := c.DefaultState(allGroups, EventView, nil)
		for _, g := range allGroups {
			if state.IsOn(g) {
				t.Errorf("expected %s to be OFF", g.Name)
			}
		}
	})

	t.Run("default_base_state", func(t *testing.T) {
		c := &Configuration{}
		state := c.DefaultState(allGroups, EventView, nil)
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
		state := c.DefaultState(allGroups, EventView, nil)
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
		state := c.DefaultState(allGroups, EventView, nil)
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
				View: &UsageView{ViewState: StateOFF},
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
		state := c.DefaultState([]*Group{g}, EventView, nil)
		if state.IsOn(g) {
			t.Error("expected group to be OFF due to ViewState=false")
		}
	})

	t.Run("with_as_print_state", func(t *testing.T) {
		g := &Group{
			Name: "Printable",
			Usage: &Usage{
				Print: &UsagePrint{PrintState: StateON},
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
		state := c.DefaultState([]*Group{g}, EventPrint, nil)
		if !state.IsOn(g) {
			t.Error("expected group to be ON due to PrintState=true")
		}
	})

	t.Run("as_wrong_event_ignored", func(t *testing.T) {
		g := &Group{
			Name: "Printable",
			Usage: &Usage{
				Print: &UsagePrint{PrintState: StateON},
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
		state := c.DefaultState([]*Group{g}, EventView, nil)
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
		state := c.DefaultState([]*Group{g}, EventView, nil)
		if !state.IsOn(g) {
			t.Error("expected group to remain ON (no usage)")
		}
	})

	t.Run("as_multiple_categories_all_on", func(t *testing.T) {
		g := &Group{
			Name: "Both",
			Usage: &Usage{
				View:   &UsageView{ViewState: StateON},
				Export: &UsageExport{ExportState: StateON},
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
		state := c.DefaultState([]*Group{g}, EventView, nil)
		if !state.IsOn(g) {
			t.Error("expected group to be ON (both categories ON)")
		}
	})

	t.Run("as_multiple_categories_one_off", func(t *testing.T) {
		g := &Group{
			Name: "Mixed",
			Usage: &Usage{
				View:   &UsageView{ViewState: StateON},
				Export: &UsageExport{ExportState: StateOFF},
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
		state := c.DefaultState([]*Group{g}, EventView, nil)
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
		state := c.DefaultState(allGroups, EventView, nil)
		if !state.Participates(viewGroup) {
			t.Error("expected View group to participate in View config")
		}
		if state.Participates(designGroup) {
			t.Error("expected Design group to not participate in View config")
		}
	})

	t.Run("design_group_excluded_from_view_config", func(t *testing.T) {
		c := &Configuration{Intent: []pdf.Name{"View"}}
		state := c.DefaultState(allGroups, EventView, nil)
		// non-participating groups are always visible
		if !state.IsOn(designGroup) {
			t.Error("expected non-participating Design group to be visible")
		}
	})

	t.Run("all_config_includes_all_groups", func(t *testing.T) {
		c := &Configuration{Intent: []pdf.Name{"All"}}
		state := c.DefaultState(allGroups, EventView, nil)
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
		state := c.DefaultState(allGroups, EventView, nil)
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
		state := c.DefaultState([]*Group{viewGroup, designGroup, mixedGroup}, EventView, nil)
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
		state := c.DefaultState(allGroups, EventView, nil)
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
		state := c.DefaultState(allGroups, EventView, nil)
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
			View:   &UsageView{ViewState: StateON},
			Export: &UsageExport{ExportState: StateOFF},
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
		state := c.DefaultState([]*Group{g}, EventView, nil)
		// View says ON, Export says OFF → AND → OFF
		if state.IsOn(g) {
			t.Error("expected group to be OFF (AND of ON and OFF)")
		}
	})

	t.Run("and_semantics_all_on", func(t *testing.T) {
		gBoth := &Group{
			Name: "BothOn",
			Usage: &Usage{
				View:   &UsageView{ViewState: StateON},
				Export: &UsageExport{ExportState: StateON},
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
		state := c.DefaultState([]*Group{gBoth}, EventView, nil)
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
	state := c.DefaultState(allGroups, EventView, nil)

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
		state := c.DefaultState(allGroups, EventView, nil)
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

func TestManualOverride(t *testing.T) {
	g := &Group{Name: "Manual"}
	state := &GroupStates{state: map[*Group]bool{g: true}}

	if state.IsManual(g) {
		t.Error("expected group to not be manual initially")
	}

	state.SetManualState(g, false)
	if !state.IsManual(g) {
		t.Error("expected group to be manual after SetManualState")
	}
	if state.IsOn(g) {
		t.Error("expected group to be OFF after SetManualState(false)")
	}

	// clone preserves manual flag
	c := state.Clone()
	if !c.IsManual(g) {
		t.Error("expected clone to preserve manual flag")
	}
}

func TestApplyViewUsageZoom(t *testing.T) {
	t.Run("on_when_in_range", func(t *testing.T) {
		g := &Group{
			Name:  "ZoomLayer",
			Usage: &Usage{Zoom: &UsageZoom{Min: 1.0, Max: 4.0}},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryZoom},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Zoom: 2.0})
		if !state.IsOn(g) {
			t.Error("expected group ON at zoom 2.0 (range [1.0, 4.0))")
		}
	})

	t.Run("off_below_min", func(t *testing.T) {
		g := &Group{
			Name:  "ZoomLayer",
			Usage: &Usage{Zoom: &UsageZoom{Min: 2.0, Max: 4.0}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryZoom},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Zoom: 1.5})
		if state.IsOn(g) {
			t.Error("expected group OFF at zoom 1.5 (range [2.0, 4.0))")
		}
	})

	t.Run("off_at_max_strict_less_than", func(t *testing.T) {
		g := &Group{
			Name:  "ZoomLayer",
			Usage: &Usage{Zoom: &UsageZoom{Min: 1.0, Max: 2.0}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryZoom},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Zoom: 2.0})
		if state.IsOn(g) {
			t.Error("expected group OFF at zoom exactly 2.0 (strict < max)")
		}
	})

	t.Run("skip_when_zoom_zero", func(t *testing.T) {
		g := &Group{
			Name:  "ZoomLayer",
			Usage: &Usage{Zoom: &UsageZoom{Min: 1.0, Max: 4.0}},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryZoom},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Zoom: 0})
		if state.IsOn(g) {
			t.Error("expected group to remain OFF when zoom=0 (skip)")
		}
	})
}

func TestApplyViewUsageLanguage(t *testing.T) {
	t.Run("exact_match", func(t *testing.T) {
		en := &Group{
			Name:  "English",
			Usage: &Usage{Language: &UsageLanguage{Lang: language.English}},
		}
		de := &Group{
			Name:  "German",
			Usage: &Usage{Language: &UsageLanguage{Lang: language.German}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{en, de},
				Category: []Category{CategoryLanguage},
			}},
		}
		state := c.DefaultState([]*Group{en, de}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Lang: language.English})
		if !state.IsOn(en) {
			t.Error("expected English group ON with English locale")
		}
		if state.IsOn(de) {
			t.Error("expected German group OFF with English locale")
		}
	})

	t.Run("partial_match_preferred", func(t *testing.T) {
		enGB := &Group{
			Name: "en-GB",
			Usage: &Usage{Language: &UsageLanguage{
				Lang:      language.MustParse("en-GB"),
				Preferred: true,
			}},
		}
		enUS := &Group{
			Name: "en-US",
			Usage: &Usage{Language: &UsageLanguage{
				Lang: language.MustParse("en-US"),
			}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{enGB, enUS},
				Category: []Category{CategoryLanguage},
			}},
		}
		// system locale is en-AU — no exact match, partial match on both
		state := c.DefaultState([]*Group{enGB, enUS}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Lang: language.MustParse("en-AU")})
		if !state.IsOn(enGB) {
			t.Error("expected en-GB ON (partial match + Preferred)")
		}
		if state.IsOn(enUS) {
			t.Error("expected en-US OFF (partial match, not Preferred)")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		en := &Group{
			Name: "English",
			Usage: &Usage{Language: &UsageLanguage{
				Lang:      language.English,
				Preferred: true,
			}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{en},
				Category: []Category{CategoryLanguage},
			}},
		}
		state := c.DefaultState([]*Group{en}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Lang: language.Japanese})
		if state.IsOn(en) {
			t.Error("expected English group OFF with Japanese locale")
		}
	})

	t.Run("skip_when_und", func(t *testing.T) {
		en := &Group{
			Name:  "English",
			Usage: &Usage{Language: &UsageLanguage{Lang: language.English}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{en},
				Category: []Category{CategoryLanguage},
			}},
		}
		state := c.DefaultState([]*Group{en}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{Lang: language.Und})
		if !state.IsOn(en) {
			t.Error("expected group to remain ON when lang=Und (skip)")
		}
	})
}

func TestApplyViewUsageUser(t *testing.T) {
	t.Run("name_match", func(t *testing.T) {
		g := &Group{
			Name: "ForAlice",
			Usage: &Usage{User: &UsageUser{
				Type: UserTypeIndividual,
				Name: []string{"Alice", "Bob"},
			}},
		}
		c := &Configuration{
			BaseState: BaseStateOFF,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryUser},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{UserName: "Alice"})
		if !state.IsOn(g) {
			t.Error("expected group ON for user Alice")
		}
	})

	t.Run("name_mismatch", func(t *testing.T) {
		g := &Group{
			Name: "ForAlice",
			Usage: &Usage{User: &UsageUser{
				Type: UserTypeIndividual,
				Name: []string{"Alice"},
			}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryUser},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{UserName: "Charlie"})
		if state.IsOn(g) {
			t.Error("expected group OFF for user Charlie")
		}
	})

	t.Run("type_filter", func(t *testing.T) {
		g := &Group{
			Name: "OrgLayer",
			Usage: &Usage{User: &UsageUser{
				Type: UserTypeOrganisation,
				Name: []string{"Acme"},
			}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryUser},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		// user type is Individual, doesn't match Organisation
		c.ApplyViewUsage(state, &ViewerContext{
			UserName: "Acme",
			UserType: UserTypeIndividual,
		})
		if state.IsOn(g) {
			t.Error("expected group OFF when user type doesn't match")
		}
	})

	t.Run("skip_when_empty", func(t *testing.T) {
		g := &Group{
			Name: "ForAlice",
			Usage: &Usage{User: &UsageUser{
				Type: UserTypeIndividual,
				Name: []string{"Alice"},
			}},
		}
		c := &Configuration{
			BaseState: BaseStateON,
			AS: []*UsageApplication{{
				Event:    EventView,
				OCGs:     []*Group{g},
				Category: []Category{CategoryUser},
			}},
		}
		state := c.DefaultState([]*Group{g}, EventView, nil)
		c.ApplyViewUsage(state, &ViewerContext{UserName: ""})
		if !state.IsOn(g) {
			t.Error("expected group to remain ON when userName empty (skip)")
		}
	})
}

func TestApplyViewUsageANDSemantics(t *testing.T) {
	// zoom ON + language OFF = OFF
	g := &Group{
		Name: "Combo",
		Usage: &Usage{
			Zoom:     &UsageZoom{Min: 1.0, Max: 4.0},
			Language: &UsageLanguage{Lang: language.German},
		},
	}
	c := &Configuration{
		BaseState: BaseStateON,
		AS: []*UsageApplication{{
			Event:    EventView,
			OCGs:     []*Group{g},
			Category: []Category{CategoryZoom, CategoryLanguage},
		}},
	}
	state := c.DefaultState([]*Group{g}, EventView, nil)
	c.ApplyViewUsage(state, &ViewerContext{
		Zoom: 2.0,              // in range → ON
		Lang: language.English, // no match → OFF
	})
	if state.IsOn(g) {
		t.Error("expected group OFF (zoom ON AND language OFF)")
	}
}

func TestApplyViewUsageManualOverride(t *testing.T) {
	g := &Group{
		Name:  "ManualLayer",
		Usage: &Usage{Zoom: &UsageZoom{Min: 1.0, Max: 4.0}},
	}
	c := &Configuration{
		BaseState: BaseStateON,
		AS: []*UsageApplication{{
			Event:    EventView,
			OCGs:     []*Group{g},
			Category: []Category{CategoryZoom},
		}},
	}
	state := c.DefaultState([]*Group{g}, EventView, nil)
	state.SetManualState(g, true) // user manually set to ON

	// zoom says OFF, but manual override should prevent change
	c.ApplyViewUsage(state, &ViewerContext{Zoom: 0.5})
	if !state.IsOn(g) {
		t.Error("expected manually overridden group to remain ON")
	}
}

func TestApplyViewUsageNilContext(t *testing.T) {
	g := &Group{
		Name:  "Layer",
		Usage: &Usage{Zoom: &UsageZoom{Min: 1.0, Max: 4.0}},
	}
	c := &Configuration{
		BaseState: BaseStateOFF,
		AS: []*UsageApplication{{
			Event:    EventView,
			OCGs:     []*Group{g},
			Category: []Category{CategoryZoom},
		}},
	}
	state := c.DefaultState([]*Group{g}, EventView, nil)
	c.ApplyViewUsage(state, nil)
	if state.IsOn(g) {
		t.Error("expected group to remain OFF with nil context")
	}
}

func TestApplyViewUsageNoAS(t *testing.T) {
	g := &Group{
		Name:  "Layer",
		Usage: &Usage{Zoom: &UsageZoom{Min: 1.0, Max: 4.0}},
	}
	c := &Configuration{BaseState: BaseStateON}
	state := c.DefaultState([]*Group{g}, EventView, nil)
	c.ApplyViewUsage(state, &ViewerContext{Zoom: 0.5})
	if !state.IsOn(g) {
		t.Error("expected group to remain ON with no AS dicts")
	}
}
