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

// PDF 2.0 sections: 8.11.4.5

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

// ViewState holds the persistent optional-content viewing state of a
// document: the group visibility a configuration prescribes, together with
// the manual changes the user or a set-OCG-state action has made.  Manual
// changes are pinned: a usage application never moves a pinned group.
//
// A ViewState is not what a render consults.  The visibility in force at any
// moment is derived from it with [ViewState.Effective], which layers the
// usage recommendations of the configuration between the prescribed states
// and the pins.
//
// The zero value is an empty state with no configuration; it can be used and
// modified directly.  A nil *ViewState is equally valid and behaves the same
// way, except that it cannot be modified.  Use [Configuration.DefaultState]
// to obtain the state a configuration prescribes; only a state obtained that
// way knows the configuration's radio-button relationships and usage
// applications.
type ViewState struct {
	config *Configuration  // the configuration the state was derived from
	base   map[*Group]bool // the configuration-prescribed visibility
	pins   map[*Group]bool // manual overrides, layered over base and usage
}

// IsOn returns the stored visibility of the group: its pinned state if the
// group was manually changed, the prescribed state otherwise.  Groups not
// participating in this state are treated as visible.  Usage recommendations
// are not part of the answer; derive them with [ViewState.Effective].
func (s *ViewState) IsOn(g *Group) bool {
	if s == nil {
		return true
	}
	if on, ok := s.pins[g]; ok {
		return on
	}
	on, ok := s.base[g]
	return !ok || on
}

// Participates reports whether the group takes part in visibility decisions.
func (s *ViewState) Participates(g *Group) bool {
	if s == nil {
		return false
	}
	if _, ok := s.pins[g]; ok {
		return true
	}
	_, ok := s.base[g]
	return ok
}

// IsManual reports whether the group's state was set manually.
func (s *ViewState) IsManual(g *Group) bool {
	if s == nil {
		return false
	}
	_, ok := s.pins[g]
	return ok
}

// SetManualState pins the visibility of a group as a manual change, without
// regard to radio-button relationships.  A pinned group keeps this state
// until the pin is replaced: later usage applications do not move it.
func (s *ViewState) SetManualState(g *Group, on bool) {
	if s.pins == nil {
		s.pins = make(map[*Group]bool)
	}
	s.pins[g] = on
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
// Every group switched here is pinned, the displaced siblings included, so
// that a later usage application does not undo any part of the change.  A
// state which did not come from [Configuration.DefaultState] carries no
// configuration, and hence no radio-button relationships to apply.
func (s *ViewState) Switch(g *Group, on bool) {
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
			if !s.Participates(other) {
				continue
			}
			s.SetManualState(other, false)
		}
	}
}

// Clone returns an independent copy of the state.
func (s *ViewState) Clone() *ViewState {
	if s == nil {
		return nil
	}
	c := &ViewState{config: s.config, base: maps.Clone(s.base)}
	if s.pins != nil {
		c.pins = maps.Clone(s.pins)
	}
	return c
}

// GroupStates is a snapshot of effective group visibility, as consulted when
// rendering or filtering content.  Groups present in the snapshot participate
// in visibility decisions; groups absent from it have no effect on visibility
// (always shown).
//
// Snapshots are usually derived from a [ViewState] with [ViewState.Effective]
// or [ViewState.EffectiveForPrint].  The zero value is an empty snapshot and
// can be populated directly with [GroupStates.SetState]; a nil *GroupStates
// is equally valid and behaves the same way, except that it cannot be
// modified.
type GroupStates struct {
	state map[*Group]bool // present = participates; true = ON, false = OFF
}

// IsOn returns whether the group is visible. Groups not participating
// in this snapshot (absent from the map) are treated as visible.
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

// SetState sets the visibility of a group, adding it to the snapshot if it
// was not participating before.
func (s *GroupStates) SetState(g *Group, on bool) {
	if s.state == nil {
		s.state = make(map[*Group]bool)
	}
	s.state[g] = on
}

// Clone returns an independent copy of the snapshot.
func (s *GroupStates) Clone() *GroupStates {
	if s == nil {
		return nil
	}
	return &GroupStates{state: maps.Clone(s.state)}
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

// DefaultState computes the group visibility this configuration prescribes:
// BaseState applied to allGroups, then the ON and OFF overrides.  Groups
// whose intent does not match the configuration's are removed so they have
// no effect on visibility, and each RBGroups collection is reduced to at
// most one visible member, the first one in the array.  Usage applications
// are not folded in here; they belong to the derivation in
// [ViewState.Effective], which re-evaluates them for every view.
// If allGroups is nil, BaseState has no effect and only groups explicitly
// listed in ON or OFF are included.
//
// The prior parameter provides the group states from the previously active
// configuration. It is used when BaseState is Unchanged (alternate configs
// only): the prior state is read with manual changes in force, but the pins
// themselves are not carried over. If prior is nil, Unchanged is treated
// as ON.
func (c *Configuration) DefaultState(allGroups []*Group, prior *ViewState) *ViewState {
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

	// step 3: remove groups whose intent does not match the configuration
	for g := range state {
		if !intentOverlaps(g.Intent, c.Intent) {
			delete(state, g)
		}
	}

	// step 4: radio-button reduction
	reduceRB(state, c.RBGroups, nil, nil)

	return &ViewState{config: c, base: state}
}

// reduceRB reduces each radio-button collection towards at most one visible
// member, which is all a state is allowed to have at any one time (8.11.4.3).
//
// Pinned values are never rewritten: several pinned members switched on at
// once — reachable only by a set-OCG-state action that asks for radio-button
// relationships to be ignored — are honoured as the explicit request they
// are.  A pinned member which is on silences the members a usage application
// recommended on, since a pin is the user's word and a recommendation is
// not; a member on from the prescribed states alone stands even then,
// because the only way to reach that combination is the same explicit
// ignore-RB request.  With no pinned member on, the first visible member in
// array order is kept: with several switched on there is nothing to prefer a
// later one.
//
// Groups the state does not cover take no part, since they decide the
// visibility of nothing, and a group listed twice is not its own sibling.
func reduceRB(state map[*Group]bool, rbGroups [][]*Group, pinned, recommended func(*Group) bool) {
	if pinned == nil {
		pinned = func(*Group) bool { return false }
	}
	if recommended == nil {
		recommended = func(*Group) bool { return false }
	}
	for _, rb := range rbGroups {
		pinnedOn := false
		for _, g := range rb {
			if on, ok := state[g]; ok && on && pinned(g) {
				pinnedOn = true
				break
			}
		}

		if pinnedOn {
			for _, g := range rb {
				if on, ok := state[g]; !ok || !on || pinned(g) {
					continue
				}
				if recommended(g) {
					state[g] = false
				}
			}
			continue
		}

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
}

// Effective derives the group visibility in force for a view under the given
// context: the prescribed states, then the recommendations of the
// configuration's View-event usage applications — which pinned groups are
// exempt from — then the pins, with each radio-button collection reduced to
// at most one visible member.  A pinned member that is on displaces the
// recommended members of its collections; among unpinned members the first
// visible one in array order wins.
//
// The context supplies the external factors the usage categories read; a nil
// context, or a context field left at its zero value, yields no
// recommendation for the categories that need it.  The result is an
// independent snapshot: later changes to the state do not affect it.
//
// Effective on a nil state returns a nil snapshot.
func (s *ViewState) Effective(ctx *ViewerContext) *GroupStates {
	if s == nil {
		return nil
	}

	state := maps.Clone(s.base)
	if state == nil {
		state = make(map[*Group]bool)
	}

	// usage recommendations, sparing the pinned groups
	var recs map[*Group]bool
	if s.config != nil {
		recs = s.config.eventRecommendations(EventView, ctx, s.IsManual)
		maps.Copy(state, recs)
	}

	// pins
	maps.Copy(state, s.pins)

	if s.config != nil {
		reduceRB(state, s.config.RBGroups, s.IsManual,
			func(g *Group) bool { _, ok := recs[g]; return ok })
	}

	return &GroupStates{state: state}
}

// EffectiveForPrint derives the group visibility a print copy uses: the
// visibility a view under the given context shows, with the recommendations
// of the configuration's Print-event usage applications applied over it.  A
// print recommendation overrides even a pinned state — it holds for the
// duration of printing only, and the pin remains in force afterwards — and
// counts like a pin in the radio-button reduction.
//
// EffectiveForPrint on a nil state returns a nil snapshot.
func (s *ViewState) EffectiveForPrint(ctx *ViewerContext) *GroupStates {
	snap := s.Effective(ctx)
	if s == nil || s.config == nil {
		return snap
	}

	recs := s.config.eventRecommendations(EventPrint, ctx, nil)
	if len(recs) == 0 {
		return snap
	}
	maps.Copy(snap.state, recs)

	// A collection a print recommendation switched a member on in follows
	// that recommendation: the first recommended-on member in array order
	// wins and every other visible member yields, the view state's choices
	// included.  Collections the recommendations only switched members off
	// in stay valid on their own.
	for _, rb := range s.config.RBGroups {
		var keeper *Group
		for _, g := range rb {
			on, isRec := recs[g]
			if isRec && on && snap.Participates(g) {
				keeper = g
				break
			}
		}
		if keeper == nil {
			continue
		}
		for _, g := range rb {
			if g == keeper {
				continue
			}
			if on, ok := snap.state[g]; ok && on {
				snap.state[g] = false
			}
		}
	}

	return snap
}

// eventRecommendations collects the state recommendations of the
// configuration's usage applications for the given event, ANDed across the
// dictionaries a group appears in (8.11.4.4).  Groups whose intent does not
// match the configuration's receive no recommendation: DefaultState excludes
// them from the state, and a usage application must not bring them back.
//
// skip names groups to withhold recommendations from (the pinned ones, for a
// View derivation); nil means no group is withheld.  A withheld group still
// takes part in choosing the best language match — it merely keeps its own
// state afterwards.  A nil context yields recommendations only for the
// categories that need no external factors.
func (c *Configuration) eventRecommendations(event Event, ctx *ViewerContext, skip func(*Group) bool) map[*Group]bool {
	if len(c.AS) == 0 {
		return nil
	}

	// groups the configuration considers at all
	considers := func(g *Group) bool {
		return intentOverlaps(g.Intent, c.Intent)
	}

	recs := map[*Group]bool{}
	add := func(g *Group, on bool) {
		prev, seen := recs[g]
		if !seen {
			recs[g] = on
		} else {
			recs[g] = prev && on
		}
	}

	for _, ua := range c.AS {
		if ua.Event != event {
			continue
		}

		// per-group evaluation for non-Language categories
		for _, g := range ua.OCGs {
			if g.Usage == nil || !considers(g) || (skip != nil && skip(g)) {
				continue
			}
			on, ok := evaluateUsage(g.Usage, ua.Category, ctx)
			if !ok {
				continue
			}
			add(g, on)
		}

		// collective language evaluation, over the considered groups only:
		// an excluded group must not influence the best-match choice either
		if ctx != nil && ctx.Lang != language.Und &&
			slices.Contains(ua.Category, CategoryLanguage) {
			candidates := make([]*Group, 0, len(ua.OCGs))
			for _, g := range ua.OCGs {
				if considers(g) {
					candidates = append(candidates, g)
				}
			}
			for g, on := range evaluateLanguage(candidates, ctx.Lang) {
				if skip != nil && skip(g) {
					continue
				}
				add(g, on)
			}
		}
	}

	return recs
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
