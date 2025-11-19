# ApplyOperator Implementation Design

**Date:** 2025-11-19
**Purpose:** Design for implementing `operator.ApplyOperator` to analyze PDF content stream operators and track graphics state changes.

## Overview

`ApplyOperator` analyzes PDF content stream operators and tracks how they modify graphics state. This supports both reading existing PDF files and implementing a new graphics writer.

## Use Cases

1. **PDF Reader** - Track graphics state while parsing content streams from existing PDFs
2. **Graphics Writer** - New implementation using operator-based approach
3. **Static Analysis** - Understand state dependencies and operator sequences without rendering

## Core Architecture

### State Struct

```go
type State struct {
    Param         graphics.Parameters  // Current parameter values
    In            graphics.StateBits    // External dependencies (accumulates)
    Out           graphics.StateBits    // Modified parameters (saved/restored)
    CurrentObject graphics.ObjectType   // Current graphics object context
    stack         []savedState          // For q/Q operators
}

type savedState struct {
    param *graphics.Parameters
    out   graphics.StateBits
}
```

**Key asymmetry:** `In` accumulates throughout operator sequence and is never restored. `Out` is saved and restored by q/Q. Once a parameter is in `Out`, it is produced by the sequence, so subsequent operators reading it don't add it to `In`.

### Function Signature

```go
func ApplyOperator(state *State, op Operator, res *resource.Resource) error
```

Uses `resource.Resource` (not `pdf.Resources`) to access font instances, ExtGState objects, and other resources directly.

### Handler Architecture

Handler table pattern with one function per operator:

```go
type opHandler func(*State, []pdf.Native, *resource.Resource) error

var handlers = map[pdf.Name]opHandler{
    "m": handleMoveTo,
    "l": handleLineTo,
    // ... 73 operators total
}
```

## File Organization

```
graphics/operator/
  operator.go       # Existing: Operator type, metadata, validation
  apply.go          # ApplyOperator, State, argParser, helpers
  path.go           # Path construction/painting handlers (17)
  state.go          # Graphics state handlers (11)
  text.go           # Text handlers (17)
  color.go          # Color handlers (11)
  misc.go           # XObjects, shading, marked content (17)
  *_test.go         # Corresponding test files
```

## Argument Parsing

Scanner pattern for clean handler code:

```go
type argParser struct {
    args []pdf.Native
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
    case pdf.Number:
        return float64(x)
    default:
        p.err = fmt.Errorf("expected number, got %T", arg)
        return 0
    }
}

func (p *argParser) Check() error {
    if len(p.args) > 0 {
        return errors.New("too many arguments")
    }
    return p.err
}
```

Additional methods: `GetInt()`, `GetName()`, `GetArray()`, `GetDict()`, `GetString()`

## State Tracking Helpers

```go
// Only mark bits as In if not already produced by sequence
func (s *State) markIn(bits graphics.StateBits) {
    s.In |= (bits &^ s.Out)
}

// Mark bits as produced by sequence
func (s *State) markOut(bits graphics.StateBits) {
    s.Out |= bits
}
```

## Handler Patterns

### Pattern 1: Simple Setters

No dependencies, only outputs:

```go
func handleSetLineWidth(s *State, args []pdf.Native, res *resource.Resource) error {
    p := argParser{args: args}
    width := p.GetFloat()
    if err := p.Check(); err != nil {
        return err
    }

    s.Param.LineWidth = width
    s.markOut(graphics.StateLineWidth)
    return nil
}
```

### Pattern 2: Operators with Dependencies

Read state to determine what to mark:

```go
func handleStroke(s *State, args []pdf.Native, res *resource.Resource) error {
    p := argParser{args: args}
    if err := p.Check(); err != nil {
        return err
    }

    if s.CurrentObject != objPath && s.CurrentObject != objClippingPath {
        return errors.New("S: not in path context")
    }

    // Finalize current subpath
    if !s.Param.ThisSubpathClosed {
        s.Param.AllSubpathsClosed = false
    }

    // Mark dependencies
    s.markIn(graphics.StateLineWidth | graphics.StateLineJoin |
             graphics.StateLineDash | graphics.StateStrokeColor)

    // Conditional dependency
    if !s.Param.AllSubpathsClosed || len(s.Param.DashPattern) > 0 {
        s.markIn(graphics.StateLineCap)
    }

    // Reset path
    s.CurrentObject = objPage
    s.Param.AllSubpathsClosed = true

    return nil
}
```

### Pattern 3: Context-aware Dependencies

Check current state values to determine dependencies:

```go
func handleTextShow(s *State, args []pdf.Native, res *resource.Resource) error {
    // ... argument parsing ...

    if s.CurrentObject != objText {
        return errors.New("Tj: not in text object")
    }

    s.markIn(graphics.StateTextFont | graphics.StateTextMatrix)

    // Dependencies based on rendering mode
    mode := s.Param.TextRenderingMode
    if mode == graphics.TextRenderingModeFill ||
       mode == graphics.TextRenderingModeFillStroke ||
       mode == graphics.TextRenderingModeFillClip ||
       mode == graphics.TextRenderingModeFillStrokeClip {
        s.markIn(graphics.StateFillColor)
    }
    if mode == graphics.TextRenderingModeStroke ||
       mode == graphics.TextRenderingModeFillStroke ||
       mode == graphics.TextRenderingModeStrokeClip ||
       mode == graphics.TextRenderingModeFillStrokeClip {
        s.markIn(graphics.StateStrokeColor | graphics.StateLineWidth |
                 graphics.StateLineJoin | graphics.StateLineCap)
    }

    s.markOut(graphics.StateTextMatrix)
    return nil
}
```

## Special Operators

### Raw Content (`%raw%`)

Represents whitespace and comments. Takes a single `pdf.String` argument containing the raw content. Has no effect on state.

The `%raw%` name uses delimiter characters, making collision with real operators impossible (operators must be 1+ printable ASCII chars from 33h-7Ah excluding delimiters).

### Inline Images (`%image%`)

Represents BI...ID...EI inline image sequences. Takes two arguments: `pdf.Dict` (image parameters) and `pdf.String` (image data). Does not modify graphics state.

The parser recognizes BI...ID...EI patterns and emits a single `%image%` operator. The serializer expands `%image%` back to BI...ID...EI syntax when writing.

Inline images are one semantic operation; the three-operator syntax is a parsing artifact.

## Path State Handling

Path state (`CurrentX`, `CurrentY`, `StartX`, `StartY`, `AllSubpathsClosed`, `ThisSubpathClosed`) is not part of the graphics state in PDF terms and is not saved by q/Q.

Path operators validate context via `CurrentObject` but don't use `StateBits` for path-specific state. This matches the PDF specification: "the current path is not part of the graphics state."

## Graphics State Stack

The `q` (push) and `Q` (pop) operators save and restore graphics state:

```go
func handlePushState(s *State, args []pdf.Native, res *resource.Resource) error {
    p := argParser{args: args}
    if err := p.Check(); err != nil {
        return err
    }

    s.stack = append(s.stack, savedState{
        param: s.Param.Clone(),
        out:   s.Out,
    })
    return nil
}

func handlePopState(s *State, args []pdf.Native, res *resource.Resource) error {
    p := argParser{args: args}
    if err := p.Check(); err != nil {
        return err
    }

    if len(s.stack) == 0 {
        return errors.New("Q: no saved state to restore")
    }

    saved := s.stack[len(s.stack)-1]
    s.stack = s.stack[:len(s.stack)-1]

    s.Param = *saved.param
    s.Out = saved.out
    // Note: In is NOT restored, CurrentObject is NOT restored

    return nil
}
```

## Resource Resolution

Operators like `Tf` (set font), `Do` (draw XObject), and `gs` (ExtGState) reference resources by name. The `resource.Resource` parameter provides direct access to resolved instances:

```go
func handleTextSetFont(s *State, args []pdf.Native, res *resource.Resource) error {
    p := argParser{args: args}
    name := p.GetName()
    size := p.GetFloat()
    if err := p.Check(); err != nil {
        return err
    }

    if res == nil || res.Font == nil {
        return errors.New("Tf: no font resources available")
    }

    fontInstance, ok := res.Font[name]
    if !ok {
        return fmt.Errorf("Tf: font %q not found", name)
    }

    s.Param.TextFont = fontInstance
    s.Param.TextFontSize = size
    s.markOut(graphics.StateTextFont)
    return nil
}
```

## Error Handling

Strict validation philosophy:
- Return errors for invalid operator usage (wrong arg count, wrong types, invalid context)
- Caller decides how to handle errors
- Reader can ignore/log errors for permissive parsing
- Writer can propagate errors for strict validation

This keeps `ApplyOperator` simple and testable while allowing different error policies in different contexts.

## Operator Distribution

### path.go (17 operators)
- Path construction: m, l, c, v, y, h, re
- Path painting: S, s, f, F, f*, B, B*, b, b*, n
- Clipping: W, W*

### state.go (11 operators)
- State management: q, Q
- Transformations: cm
- Line parameters: w, J, j, M, d
- Other: ri, i, gs

### text.go (17 operators)
- Text objects: BT, ET
- Text state: Tc, Tw, Tz, TL, Tf, Tr, Ts
- Text positioning: Td, TD, Tm, T*
- Text showing: Tj, TJ, ', "

### color.go (11 operators)
- Color spaces: CS, cs
- Generic color: SC, SCN, sc, scn
- Device colors: G, g, RG, rg, K, k

### misc.go (17 operators)
- Shading: sh
- XObjects: Do
- Marked content: MP, DP, BMC, BDC, EMC
- Type 3 fonts: d0, d1
- Compatibility: BX, EX
- Special: %raw%, %image%

## Testing Strategy

### Unit Tests
Each handler function tested independently:
- Valid argument parsing
- Argument validation (count, types)
- State parameter updates
- Dependency tracking (In/Out bits)
- Context validation

### Integration Tests
Realistic operator sequences:
- Path with mixed open/closed subpaths
- Text rendering mode dependencies
- Graphics state stack (nested q/Q)
- Resource resolution

### Round-trip Tests
For reader/writer use:
- Parse content stream to operators
- Apply operators to track state
- Verify state correctness
- Generate operators from state
- Verify equivalence

## Implementation Notes

1. All 73 operators implemented from the start for completeness
2. Conditional dependencies checked based on actual parameter values for accuracy
3. Path closure tracking (AllSubpathsClosed/ThisSubpathClosed) used to determine LineCap requirements
4. Text rendering mode checked to determine color and line parameter dependencies
5. Graphics state stack saves parameters and Out bits, but In accumulates across the entire sequence
6. Special operators use delimiter-containing names to avoid collision with real operators
