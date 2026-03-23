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
	"errors"

	"seehuhn.de/go/pdf"
)

// PDF 2.0 sections: 8.11.4.3

// BaseState controls the initial group states when a configuration is applied.
type BaseState pdf.Name

const (
	// BaseStateON sets all groups to ON.
	BaseStateON BaseState = "ON"

	// BaseStateOFF sets all groups to OFF.
	BaseStateOFF BaseState = "OFF"

	// BaseStateUnchanged leaves all group states unchanged.
	BaseStateUnchanged BaseState = "Unchanged"
)

// ListMode controls which groups in the Order array are displayed to the user.
type ListMode pdf.Name

const (
	// ListModeAllPages displays all groups in the Order array.
	ListModeAllPages ListMode = "AllPages"

	// ListModeVisiblePages displays only groups referenced by visible pages.
	ListModeVisiblePages ListMode = "VisiblePages"
)

// Configuration represents an optional content configuration dictionary
// that specifies initial states and display settings for optional content groups.
// This corresponds to Table 99 in the PDF specification.
type Configuration struct {
	// Name (optional) is a name for the configuration, suitable for
	// presentation in a user interface.
	Name string

	// Creator (optional) is the name of the application or feature that
	// created this configuration dictionary.
	Creator string

	// BaseState (optional) controls initial group states when this
	// configuration is applied.
	// When encoding, an empty value is treated as BaseStateON.
	BaseState BaseState

	// ON (optional) lists groups forced ON after BaseState is applied.
	ON []*Group

	// OFF (optional) lists groups forced OFF after BaseState is applied.
	OFF []*Group

	// Intent (optional) lists intents to consider when determining visibility.
	// Content controlled by groups with no matching intent is always shown.
	//
	// A nil slice means the entry is absent from the PDF (defaults to "View").
	// A non-nil empty slice means an explicit empty array was present,
	// which per the spec means no groups participate and all content is visible.
	Intent []pdf.Name

	// AS (optional) lists usage application dictionaries for automatic
	// state management.
	AS []*UsageApplication

	// Order (optional) specifies the presentation hierarchy for a user interface.
	Order []OrderItem

	// ListMode (optional) controls which groups are displayed.
	// When encoding, an empty value is treated as ListModeAllPages.
	ListMode ListMode

	// RBGroups (optional) lists collections of groups that follow a
	// radio-button paradigm.
	RBGroups [][]*Group

	// Locked (optional, PDF 1.6) lists groups locked in the user interface.
	Locked []*Group

	// SingleUse determines if Embed returns a dictionary (true) or
	// a reference (false).
	SingleUse bool
}

var _ pdf.Embedder = (*Configuration)(nil)

// ExtractConfiguration extracts an optional content configuration dictionary
// from a PDF object.
func ExtractConfiguration(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, isDirect bool) (*Configuration, error) {
	r := x.R
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing optional content configuration dictionary")
	}

	c := &Configuration{}

	// Name (optional)
	if name, err := pdf.Optional(pdf.GetTextString(r, dict["Name"])); err != nil {
		return nil, err
	} else {
		c.Name = string(name)
	}

	// Creator (optional)
	if creator, err := pdf.Optional(pdf.GetTextString(r, dict["Creator"])); err != nil {
		return nil, err
	} else {
		c.Creator = string(creator)
	}

	// BaseState (optional, default ON)
	if bs, err := pdf.Optional(pdf.GetName(r, dict["BaseState"])); err != nil {
		return nil, err
	} else {
		switch BaseState(bs) {
		case BaseStateON, BaseStateOFF, BaseStateUnchanged:
			c.BaseState = BaseState(bs)
		default:
			c.BaseState = BaseStateON
		}
	}

	// ON (optional)
	c.ON, err = extractGroupArray(x, path, dict["ON"])
	if err != nil {
		return nil, err
	}

	// OFF (optional)
	c.OFF, err = extractGroupArray(x, path, dict["OFF"])
	if err != nil {
		return nil, err
	}

	// Intent (optional)
	// A nil slice means absent (defaults to "View" at evaluation time).
	// A non-nil empty slice means an explicit empty array was present
	// (per spec: no groups participate, all content visible).
	intentObj, err := x.Resolve(path, dict["Intent"])
	if err != nil {
		return nil, err
	}
	switch intent := intentObj.(type) {
	case pdf.Array:
		// preserve explicit empty array as non-nil empty slice
		c.Intent = []pdf.Name{}
		for _, o := range intent {
			if name, err := pdf.Optional(x.GetName(path, o)); err != nil {
				return nil, err
			} else if name != "" {
				c.Intent = append(c.Intent, name)
			}
		}
	case pdf.Name:
		if intent != "" {
			c.Intent = []pdf.Name{intent}
		}
	}

	// AS (optional)
	if asArr, err := pdf.Optional(pdf.GetArray(r, dict["AS"])); err != nil {
		return nil, err
	} else {
		for _, item := range asArr {
			ua, err := pdf.ExtractorGetOptional(x, path, item, ExtractUsageApplication)
			if err != nil {
				continue // permissive
			}
			if ua != nil {
				c.AS = append(c.AS, ua)
			}
		}
	}

	// Order (optional)
	if orderArr, err := pdf.Optional(pdf.GetArray(r, dict["Order"])); err != nil {
		return nil, err
	} else if len(orderArr) > 0 {
		c.Order, err = extractOrderItems(x, path, orderArr)
		if err != nil {
			return nil, err
		}
	}

	// ListMode (optional, default AllPages)
	if lm, err := pdf.Optional(pdf.GetName(r, dict["ListMode"])); err != nil {
		return nil, err
	} else {
		switch ListMode(lm) {
		case ListModeAllPages, ListModeVisiblePages:
			c.ListMode = ListMode(lm)
		default:
			c.ListMode = ListModeAllPages
		}
	}

	// RBGroups (optional)
	if rbArr, err := pdf.Optional(pdf.GetArray(r, dict["RBGroups"])); err != nil {
		return nil, err
	} else {
		for _, item := range rbArr {
			inner, err := pdf.Optional(pdf.GetArray(r, item))
			if err != nil {
				continue
			}
			groups, err := extractGroupArrayFromArray(x, path, inner)
			if err != nil {
				continue
			}
			if len(groups) > 0 {
				c.RBGroups = append(c.RBGroups, groups)
			}
		}
	}

	// Locked (optional, PDF 1.6)
	c.Locked, err = extractGroupArray(x, path, dict["Locked"])
	if err != nil {
		return nil, err
	}

	c.SingleUse = isDirect

	return c, nil
}

// Embed adds the configuration dictionary to a PDF file.
func (c *Configuration) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "optional content configuration", pdf.V1_5); err != nil {
		return nil, err
	}

	dict := pdf.Dict{}

	if c.Name != "" {
		dict["Name"] = pdf.TextString(c.Name)
	}
	if c.Creator != "" {
		dict["Creator"] = pdf.TextString(c.Creator)
	}

	// BaseState (default ON, omit when default)
	bs := c.BaseState
	if bs == "" {
		bs = BaseStateON
	}
	switch bs {
	case BaseStateON:
		// omit; ON is the default
	case BaseStateOFF, BaseStateUnchanged:
		dict["BaseState"] = pdf.Name(bs)
	default:
		return nil, errors.New("invalid BaseState value")
	}

	// ON
	if len(c.ON) > 0 {
		arr, err := embedGroupArray(rm, c.ON)
		if err != nil {
			return nil, err
		}
		dict["ON"] = arr
	}

	// OFF
	if len(c.OFF) > 0 {
		arr, err := embedGroupArray(rm, c.OFF)
		if err != nil {
			return nil, err
		}
		dict["OFF"] = arr
	}

	// Intent
	// nil = absent (default View); non-nil empty = explicit empty array
	if c.Intent != nil {
		if len(c.Intent) == 0 {
			dict["Intent"] = pdf.Array{}
		} else if len(c.Intent) == 1 {
			dict["Intent"] = c.Intent[0]
		} else {
			intentArr := make(pdf.Array, len(c.Intent))
			for i, intent := range c.Intent {
				intentArr[i] = intent
			}
			dict["Intent"] = intentArr
		}
	}

	// AS
	if len(c.AS) > 0 {
		asArr := make(pdf.Array, len(c.AS))
		for i, ua := range c.AS {
			obj, err := rm.Embed(ua)
			if err != nil {
				return nil, err
			}
			asArr[i] = obj
		}
		dict["AS"] = asArr
	}

	// Order
	if len(c.Order) > 0 {
		orderArr, err := embedOrderItems(rm, c.Order)
		if err != nil {
			return nil, err
		}
		dict["Order"] = orderArr
	}

	// ListMode (default AllPages)
	lm := c.ListMode
	if lm == "" {
		lm = ListModeAllPages
	}
	switch lm {
	case ListModeAllPages:
		// default, omit
	case ListModeVisiblePages:
		dict["ListMode"] = pdf.Name(lm)
	default:
		return nil, errors.New("invalid ListMode value")
	}

	// RBGroups
	if len(c.RBGroups) > 0 {
		rbArr := make(pdf.Array, len(c.RBGroups))
		for i, groups := range c.RBGroups {
			if len(groups) == 0 {
				return nil, errors.New("RBGroups inner array must not be empty")
			}
			inner, err := embedGroupArray(rm, groups)
			if err != nil {
				return nil, err
			}
			rbArr[i] = inner
		}
		dict["RBGroups"] = rbArr
	}

	// Locked (PDF 1.6)
	if len(c.Locked) > 0 {
		if err := pdf.CheckVersion(rm.Out(), "locked optional content groups", pdf.V1_6); err != nil {
			return nil, err
		}
		arr, err := embedGroupArray(rm, c.Locked)
		if err != nil {
			return nil, err
		}
		dict["Locked"] = arr
	}

	if c.SingleUse {
		return dict, nil
	}

	ref := rm.Alloc()
	err := rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}

// extractGroupArray extracts an array of Group references from a PDF object.
func extractGroupArray(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object) ([]*Group, error) {
	arr, err := pdf.Optional(pdf.GetArray(x.R, obj))
	if err != nil {
		return nil, err
	}
	return extractGroupArrayFromArray(x, path, arr)
}

// extractGroupArrayFromArray extracts Groups from an already-resolved PDF array.
func extractGroupArrayFromArray(x *pdf.Extractor, path *pdf.CycleCheck, arr pdf.Array) ([]*Group, error) {
	if len(arr) == 0 {
		return nil, nil
	}
	var groups []*Group
	for _, item := range arr {
		group, err := pdf.ExtractorGetOptional(x, path, item, ExtractGroup)
		if err != nil {
			continue // permissive
		}
		if group != nil {
			groups = append(groups, group)
		}
	}
	if len(groups) == 0 {
		return nil, nil
	}
	return groups[:len(groups):len(groups)], nil
}

// embedGroupArray embeds an array of Groups.
func embedGroupArray(rm *pdf.EmbedHelper, groups []*Group) (pdf.Array, error) {
	arr := make(pdf.Array, len(groups))
	for i, g := range groups {
		obj, err := rm.Embed(g)
		if err != nil {
			return nil, err
		}
		arr[i] = obj
	}
	return arr, nil
}

// maxOrderItems is the maximum number of items (groups and sub-groups)
// allowed in an Order array. This represents user-selectable check-boxes
// in a layer panel, so real documents need far fewer.
const maxOrderItems = 1000

// extractOrderItems extracts Order items from a PDF array.
func extractOrderItems(x *pdf.Extractor, path *pdf.CycleCheck, arr pdf.Array) ([]OrderItem, error) {
	remaining := maxOrderItems
	return doExtractOrderItems(x, path, arr, &remaining)
}

func doExtractOrderItems(x *pdf.Extractor, path *pdf.CycleCheck, arr pdf.Array, remaining *int) ([]OrderItem, error) {
	var items []OrderItem
	for _, elem := range arr {
		if *remaining <= 0 {
			break
		}
		resolved, err := x.Resolve(path, elem)
		if err != nil {
			continue
		}
		switch v := resolved.(type) {
		case pdf.Array:
			// nested array: may have text label as first element
			*remaining--
			og := &OrderGroup{}
			start := 0
			if len(v) > 0 {
				if label, err := pdf.Optional(pdf.GetTextString(x.R, v[0])); err == nil && label != "" {
					og.Label = string(label)
					start = 1
				}
			}
			children, err := doExtractOrderItems(x, path, v[start:], remaining)
			if err != nil {
				continue
			}
			og.Children = children
			items = append(items, og)
		default:
			// try to extract as a Group
			group, err := pdf.ExtractorGetOptional(x, path, elem, ExtractGroup)
			if err != nil {
				continue
			}
			if group != nil {
				*remaining--
				items = append(items, group)
			}
		}
	}
	return items, nil
}

// embedOrderItems embeds Order items as a PDF array.
func embedOrderItems(rm *pdf.EmbedHelper, items []OrderItem) (pdf.Array, error) {
	arr := make(pdf.Array, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case *Group:
			obj, err := rm.Embed(v)
			if err != nil {
				return nil, err
			}
			arr = append(arr, obj)
		case *OrderGroup:
			children, err := embedOrderItems(rm, v.Children)
			if err != nil {
				return nil, err
			}
			inner := children
			if v.Label != "" {
				inner = make(pdf.Array, 0, len(children)+1)
				inner = append(inner, pdf.TextString(v.Label))
				inner = append(inner, children...)
			}
			arr = append(arr, inner)
		}
	}
	return arr, nil
}
