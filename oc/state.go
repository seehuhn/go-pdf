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

	"golang.org/x/text/language"
	"seehuhn.de/go/pdf"
)

// ViewerContext provides runtime information needed to evaluate
// Zoom, Language, and User usage categories.
type ViewerContext struct {
	// Zoom is the user-facing magnification factor.
	// Zero means skip zoom evaluation.
	Zoom float64

	// Lang is the system locale. language.Und means skip language evaluation.
	Lang language.Tag

	// UserName is the current user's name. Empty means skip user evaluation.
	UserName string

	// UserType filters the user match to a specific type.
	// Empty means match any type.
	UserType UserType
}

// GroupStates tracks the visibility state of optional content groups.
// Groups present in the map participate in visibility decisions;
// groups absent from the map have no effect on visibility (always shown).
//
// The zero value is an empty state in which no group participates, and can be
// used and modified directly.  A nil *GroupStates is equally valid and behaves
// the same way, except that it cannot be modified.  Use
// [Configuration.DefaultState] to obtain the states a configuration prescribes;
// only a state obtained that way knows the radio-button relationships
// [GroupStates.Switch] applies.
type GroupStates struct {
	state  map[*Group]bool // present = participates; true = ON, false = OFF
	manual map[*Group]bool // groups set manually by the user
	config *Configuration  // the configuration the states were derived from
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
	if s.state == nil {
		s.state = make(map[*Group]bool)
	}
	s.state[g] = on
}

// SetManualState sets the visibility of a group and marks it as manually
// overridden by the user. Groups with manual overrides are not affected
// by [Configuration.ApplyViewUsage].
func (s *GroupStates) SetManualState(g *Group, on bool) {
	s.SetState(g, on)
	if s.manual == nil {
		s.manual = make(map[*Group]bool)
	}
	s.manual[g] = true
}

// IsManual reports whether the group was set manually by the user.
func (s *GroupStates) IsManual(g *Group) bool {
	if s == nil {
		return false
	}
	return s.manual[g]
}

// Switch sets the visibility of a group as a manual change, made by the user
// or by a set-OCG-state action, and applies the radio-button relationships of
// the configuration the state came from: switching a group on switches off
// the other members of every radio-button collection it belongs to.
// Switching a group off leaves the other members alone.
//
// The group named is always switched, but a sibling is only switched off if
// the state already covers it: a group outside the state decides the
// visibility of nothing, and making it participate would hide content the
// configuration wants shown.
//
// Every group switched here counts as manually set, so that a later usage
// application does not undo it.  A state which did not come from
// [Configuration.DefaultState] carries no configuration, and hence no
// radio-button relationships to apply.
func (s *GroupStates) Switch(g *Group, on bool) {
	s.SetManualState(g, on)
	if !on || s.config == nil {
		return
	}
	for _, rb := range s.config.RBGroups {
		if !slices.Contains(rb, g) {
			continue
		}
		for _, other := range rb {
			if other == g {
				continue
			}
			if _, ok := s.state[other]; !ok {
				continue
			}
			s.SetManualState(other, false)
		}
	}
}

// Clone returns a deep copy of the state.
func (s *GroupStates) Clone() *GroupStates {
	if s == nil {
		return nil
	}
	c := &GroupStates{state: maps.Clone(s.state), config: s.config}
	if s.manual != nil {
		c.manual = maps.Clone(s.manual)
	}
	return c
}

// intentOverlaps reports whether a group's intent matches a configuration's
// intent. Nil config/group intents default to ["View"]. A config intent
// containing "All" matches everything.
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
// Finally each RBGroups array is reduced to at most one visible member, the
// first one in the array.
// If allGroups is nil, BaseState has no effect and only groups explicitly
// listed in ON, OFF, or AS are included.
//
// The prior parameter provides the group states from the previously active
// configuration. It is used when BaseState is Unchanged (alternate configs
// only). If prior is nil, Unchanged is treated as ON.
func (c *Configuration) DefaultState(allGroups []*Group, event Event, prior *GroupStates) *GroupStates {
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
		// preserve prior state; groups absent from prior default to ON
		for _, g := range allGroups {
			state[g] = prior.IsOn(g)
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
			on, ok := evaluateUsage(g.Usage, ua.Category, nil)
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

	// step 5: reduce each radio-button collection to at most one visible
	// member, which is all a state is allowed to have at any one time
	// (8.11.4.3).  The first visible member in array order is the one kept:
	// with several switched on there is nothing to prefer a later one.
	// Groups the state does not cover take no part, since they decide the
	// visibility of nothing, and a group listed twice is not its own sibling.
	for _, rb := range c.RBGroups {
		var keeper *Group
		for _, g := range rb {
			if on, ok := state[g]; !ok || !on {
				continue
			}
			if keeper == nil {
				keeper = g
			} else if g != keeper {
				state[g] = false
			}
		}
	}

	return &GroupStates{state: state, config: c}
}

// evaluateUsage evaluates the usage dictionary for the given categories.
// It returns the recommended state and true if any categories matched,
// or false, false if none matched.
// Per spec: the group is ON only if all consulted categories yield ON.
//
// The ctx parameter provides runtime context for Zoom and User categories.
// If ctx is nil, runtime categories are skipped. Language is always skipped
// here because it requires collective evaluation across groups.
func evaluateUsage(u *Usage, categories []Category, ctx *ViewerContext) (on bool, matched bool) {
	allOn := true

	for _, cat := range categories {
		var catOn bool
		var catMatched bool

		switch cat {
		case CategoryView:
			if u.View != nil && u.View.ViewState != StateUnset {
				catOn = u.View.ViewState.IsOn()
				catMatched = true
			}
		case CategoryPrint:
			if u.Print != nil && u.Print.PrintState != StateUnset {
				catOn = u.Print.PrintState.IsOn()
				catMatched = true
			}
		case CategoryExport:
			if u.Export != nil && u.Export.ExportState != StateUnset {
				catOn = u.Export.ExportState.IsOn()
				catMatched = true
			}
		case CategoryZoom:
			if ctx != nil && ctx.Zoom > 0 && u.Zoom != nil {
				catOn = u.Zoom.Min <= ctx.Zoom && ctx.Zoom < u.Zoom.Max
				catMatched = true
			}
		case CategoryUser:
			if ctx != nil && ctx.UserName != "" && u.User != nil {
				if ctx.UserType == "" || ctx.UserType == u.User.Type {
					catOn = slices.Contains(u.User.Name, ctx.UserName)
				}
				catMatched = true
			}
		case CategoryLanguage:
			// language requires collective evaluation; handled by evaluateLanguage
		case CategoryCreatorInfo:
			// creator info does not yield a state recommendation
		case CategoryPageElement:
			// page element does not yield a state recommendation
		}

		if catMatched {
			matched = true
			if !catOn {
				allOn = false
			}
		}
	}

	return allOn, matched
}

// evaluateLanguage performs collective language matching for a single
// usage application dictionary's OCGs list.
//
// Per spec (8.11.4.4): all groups with Language usage in the same AS dict
// are considered together. If any group's language exactly matches the
// system locale, exact-matching groups are ON and others are OFF. If no
// exact match exists, groups whose language partially matches (same base
// language) and whose Preferred flag is ON are turned ON; others are OFF.
//
// Returns a map from group to recommended state, containing only groups
// that have Language usage.
func evaluateLanguage(groups []*Group, sysLang language.Tag) map[*Group]bool {
	if sysLang == language.Und {
		return nil
	}

	// collect groups with Language usage
	type langGroup struct {
		group *Group
		tag   language.Tag
		pref  bool
	}
	var candidates []langGroup
	for _, g := range groups {
		if g.Usage == nil || g.Usage.Language == nil {
			continue
		}
		candidates = append(candidates, langGroup{
			group: g,
			tag:   g.Usage.Language.Lang,
			pref:  g.Usage.Language.Preferred,
		})
	}
	if len(candidates) == 0 {
		return nil
	}

	// pass 1: check for exact locale matches
	hasExact := false
	for _, c := range candidates {
		if c.tag == sysLang {
			hasExact = true
			break
		}
	}

	result := make(map[*Group]bool, len(candidates))
	if hasExact {
		// exact match: groups with exact match are ON, others OFF
		for _, c := range candidates {
			result[c.group] = (c.tag == sysLang)
		}
	} else {
		// partial match: same base language + Preferred=ON → ON
		sysMatcher := language.NewMatcher([]language.Tag{sysLang})
		for _, c := range candidates {
			_, _, conf := sysMatcher.Match(c.tag)
			partial := conf >= language.Low
			result[c.group] = partial && c.pref
		}
	}

	return result
}

// ApplyViewUsage re-evaluates all View-event AS dicts with runtime context
// and updates the state accordingly. Groups with manual overrides are
// not affected, and neither are groups whose intent does not match the
// configuration's: [Configuration.DefaultState] excludes those from the state,
// and a usage application must not bring them back.
//
// If ctx is nil, or if state is nil and thus cannot be modified, this is a
// no-op.
func (c *Configuration) ApplyViewUsage(state *GroupStates, ctx *ViewerContext) {
	if ctx == nil || state == nil || len(c.AS) == 0 {
		return
	}

	// groups the configuration considers at all
	considers := func(g *Group) bool {
		return intentOverlaps(g.Intent, c.Intent)
	}

	// collect recommendations per group, ANDing across AS dicts
	recs := map[*Group]bool{}
	for _, ua := range c.AS {
		if ua.Event != EventView {
			continue
		}

		// per-group evaluation for non-Language categories
		for _, g := range ua.OCGs {
			if g.Usage == nil || !considers(g) || state.IsManual(g) {
				continue
			}
			on, ok := evaluateUsage(g.Usage, ua.Category, ctx)
			if !ok {
				continue
			}
			prev, seen := recs[g]
			if !seen {
				recs[g] = on
			} else {
				recs[g] = prev && on
			}
		}

		// Collective language evaluation, over the considered groups only:
		// an excluded group must not influence the best-match choice either.
		// A manually overridden group, by contrast, still takes part in the
		// choice — it merely keeps its own state afterwards.
		if slices.Contains(ua.Category, CategoryLanguage) && ctx.Lang != language.Und {
			candidates := make([]*Group, 0, len(ua.OCGs))
			for _, g := range ua.OCGs {
				if considers(g) {
					candidates = append(candidates, g)
				}
			}
			langRecs := evaluateLanguage(candidates, ctx.Lang)
			for g, on := range langRecs {
				if state.IsManual(g) {
					continue
				}
				prev, seen := recs[g]
				if !seen {
					recs[g] = on
				} else {
					recs[g] = prev && on
				}
			}
		}
	}

	// apply results
	for g, on := range recs {
		state.SetState(g, on)
	}
}
