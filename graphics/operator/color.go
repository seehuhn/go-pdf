package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/resource"
)

// handleSetStrokeColorSpace implements the CS operator
func handleSetStrokeColorSpace(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetFillColorSpace(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetStrokeColor(s *State, args []pdf.Native, res *resource.Resource) error {
	// For simplicity, just mark the dependency
	// Full implementation would parse components based on current color space
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetStrokeColorN implements the SCN operator
func handleSetStrokeColorN(s *State, args []pdf.Native, res *resource.Resource) error {
	// Similar to SC but supports patterns
	s.markOut(graphics.StateStrokeColor)
	return nil
}

// handleSetFillColor implements the sc operator
func handleSetFillColor(s *State, args []pdf.Native, res *resource.Resource) error {
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetFillColorN implements the scn operator
func handleSetFillColorN(s *State, args []pdf.Native, res *resource.Resource) error {
	s.markOut(graphics.StateFillColor)
	return nil
}

// handleSetStrokeGray implements the G operator
func handleSetStrokeGray(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetFillGray(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetStrokeRGB(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetFillRGB(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetStrokeCMYK(s *State, args []pdf.Native, res *resource.Resource) error {
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
func handleSetFillCMYK(s *State, args []pdf.Native, res *resource.Resource) error {
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
