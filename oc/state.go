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
	"maps"
	"slices"

	"seehuhn.de/go/pdf"
)

// GroupStates tracks the visibility state of optional content groups.
// Groups present in the map participate in visibility decisions;
// groups absent from the map have no effect on visibility (always shown).
type GroupStates struct {
	state map[*Group]bool // present = participates; true = ON, false = OFF
}

// IsOn returns whether the group is visible. Groups not participating
// in this state (absent from the map) are treated as visible.
func (s *GroupStates) IsOn(g *Group) bool {
	if s == nil {
		return true
	}
	on, ok := s.state[g]
	return !ok || on
}

// Participates reports whether the group takes part in visibility decisions.
func (s *GroupStates) Participates(g *Group) bool {
	if s == nil {
		return false
	}
	_, ok := s.state[g]
	return ok
}

// SetState sets the visibility of a group, making it participate.
func (s *GroupStates) SetState(g *Group, on bool) {
	s.state[g] = on
}

// Clone returns a deep copy of the state.
func (s *GroupStates) Clone() *GroupStates {
	if s == nil {
		return nil
	}
	return &GroupStates{state: maps.Clone(s.state)}
}

// intentOverlaps reports whether a group's intent matches a configuration's
// intent.
//
// A nil config intent defaults to ["View"]. A non-nil empty config intent
// means no groups participate (per spec 8.11.2.3: "If the configuration's
// Intent is an empty array, no groups shall be used in determining visibility;
// therefore, all content shall be considered visible.").
//
// A nil or empty group intent defaults to ["View"].
// A config intent containing "All" matches everything.
func intentOverlaps(groupIntent, configIntent []pdf.Name) bool {
	if len(groupIntent) == 0 {
		groupIntent = []pdf.Name{"View"}
	}
	if configIntent == nil {
		// absent from PDF → default to View
		configIntent = []pdf.Name{"View"}
	} else if len(configIntent) == 0 {
		// explicit empty array → no groups participate
		return false
	}
	for _, ci := range configIntent {
		if ci == "All" {
			return true
		}
		if slices.Contains(groupIntent, ci) {
			return true
		}
	}
	return false
}

// DefaultState computes the initial group visibility from this configuration.
// It applies BaseState to allGroups, then ON/OFF overrides, then AS usage
// applications for the given event. Groups whose intent does not match the
// configuration's intent are removed so they have no effect on visibility.
// If allGroups is nil, BaseState has no effect and only groups explicitly
// listed in ON, OFF, or AS are included.
func (c *Configuration) DefaultState(allGroups []*Group, event Event) *GroupStates {
	state := make(map[*Group]bool)

	// step 1: apply BaseState to all groups
	bs := c.BaseState
	if bs == "" {
		bs = BaseStateON
	}

	switch bs {
	case BaseStateON:
		for _, g := range allGroups {
			state[g] = true
		}
	case BaseStateOFF:
		for _, g := range allGroups {
			state[g] = false
		}
	case BaseStateUnchanged:
		// per spec the default config must use ON, but alternate configs
		// may use Unchanged; treat absent groups as ON so they participate
		for _, g := range allGroups {
			state[g] = true
		}
	}

	// step 2: apply ON/OFF overrides
	for _, g := range c.ON {
		state[g] = true
	}
	for _, g := range c.OFF {
		state[g] = false
	}

	// step 3: apply AS usage application dictionaries for the given event
	//
	// Per spec (8.11.4.4): "If a given optional content group appears in
	// more than one OCGs array, its state shall be ON only if all
	// categories in all the usage application dictionaries it appears in
	// have a state of ON."
	//
	// Collect all AS recommendations per group, then apply with AND.
	asRecs := map[*Group]bool{} // true = all recs so far are ON
	for _, ua := range c.AS {
		if ua.Event != event {
			continue
		}
		for _, g := range ua.OCGs {
			if g.Usage == nil {
				continue
			}
			on, ok := evaluateUsage(g.Usage, ua.Category)
			if !ok {
				continue
			}
			prev, seen := asRecs[g]
			if !seen {
				asRecs[g] = on
			} else {
				asRecs[g] = prev && on
			}
		}
	}
	maps.Copy(state, asRecs)

	// step 4: remove groups whose intent does not match the configuration
	for g := range state {
		if !intentOverlaps(g.Intent, c.Intent) {
			delete(state, g)
		}
	}

	return &GroupStates{state: state}
}

// evaluateUsage evaluates the usage dictionary for the given categories.
// It returns the recommended state and true if any categories matched,
// or false, false if none matched.
// Per spec: the group is ON only if all consulted categories yield ON.
func evaluateUsage(u *Usage, categories []Category) (on bool, matched bool) {
	allOn := true

	for _, cat := range categories {
		var val bool
		var found bool
		switch cat {
		case CategoryView:
			if u.View != nil {
				val = u.View.ViewState
				found = true
			}
		case CategoryPrint:
			if u.Print != nil {
				val = u.Print.PrintState
				found = true
			}
		case CategoryExport:
			if u.Export != nil {
				val = u.Export.ExportState
				found = true
			}
		case CategoryZoom:
			// zoom requires runtime magnification level; not applicable here
		case CategoryLanguage:
			// language requires runtime locale; not applicable here
		case CategoryUser:
			// user requires runtime user info; not applicable here
		case CategoryCreatorInfo:
			// creator info does not yield a state recommendation
		case CategoryPageElement:
			// page element does not yield a state recommendation
		}
		if found {
			matched = true
			if !val {
				allOn = false
			}
		}
	}

	return allOn, matched
}
