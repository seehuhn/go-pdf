package builder

import (
	"fmt"
	"math"
	"strconv"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
	"seehuhn.de/go/pdf/graphics/content"
)

// Builder constructs PDF content streams using a type-safe API.
type Builder struct {
	Resources *content.Resources
	Stream    content.Stream
	State     *content.GraphicsState
	Err       error

	// resName tracks allocated resource names for deduplication
	resName map[resKey]pdf.Name
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

// emit appends an operator to the stream and applies it to the state.
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

// SetLineWidth sets the line width.
//
// This implements the PDF graphics operator "w".
func (b *Builder) SetLineWidth(width float64) {
	if width < 0 {
		b.Err = fmt.Errorf("SetLineWidth: negative width %f", width)
		return
	}
	if b.State.Out&graphics.StateLineWidth != 0 && nearlyEqual(width, b.State.Param.LineWidth) {
		return
	}
	b.emit(content.OpSetLineWidth, pdf.Number(width))
}

func nearlyEqual(a, b float64) bool {
	const ε = 1e-6
	return math.Abs(a-b) < ε
}

// allocateName generates a new unique name with the given prefix in the dict.
func allocateName[T any](prefix pdf.Name, dict map[pdf.Name]T) pdf.Name {
	for i := 1; ; i++ {
		name := pdf.Name(string(prefix) + strconv.Itoa(i))
		if _, exists := dict[name]; !exists {
			return name
		}
	}
}
