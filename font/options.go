// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2023  Jochen Voss <voss@seehuhn.de>
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

package font

import (
	"golang.org/x/text/language"
	"seehuhn.de/go/pdf/font/cmap"
)

// Options allows to customize fonts for embedding into PDF files.
// Not all fields apply to all font types.
type Options struct {
	Language     language.Tag
	MakeGIDToCID func() cmap.GIDToCID
	MakeEncoder  func(cmap.GIDToCID) cmap.CIDEncoder
	GsubFeatures map[string]bool
	GposFeatures map[string]bool
}

// MergeOptions takes an options struct and a default values struct and returns a new
// options struct with all fields set to the values from the options struct,
// except for the fields which are set to the zero value in the options struct.
// `opt` can be nil in which case the default values are returned.
// `defaultValues` must not be nil.
func MergeOptions(opt, defaultValues *Options) *Options {
	if opt == nil {
		return defaultValues
	}

	res := &Options{}
	var zeroLang language.Tag
	if opt.Language != zeroLang {
		res.Language = opt.Language
	} else {
		res.Language = defaultValues.Language
	}
	if opt.MakeGIDToCID != nil {
		res.MakeGIDToCID = opt.MakeGIDToCID
	} else {
		res.MakeGIDToCID = defaultValues.MakeGIDToCID
	}
	if opt.MakeEncoder != nil {
		res.MakeEncoder = opt.MakeEncoder
	} else {
		res.MakeEncoder = defaultValues.MakeEncoder
	}
	if opt.GsubFeatures != nil {
		res.GsubFeatures = opt.GsubFeatures
	} else {
		res.GsubFeatures = defaultValues.GsubFeatures
	}
	if opt.GposFeatures != nil {
		res.GposFeatures = opt.GposFeatures
	} else {
		res.GposFeatures = defaultValues.GposFeatures
	}
	return res
}
