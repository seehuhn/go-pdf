package operator

import (
	"errors"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/resource"
)

// handleShading implements the sh operator (paint shading)
func handleShading(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if _, ok := res.Shading[name]; !ok {
		return errors.New("shading not found")
	}

	return nil
}

// handleXObject implements the Do operator (paint XObject)
func handleXObject(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	name := p.GetName()
	if err := p.Check(); err != nil {
		return err
	}

	if _, ok := res.XObject[name]; !ok {
		return errors.New("XObject not found")
	}

	return nil
}

// handleMarkedContentPoint implements the MP operator
func handleMarkedContentPoint(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleMarkedContentPointWithProperties implements the DP operator
func handleMarkedContentPointWithProperties(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	// Second arg can be name or dict
	if len(p.args) > 0 {
		p.args = p.args[1:]
	}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginMarkedContent implements the BMC operator
func handleBeginMarkedContent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginMarkedContentWithProperties implements the BDC operator
func handleBeginMarkedContentWithProperties(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetName() // tag
	// Second arg can be name or dict
	if len(p.args) > 0 {
		p.args = p.args[1:]
	}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleEndMarkedContent implements the EMC operator
func handleEndMarkedContent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleType3d0 implements the d0 operator (Type 3 font glyph width)
func handleType3d0(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetFloat() // wx
	_ = p.GetFloat() // wy
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleType3d1 implements the d1 operator (Type 3 font glyph width and bbox)
func handleType3d1(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetFloat() // wx
	_ = p.GetFloat() // wy
	_ = p.GetFloat() // llx
	_ = p.GetFloat() // lly
	_ = p.GetFloat() // urx
	_ = p.GetFloat() // ury
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleBeginCompatibility implements the BX operator
func handleBeginCompatibility(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleEndCompatibility implements the EX operator
func handleEndCompatibility(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleRawContent implements the %raw% special operator
func handleRawContent(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetString() // raw content
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}

// handleInlineImage implements the %image% special operator
func handleInlineImage(s *State, args []pdf.Native, res *resource.Resource) error {
	p := argParser{args: args}
	_ = p.GetDict()   // image parameters
	_ = p.GetString() // image data
	if err := p.Check(); err != nil {
		return err
	}
	return nil
}
