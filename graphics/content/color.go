// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2025  Jochen Voss <voss@seehuhn.de>
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

package content

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
)

// handleSetStrokeColorSpace implements the CS operator
func handleSetStrokeColorSpace(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	// Handle device color spaces directly
	var cs color.Space
	switch name {
	case "DeviceGray":
		cs = color.SpaceDeviceGray
	case "DeviceRGB":
		cs = color.SpaceDeviceRGB
	case "DeviceCMYK":
		cs = color.SpaceDeviceCMYK
	default:
		// Look up in resources
		var ok bool
		cs, ok = res.ColorSpace[name]
		if !ok {
			return errors.New("color space not found")
		}
	}

	s.Param.StrokeColor = cs.Default()
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillColorSpace implements the cs operator
func handleSetFillColorSpace(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	var cs color.Space
	switch name {
	case "DeviceGray":
		cs = color.SpaceDeviceGray
	case "DeviceRGB":
		cs = color.SpaceDeviceRGB
	case "DeviceCMYK":
		cs = color.SpaceDeviceCMYK
	default:
		var ok bool
		cs, ok = res.ColorSpace[name]
		if !ok {
			return errors.New("color space not found")
		}
	}

	s.Param.FillColor = cs.Default()
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeColor implements the SC operator
func handleSetStrokeColor(s *GraphicsState, args []pdf.Native, res *Resources) error {
	// For simplicity, just mark the dependency
	// Full implementation would parse components based on current color space
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetStrokeColorN implements the SCN operator
func handleSetStrokeColorN(s *GraphicsState, args []pdf.Native, res *Resources) error {
	// Similar to SC but supports patterns
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillColor implements the sc operator
func handleSetFillColor(s *GraphicsState, args []pdf.Native, res *Resources) error {
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetFillColorN implements the scn operator
func handleSetFillColorN(s *GraphicsState, args []pdf.Native, res *Resources) error {
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeGray implements the G operator
func handleSetStrokeGray(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	gray := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.StrokeColor = color.DeviceGray(gray)
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillGray implements the g operator
func handleSetFillGray(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	gray := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FillColor = color.DeviceGray(gray)
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeRGB implements the RG operator
func handleSetStrokeRGB(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	r := p.GetFloat()
	g := p.GetFloat()
	b := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.StrokeColor = color.DeviceRGB(r, g, b)
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillRGB implements the rg operator
func handleSetFillRGB(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	r := p.GetFloat()
	g := p.GetFloat()
	b := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FillColor = color.DeviceRGB(r, g, b)
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeCMYK implements the K operator
func handleSetStrokeCMYK(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	c := p.GetFloat()
	m := p.GetFloat()
	y := p.GetFloat()
	k := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.StrokeColor = color.DeviceCMYK(c, m, y, k)
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillCMYK implements the k operator
func handleSetFillCMYK(s *GraphicsState, args []pdf.Native, res *Resources) error {
	p := argParser{args: args}
	c := p.GetFloat()
	m := p.GetFloat()
	y := p.GetFloat()
	k := p.GetFloat()
	if err := p.Check(); err != nil {
		return err
	}

	s.Param.FillColor = color.DeviceCMYK(c, m, y, k)
	s.markOut(graphics.StateFillColor)
	return nil
}
