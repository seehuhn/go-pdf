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

// PDF 2.0 sections: 8.11.4.2

// Properties represents the optional content properties dictionary
// that contains all optional content groups and their configurations.
// This corresponds to Table 98 in the PDF specification.
type Properties struct {
	// OCGs lists all optional content groups in the document.
	OCGs []*Group

	// D is the default configuration, defining initial group states
	// and UI presentation.
	D *Configuration

	// Configs (optional) lists alternate configurations.
	Configs []*Configuration
}

var _ pdf.Embedder = (*Properties)(nil)

// ExtractProperties extracts an optional content properties dictionary
// from a PDF object.
func ExtractProperties(x *pdf.Extractor, path *pdf.CycleCheck, obj pdf.Object, _ bool) (*Properties, error) {
	r := x.R
	dict, err := pdf.GetDict(r, obj)
	if err != nil {
		return nil, err
	} else if dict == nil {
		return nil, pdf.Error("missing optional content properties dictionary")
	}

	p := &Properties{}

	// OCGs (required)
	p.OCGs, err = extractGroupArray(x, path, dict["OCGs"])
	if err != nil {
		return nil, err
	}
	if len(p.OCGs) == 0 {
		return nil, pdf.Error("missing OCGs array in optional content properties")
	}

	// D (required)
	p.D, err = pdf.ExtractorGetOptional(x, path, dict["D"], ExtractConfiguration)
	if err != nil {
		return nil, err
	}
	if p.D == nil {
		// permissive: synthesize a default configuration
		p.D = &Configuration{BaseState: BaseStateON}
	}
	// the default configuration has stricter constraints than alternate ones:
	// its BaseState must be ON and its Intent must be View (8.11.4.3).  Snap
	// any non-conforming value so a permissively-read Properties round-trips
	// through the strict writer.  Copy first so a configuration shared with
	// the Configs array (via the extractor cache) is not mutated.
	if p.D.BaseState != BaseStateON || len(p.D.Intent) != 1 || p.D.Intent[0] != "View" {
		d := *p.D
		d.BaseState = BaseStateON
		d.Intent = []pdf.Name{"View"}
		p.D = &d
	}

	// Configs (optional)
	if configsArr, err := pdf.Optional(pdf.GetArray(r, dict["Configs"])); err != nil {
		return nil, err
	} else {
		for _, item := range configsArr {
			config, err := pdf.ExtractorGetOptional(x, path, item, ExtractConfiguration)
			if err != nil {
				continue // permissive
			}
			if config == nil {
				continue
			}
			// per spec, alternate configs inherit Order and RBGroups from D
			if config.Order == nil {
				config.Order = p.D.Order
			}
			if config.RBGroups == nil {
				config.RBGroups = p.D.RBGroups
			}
			p.Configs = append(p.Configs, config)
		}
	}

	return p, nil
}

// Embed adds the properties dictionary to a PDF file.
func (p *Properties) Embed(rm *pdf.EmbedHelper) (pdf.Native, error) {
	if err := pdf.CheckVersion(rm.Out(), "optional content properties", pdf.V1_5); err != nil {
		return nil, err
	}

	if len(p.OCGs) == 0 {
		return nil, errors.New("Properties.OCGs is required")
	}
	if p.D == nil {
		return nil, errors.New("Properties.D is required")
	}
	if p.D.BaseState != "" && p.D.BaseState != BaseStateON {
		return nil, errors.New("default configuration BaseState must be ON")
	}
	if len(p.D.Intent) == 1 && p.D.Intent[0] != "View" ||
		len(p.D.Intent) > 1 {
		return nil, errors.New("default configuration Intent must be View")
	}

	dict := pdf.Dict{}

	// OCGs
	ocgArr, err := embedGroupArray(rm, p.OCGs)
	if err != nil {
		return nil, err
	}
	dict["OCGs"] = ocgArr

	// D
	dObj, err := rm.Embed(p.D)
	if err != nil {
		return nil, err
	}
	dict["D"] = dObj

	// Configs
	if len(p.Configs) > 0 {
		configsArr := make(pdf.Array, len(p.Configs))
		for i, config := range p.Configs {
			obj, err := rm.Embed(config)
			if err != nil {
				return nil, err
			}
			configsArr[i] = obj
		}
		dict["Configs"] = configsArr
	}

	ref := rm.Alloc()
	err = rm.Out().Put(ref, dict)
	if err != nil {
		return nil, err
	}
	return ref, nil
}
