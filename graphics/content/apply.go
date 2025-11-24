package content

import (
	"errors"
	"fmt"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/graphics"
)

// See Figure 9 (p. 113) of PDF 32000-1:2008.
type Object byte

func (s Object) String() string {
	switch s {
	case ObjPage:
		return "page"
	case ObjPath:
		return "path"
	case ObjText:
		return "text"
	case ObjClippingPath:
		return "clipping path"
	default:
		return fmt.Sprintf("objectType(%d)", s)
	}
}

const (
	ObjPage         Object = 1 << iota // Page-level context (initial state)
	ObjPath                            // Path construction in progress
	ObjText                            // Inside text object (BT...ET)
	ObjClippingPath                    // Clipping path operator executed
)

type GraphicsState struct {
	// Param contains the current values of all graphics parameters.
	// Only those parameters listed in Out are guaranteed to have valid values.
	Param graphics.Parameters

	// In lists all graphics parameters which need to be set before executing
	// a given sequence of operators.
	In graphics.StateBits

	// Out lists all graphics parameters which have been modified
	// by executing a given sequence of operators.
	Out graphics.StateBits

	// CurrentObject lists the current graphics object being constructed.
	CurrentObject Object

	// stack is the graphics state stack for q/Q
	stack []savedState
}

// NewState creates a State initialized to the default PDF graphics state.
// The initial CurrentObject is ObjPage.
func NewState() *GraphicsState {
	return &GraphicsState{
		Param:         *graphics.NewState().Parameters,
		CurrentObject: ObjPage,
	}
}

// Apply applies a single content stream operator to the state.
//
// The res parameter must not be nil. It provides access to fonts, XObjects,
// color spaces, and other PDF resources referenced by operators. Individual
// resource maps within res may be nil if not needed for the content stream.
//
// Returns an error if the operator is unknown, has invalid arguments,
// or is used in an invalid context (e.g., text operators outside BT...ET).
func (state *GraphicsState) Apply(res *Resources, op Operator) error {
	handler, ok := handlers[op.Name]
	if !ok {
		return fmt.Errorf("unknown operator: %s", op.Name)
	}
	return handler(state, op.Args, res)
}

// argParser provides a scanner-style API for parsing operator arguments
type argParser struct {
	args []pdf.Object
	err  error
}

func (p *argParser) GetFloat() float64 {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return 0
	}
	arg := p.args[0]
	p.args = p.args[1:]

	switch x := arg.(type) {
	case pdf.Real:
		return float64(x)
	case pdf.Integer:
		return float64(x)
	default:
		p.err = fmt.Errorf("expected number, got %T", arg)
		return 0
	}
}

func (p *argParser) GetInt() int {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return 0
	}
	arg := p.args[0]
	p.args = p.args[1:]

	i, ok := arg.(pdf.Integer)
	if !ok {
		p.err = fmt.Errorf("expected integer, got %T", arg)
		return 0
	}
	return int(i)
}

func (p *argParser) GetName() pdf.Name {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return ""
	}
	arg := p.args[0]
	p.args = p.args[1:]

	name, ok := arg.(pdf.Name)
	if !ok {
		p.err = fmt.Errorf("expected name, got %T", arg)
		return ""
	}
	return name
}

func (p *argParser) GetArray() pdf.Array {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	arr, ok := arg.(pdf.Array)
	if !ok {
		p.err = fmt.Errorf("expected array, got %T", arg)
		return nil
	}
	return arr
}

func (p *argParser) GetDict() pdf.Dict {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	dict, ok := arg.(pdf.Dict)
	if !ok {
		p.err = fmt.Errorf("expected dict, got %T", arg)
		return nil
	}
	return dict
}

func (p *argParser) GetString() pdf.String {
	if p.err != nil || len(p.args) == 0 {
		if p.err == nil {
			p.err = errors.New("not enough arguments")
		}
		return nil
	}
	arg := p.args[0]
	p.args = p.args[1:]

	str, ok := arg.(pdf.String)
	if !ok {
		p.err = fmt.Errorf("expected string, got %T", arg)
		return nil
	}
	return str
}

func (p *argParser) Check() error {
	if len(p.args) > 0 {
		return errors.New("too many arguments")
	}
	return p.err
}

// State tracking helpers

func (s *GraphicsState) markIn(bits graphics.StateBits) {
	s.In |= (bits &^ s.Out)
}

func (s *GraphicsState) markOut(bits graphics.StateBits) {
	s.Out |= bits
}

// savedState stores graphics state for q/Q operators
type savedState struct {
	param *graphics.Parameters
	out   graphics.StateBits
}

// opHandler is the function signature for operator handlers
type opHandler func(*GraphicsState, []pdf.Object, *Resources) error

// handlers maps operator names to their handler functions
var handlers = map[OpName]opHandler{
	// Graphics state
	OpPushGraphicsState:    handlePushState,
	OpPopGraphicsState:     handlePopState,
	OpTransform:            handleConcatMatrix,
	OpSetLineWidth:         handleSetLineWidth,
	OpSetLineCap:           handleSetLineCap,
	OpSetLineJoin:          handleSetLineJoin,
	OpSetMiterLimit:        handleSetMiterLimit,
	OpSetLineDash:          handleSetLineDash,
	OpSetRenderingIntent:   handleSetRenderingIntent,
	OpSetFlatnessTolerance: handleSetFlatness,
	OpSetExtGState:         handleSetExtGState,

	// Path construction
	OpMoveTo:    handleMoveTo,
	OpLineTo:    handleLineTo,
	OpCurveTo:   handleCurveTo,
	OpCurveToV:  handleCurveToV,
	OpCurveToY:  handleCurveToY,
	OpClosePath: handleClosePath,
	OpRectangle: handleRectangle,

	// Path painting
	OpStroke:                    handleStroke,
	OpCloseAndStroke:            handleCloseAndStroke,
	OpFill:                      handleFill,
	OpFillCompat:                handleFillCompat,
	OpFillEvenOdd:               handleFillEvenOdd,
	OpFillAndStroke:             handleFillAndStroke,
	OpFillAndStrokeEvenOdd:      handleFillAndStrokeEvenOdd,
	OpCloseFillAndStroke:        handleCloseFillAndStroke,
	OpCloseFillAndStrokeEvenOdd: handleCloseFillAndStrokeEvenOdd,
	OpEndPath:                   handleEndPath,
	OpClipNonZero:               handleClip,
	OpClipEvenOdd:               handleClipEvenOdd,

	// Text objects
	OpTextBegin: handleTextBegin,
	OpTextEnd:   handleTextEnd,

	// Text state
	OpTextSetCharacterSpacing:  handleTextSetCharSpacing,
	OpTextSetWordSpacing:       handleTextSetWordSpacing,
	OpTextSetHorizontalScaling: handleTextSetHorizontalScaling,
	OpTextSetLeading:           handleTextSetLeading,
	OpTextSetFont:              handleTextSetFont,
	OpTextSetRenderingMode:     handleTextSetRenderingMode,
	OpTextSetRise:              handleTextSetRise,

	// Text positioning
	OpTextMoveOffset:           handleTextMoveOffset,
	OpTextMoveOffsetSetLeading: handleTextMoveOffsetSetLeading,
	OpTextSetMatrix:            handleTextSetMatrix,
	OpTextNextLine:             handleTextNextLine,

	// Text showing
	OpTextShow:                       handleTextShow,
	OpTextShowArray:                  handleTextShowArray,
	OpTextShowMoveNextLine:           handleTextShowMoveNextLine,
	OpTextShowMoveNextLineSetSpacing: handleTextShowMoveNextLineSetSpacing,

	// Color spaces
	OpSetStrokeColorSpace: handleSetStrokeColorSpace,
	OpSetFillColorSpace:   handleSetFillColorSpace,

	// Generic color
	OpSetStrokeColor:  handleSetStrokeColor,
	OpSetStrokeColorN: handleSetStrokeColorN,
	OpSetFillColor:    handleSetFillColor,
	OpSetFillColorN:   handleSetFillColorN,

	// Device colors
	OpSetStrokeGray: handleSetStrokeGray,
	OpSetFillGray:   handleSetFillGray,
	OpSetStrokeRGB:  handleSetStrokeRGB,
	OpSetFillRGB:    handleSetFillRGB,
	OpSetStrokeCMYK: handleSetStrokeCMYK,
	OpSetFillCMYK:   handleSetFillCMYK,

	// Shading
	OpShading: handleShading,

	// XObjects
	OpXObject: handleXObject,

	// Marked content
	OpMarkedContentPoint:               handleMarkedContentPoint,
	OpMarkedContentPointWithProperties: handleMarkedContentPointWithProperties,
	OpBeginMarkedContent:               handleBeginMarkedContent,
	OpBeginMarkedContentWithProperties: handleBeginMarkedContentWithProperties,
	OpEndMarkedContent:                 handleEndMarkedContent,

	// Type 3 fonts
	OpType3SetWidthOnly:           handleType3d0,
	OpType3SetWidthAndBoundingBox: handleType3d1,

	// Compatibility
	OpBeginCompatibility: handleBeginCompatibility,
	OpEndCompatibility:   handleEndCompatibility,

	// Special operators
	OpRawContent:  handleRawContent,
	OpInlineImage: handleInlineImage,
}
