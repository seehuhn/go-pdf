package builder

import (
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/color"
	"seehuhn.de/go/pdf/graphics/content"
)

// Builder constructs PDF content streams using a type-safe API.
//
// PDF version requirements are not checked during construction; use
// [content.Stream.Validate] before writing to verify compatibility.
type Builder struct {
	Resources *content.Resources
	Stream    content.Stream
	State     *content.GraphicsState
	Err       error

	// resName tracks allocated resource names for deduplication
	resName map[resKey]pdf.Name

	// glyphBuf is reused by TextShow to avoid allocations
	glyphBuf *font.GlyphSeq
}

// resKey identifies a resource for deduplication purposes.
// The prefix is included so the same object can be used in different categories.
type resKey struct {
	prefix pdf.Name
	obj    pdf.Embedder
}

// New creates a new Builder with initialized state.
func New() *Builder {
	return &Builder{
		Resources: &content.Resources{},
		State:     content.NewState(),
		resName:   make(map[resKey]pdf.Name),
	}
}

// emit appends an operator to the content stream and updates the graphics state.
//
// The state update (via State.Apply) validates operator context (e.g., text
// operators require BT...ET), tracks required input state (via State.In),
// updates output state (via State.Out), manages the graphics state stack
// for q/Q, and maintains path state (current point, subpath closedness).
//
// If an error occurs (including context validation failures), it is stored
// in b.Err and subsequent calls become no-ops.
func (b *Builder) emit(name content.OpName, args ...pdf.Object) {
	if b.Err != nil {
		return
	}
	op := content.Operator{Name: name, Args: args}
	if err := b.State.Apply(b.Resources, op); err != nil {
		b.Err = err
		return
	}
	b.Stream = append(b.Stream, op)
}

func (b *Builder) isSet(bits graphics.StateBits) bool {
	return b.State.Out&bits == bits
}

func (b *Builder) getColorSpaceName(cs color.Space) pdf.Name {
	key := resKey{"C", cs}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.ColorSpace == nil {
		b.Resources.ColorSpace = make(map[pdf.Name]color.Space)
	}
	name := allocateName("C", b.Resources.ColorSpace)
	b.Resources.ColorSpace[name] = cs
	b.resName[key] = name
	return name
}

func (b *Builder) getExtGStateName(gs *graphics.ExtGState) pdf.Name {
	key := resKey{"E", gs}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.ExtGState == nil {
		b.Resources.ExtGState = make(map[pdf.Name]*graphics.ExtGState)
	}
	name := allocateName("E", b.Resources.ExtGState)
	b.Resources.ExtGState[name] = gs
	b.resName[key] = name
	return name
}

func (b *Builder) getFontName(f font.Instance) pdf.Name {
	key := resKey{"F", f}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Font == nil {
		b.Resources.Font = make(map[pdf.Name]font.Instance)
	}
	name := allocateName("F", b.Resources.Font)
	b.Resources.Font[name] = f
	b.resName[key] = name
	return name
}

func (b *Builder) getPatternName(p color.Pattern) pdf.Name {
	key := resKey{"P", p}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Pattern == nil {
		b.Resources.Pattern = make(map[pdf.Name]color.Pattern)
	}
	name := allocateName("P", b.Resources.Pattern)
	b.Resources.Pattern[name] = p
	b.resName[key] = name
	return name
}

func (b *Builder) getShadingName(s graphics.Shading) pdf.Name {
	key := resKey{"S", s}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.Shading == nil {
		b.Resources.Shading = make(map[pdf.Name]graphics.Shading)
	}
	name := allocateName("S", b.Resources.Shading)
	b.Resources.Shading[name] = s
	b.resName[key] = name
	return name
}

func (b *Builder) getXObjectName(x graphics.XObject) pdf.Name {
	key := resKey{"X", x}
	if name, ok := b.resName[key]; ok {
		return name
	}
	if b.Resources.XObject == nil {
		b.Resources.XObject = make(map[pdf.Name]graphics.XObject)
	}
	name := allocateName("X", b.Resources.XObject)
	b.Resources.XObject[name] = x
	b.resName[key] = name
	return name
}

func allocateName[T any](prefix pdf.Name, dict map[pdf.Name]T) pdf.Name {
	// Start from len+1 and decrement to avoid quadratic complexity when
	// allocating many resources: the first check usually succeeds.
	for k := len(dict) + 1; ; k-- {
		name := pdf.Name(string(prefix) + strconv.Itoa(k))
		if _, exists := dict[name]; !exists {
			return name
		}
	}
}
