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

package printprep

import "seehuhn.de/go/pdf"

// maxResourceDepth bounds recursion through nested form XObjects and patterns,
// protecting against cycles and pathologically deep nesting in malformed input.
const maxResourceDepth = 40

// form XObject dictionary keys that are carried over.  Content-defining keys
// (Length, Filter, DecodeParms) are rewritten, Resources is converted, and
// everything else (metadata, structure, optional content, piece info, ...) is
// dropped.
var keepFormKeys = []pdf.Name{
	"Subtype",
	"FormType",
	"BBox",
	"Matrix",
	"Group",
}

// convertResources builds a print-safe resource dictionary: form XObjects and
// tiling patterns are rewritten so their content is transformed too, images
// and other resources are copied unchanged.  A nil or non-dictionary argument
// yields nil.
func (c *converter) convertResources(resObj pdf.Object, depth int, fontDict pdf.Object) (pdf.Object, error) {
	src, err := pdf.CursorAt(c.x, nil).Dict(resObj)
	if err != nil || src == nil {
		return nil, err
	}

	out := pdf.Dict{}
	for key, val := range src {
		switch key {
		case "Font":
			// converted by the content walk; unused fonts are dropped
			if fontDict != nil {
				out[key] = fontDict
			}
		case "XObject":
			cv, err := c.convertXObjects(val, depth)
			if err != nil {
				return nil, err
			}
			out[key] = cv
		case "Pattern":
			cv, err := c.convertPatterns(val, depth)
			if err != nil {
				return nil, err
			}
			out[key] = cv
		default:
			// ExtGState, ColorSpace, Shading, Properties, ProcSet: copied.
			nv, ok := val.(pdf.Native)
			if !ok {
				continue
			}
			cv, err := c.copy.Copy(nv)
			if err != nil {
				return nil, err
			}
			out[key] = cv
		}
	}
	return out, nil
}

// convertXObjects converts an /XObject subdictionary: form XObjects are
// rewritten, image (and other) XObjects are copied.
func (c *converter) convertXObjects(obj pdf.Object, depth int) (pdf.Object, error) {
	src, err := pdf.CursorAt(c.x, nil).Dict(obj)
	if err != nil || src == nil {
		return nil, err
	}
	out := pdf.Dict{}
	for name, val := range src {
		cv, err := c.convertXObject(val, depth)
		if err != nil {
			return nil, err
		}
		if cv != nil {
			out[name] = cv
		}
	}
	return out, nil
}

// convertXObject converts a single XObject value, rewriting form XObjects and
// copying anything else.
func (c *converter) convertXObject(val pdf.Object, depth int) (pdf.Object, error) {
	cur := pdf.CursorAt(c.x, nil)
	stm, err := cur.Stream(val)
	if err != nil || stm == nil {
		return c.copyNative(val)
	}
	if stm.Dict["Subtype"] != pdf.Name("Form") || depth >= maxResourceDepth {
		return c.copyNative(val)
	}
	ref, ok := val.(pdf.Reference)
	if !ok {
		// direct (inline) form XObject: no reference to cache or cycle on
		return c.buildForm(stm, depth)
	}
	if out, ok := c.formCache[ref]; ok {
		return out, nil
	}
	outRef := c.out.Alloc()
	c.formCache[ref] = outRef // publish before recursing, breaking cycles
	built, err := c.buildFormAt(outRef, stm, depth)
	if err != nil {
		return nil, err
	}
	return built, nil
}

// buildForm writes a rewritten copy of a form XObject to a fresh object and
// returns its reference.
func (c *converter) buildForm(stm *pdf.Stream, depth int) (pdf.Reference, error) {
	return c.buildFormAt(c.out.Alloc(), stm, depth)
}

// buildFormAt writes the rewritten form XObject stm to outRef.
func (c *converter) buildFormAt(outRef pdf.Reference, stm *pdf.Stream, depth int) (pdf.Reference, error) {
	srcRes, _ := pdf.CursorAt(c.x, nil).Dict(stm.Dict["Resources"])

	fc := c.newFontContext(srcRes)
	body, nesting, err := c.rewriteContentBytes(stm, srcRes, fc)
	if err != nil {
		return 0, err
	}
	body = closeGraphicsState(body, nesting)
	fontDict, err := fc.subdict()
	if err != nil {
		return 0, err
	}
	newRes, err := c.convertResources(stm.Dict["Resources"], depth+1, fontDict)
	if err != nil {
		return 0, err
	}

	dict := pdf.Dict{"Type": pdf.Name("XObject")}
	for _, key := range keepFormKeys {
		nv, ok := stm.Dict[key].(pdf.Native)
		if !ok {
			continue
		}
		cv, err := c.copy.Copy(nv)
		if err != nil {
			return 0, err
		}
		dict[key] = cv
	}
	if newRes != nil {
		dict["Resources"] = newRes
	}
	if dict["Subtype"] == nil {
		dict["Subtype"] = pdf.Name("Form")
	}

	w, err := c.out.OpenStream(outRef, dict, pdf.FilterCompress{})
	if err != nil {
		return 0, err
	}
	if _, err := w.Write(body); err != nil {
		return 0, err
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	return outRef, nil
}

// convertPatterns converts a /Pattern subdictionary: tiling patterns (streams)
// are rewritten, shading patterns (dictionaries) are copied.
func (c *converter) convertPatterns(obj pdf.Object, depth int) (pdf.Object, error) {
	src, err := pdf.CursorAt(c.x, nil).Dict(obj)
	if err != nil || src == nil {
		return nil, err
	}
	out := pdf.Dict{}
	for name, val := range src {
		cv, err := c.convertPattern(val, depth)
		if err != nil {
			return nil, err
		}
		if cv != nil {
			out[name] = cv
		}
	}
	return out, nil
}

// convertPattern converts a single pattern value: a tiling pattern (stream) is
// rewritten, anything else copied.  Tiling patterns are deduplicated and
// cycle-broken through patternCache.
func (c *converter) convertPattern(val pdf.Object, depth int) (pdf.Object, error) {
	cur := pdf.CursorAt(c.x, nil)
	stm, err := cur.Stream(val)
	if err != nil || stm == nil || depth >= maxResourceDepth {
		return c.copyNative(val)
	}
	ref, ok := val.(pdf.Reference)
	if !ok {
		return c.buildPatternAt(c.out.Alloc(), stm, depth)
	}
	if out, ok := c.patternCache[ref]; ok {
		return out, nil
	}
	outRef := c.out.Alloc()
	c.patternCache[ref] = outRef // publish before recursing, breaking cycles
	return c.buildPatternAt(outRef, stm, depth)
}

// buildPatternAt writes a rewritten copy of a tiling pattern to outRef.
func (c *converter) buildPatternAt(outRef pdf.Reference, stm *pdf.Stream, depth int) (pdf.Reference, error) {
	srcRes, _ := pdf.CursorAt(c.x, nil).Dict(stm.Dict["Resources"])

	fc := c.newFontContext(srcRes)
	body, nesting, err := c.rewriteContentBytes(stm, srcRes, fc)
	if err != nil {
		return 0, err
	}
	body = closeGraphicsState(body, nesting)
	fontDict, err := fc.subdict()
	if err != nil {
		return 0, err
	}
	newRes, err := c.convertResources(stm.Dict["Resources"], depth+1, fontDict)
	if err != nil {
		return 0, err
	}

	dict := pdf.Dict{}
	for key, val := range stm.Dict {
		switch key {
		case "Length", "Filter", "DecodeParms", "Resources":
			continue
		}
		nv, ok := val.(pdf.Native)
		if !ok {
			continue
		}
		cv, err := c.copy.Copy(nv)
		if err != nil {
			return 0, err
		}
		dict[key] = cv
	}
	if newRes != nil {
		dict["Resources"] = newRes
	}

	w, err := c.out.OpenStream(outRef, dict, pdf.FilterCompress{})
	if err != nil {
		return 0, err
	}
	if _, err := w.Write(body); err != nil {
		return 0, err
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	return outRef, nil
}

// copyNative copies a value through the copier, skipping non-Native values
// (which cannot occur in well-formed input).
func (c *converter) copyNative(val pdf.Object) (pdf.Object, error) {
	nv, ok := val.(pdf.Native)
	if !ok {
		return nil, nil
	}
	return c.copy.Copy(nv)
}
