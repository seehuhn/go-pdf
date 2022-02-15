// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
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

package cff

import (
	"container/heap"
	"errors"
	"fmt"
	"math"

	"seehuhn.de/go/pdf"
	"seehuhn.de/go/pdf/font"
)

// Glyph represents a glyph in a CFF font.
type Glyph struct {
	Cmds  []GlyphOp
	HStem []int16
	VStem []int16
	Name  pdf.Name
	Width int32 // TODO(voss): use uint16 here
}

// NewGlyph allocates a new glyph.
func NewGlyph(name pdf.Name, width int32) *Glyph {
	return &Glyph{
		Name:  name,
		Width: width,
	}
}

// MoveTo starts a new sub-path and moves the current point to (x, y).
// The previous sub-path, if any, is closed.
func (g *Glyph) MoveTo(x, y float64) {
	g.Cmds = append(g.Cmds, GlyphOp{Op: OpMoveTo, Args: []float64{x, y}})
}

// LineTo adds a straight line to the current sub-path.
func (g *Glyph) LineTo(x, y float64) {
	g.Cmds = append(g.Cmds, GlyphOp{Op: OpLineTo, Args: []float64{x, y}})
}

// CurveTo adds a cubic Bezier curve to the current sub-path.
func (g *Glyph) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	g.Cmds = append(g.Cmds, GlyphOp{Op: OpCurveTo, Args: []float64{x1, y1, x2, y2, x3, y3}})
}

// Extent computes the Glyph extent in font design units
func (g *Glyph) Extent() font.Rect {
	var left, right, top, bottom float64
	first := true
cmdLoop:
	for _, cmd := range g.Cmds {
		var x, y float64
		switch cmd.Op {
		case OpMoveTo, OpLineTo:
			x = cmd.Args[0]
			y = cmd.Args[1]
		case OpCurveTo:
			x = cmd.Args[4]
			y = cmd.Args[5]
		default:
			continue cmdLoop
		}
		if first || x < left {
			left = x
		}
		if first || x > right {
			right = x
		}
		if first || y < bottom {
			bottom = y
		}
		if first || y > top {
			top = y
		}
		first = false
	}
	return font.Rect{
		LLx: int16(math.Floor(left)),
		LLy: int16(math.Floor(bottom)),
		URx: int16(math.Ceil(right)),
		URy: int16(math.Ceil(top)),
	}
}

func (g *Glyph) getCharString(defaultWidth, nominalWidth int32) ([]byte, error) {
	var header [][]byte
	w := g.Width
	if w != defaultWidth {
		header = append(header, encodeInt(int16(w-nominalWidth)))
	}

	hintMaskUsed := false
	for _, cmd := range g.Cmds {
		if cmd.Op == OpHintMask || cmd.Op == OpCntrMask {
			hintMaskUsed = true
			break
		}
	}

	type stemInfo struct {
		stems []int16
		op    t2op
	}
	allStems := []stemInfo{
		{stems: g.HStem, op: t2hstem},
		{stems: g.VStem, op: t2vstem},
	}
	if hintMaskUsed {
		allStems[0].op = t2hstemhm
		allStems[1].op = t2vstemhm
	}
	extra := len(header)
	for i, pair := range allStems {
		stems := pair.stems
		op := pair.op
		if len(stems)%2 != 0 {
			return nil, errors.New("invalid number of stems")
		}
		for len(stems) > 0 {
			k := (maxStack - extra) / 2
			if k > len(stems)/2 {
				k = len(stems) / 2
			}
			chunk := stems[:2*k]
			stems = stems[2*k:]
			prev := int16(0)
			for _, x := range chunk {
				header = append(header, encodeInt(x-prev))
				prev = x
			}

			canOmitVStem := (i == 1 &&
				len(stems) == 0 &&
				len(g.Cmds) > 0 &&
				(g.Cmds[0].Op == OpHintMask || g.Cmds[0].Op == OpCntrMask))
			if !canOmitVStem {
				header = append(header, op.Bytes())
			}
			extra = 0
		}
	}

	data := encodePaths(g.Cmds)

	k := 0
	for _, b := range header {
		k += len(b)
	}
	for _, b := range data {
		k += len(b)
	}
	code := make([]byte, 0, k)
	for _, b := range header {
		code = append(code, b...)
	}
	for _, b := range data {
		code = append(code, b...)
	}

	return code, nil
}

func encodePaths(commands []GlyphOp) [][]byte {
	var res [][]byte

	cmds := encodeArgs(commands)

	for len(cmds) > 0 {
		switch cmds[0].Op {
		case OpMoveTo:
			mov := cmds[0]
			if mov.Args[0].IsZero() {
				res = append(res, mov.Args[1].Code, t2vmoveto.Bytes())
			} else if mov.Args[1].IsZero() {
				res = append(res, mov.Args[0].Code, t2hmoveto.Bytes())
			} else {
				res = append(res, mov.Args[0].Code, mov.Args[1].Code, t2rmoveto.Bytes())
			}

			cmds = cmds[1:]

		case OpLineTo, OpCurveTo:
			k := 1
			for k < len(cmds) && (cmds[k].Op == OpLineTo || cmds[k].Op == OpCurveTo) {
				k++
			}
			path := cmds[:k]
			cmds = cmds[k:]

			res = append(res, encodeSubPath(path)...)

		case OpHintMask, OpCntrMask:
			op := t2hintmask
			if cmds[0].Op == OpCntrMask {
				op = t2cntrmask
			}
			res = append(res, append(op.Bytes(), cmds[0].Args[0].Code...))

			cmds = cmds[1:]
		default:
			panic("unhandled command")
		}
	}
	res = append(res, t2endchar.Bytes())

	return res
}

func encodeSubPath(cmds []enCmd) [][]byte {
	n := len(cmds)
	done := make([]bool, n+1)
	best := &priorityQueue{
		idx: make(map[int]int),
	}
	heap.Push(best, &bestPath{pos: 0, code: nil})
	for {
		v := heap.Pop(best).(*bestPath)
		from := v.pos
		if from == n {
			return v.code
		}
		for _, edge := range findEdges(cmds[from:]) {
			to := from + edge.skip
			if done[to] {
				continue
			}
			best.Update(to, v, edge.code)
		}
		done[from] = true
	}
}

func encodeArgs(cmds []GlyphOp) []enCmd {
	res := make([]enCmd, len(cmds))

	posX := 0.0
	posY := 0.0
	for i, cmd := range cmds {
		res[i] = enCmd{
			Op: cmd.Op,
		}
		switch cmd.Op {
		case OpMoveTo, OpLineTo:
			dx := encodeNumber(cmd.Args[0] - posX)
			dy := encodeNumber(cmd.Args[1] - posY)
			res[i].Args = []encodedNumber{dx, dy}
			posX += dx.Val
			posY += dy.Val

		case OpCurveTo:
			dxa := encodeNumber(cmd.Args[0] - posX)
			dya := encodeNumber(cmd.Args[1] - posY)
			dxb := encodeNumber(cmd.Args[2] - cmd.Args[0])
			dyb := encodeNumber(cmd.Args[3] - cmd.Args[1])
			dxc := encodeNumber(cmd.Args[4] - cmd.Args[2])
			dyc := encodeNumber(cmd.Args[5] - cmd.Args[3])
			res[i].Args = []encodedNumber{dxa, dya, dxb, dyb, dxc, dyc}
			posX += dxa.Val + dxb.Val + dxc.Val
			posY += dya.Val + dyb.Val + dyc.Val

		case OpHintMask, OpCntrMask:
			k := len(cmd.Args)
			code := make([]byte, k)
			for i, arg := range cmd.Args {
				code[i] = byte(arg)
			}
			res[i].Args = []encodedNumber{{Code: code}}

		default:
			panic("unhandled command")
		}
	}
	return res
}

type bestPath struct {
	code [][]byte
	pos  int
	cost int
}

type priorityQueue struct {
	entries []*bestPath
	idx     map[int]int
}

// Update updates the priority queue entry for the given position.
// `newPos` is a position in the command list, in the range 0, ..., len(cmds).
// The call registers a new edge, from `head` to `newPos`, obtained by
// appending the code `tail` to the code for `head`.
func (pq *priorityQueue) Update(newPos int, head *bestPath, tail [][]byte) {
	code := make([][]byte, len(head.code)+len(tail))
	copy(code, head.code)
	copy(code[len(head.code):], tail)
	cost := head.cost
	for _, blob := range tail {
		cost += len(blob)
	}

	var entry *bestPath
	idx, alreadyExists := pq.idx[newPos]
	if alreadyExists {
		entry = pq.entries[idx]
		if entry.cost > cost {
			entry.code = code
			entry.cost = cost
			heap.Fix(pq, idx)
		}
	} else {
		entry = &bestPath{
			pos:  newPos,
			code: code,
			cost: cost,
		}
		heap.Push(pq, entry)
	}
}

// Len implements heap.Interface.
func (pq *priorityQueue) Len() int {
	return len(pq.entries)
}

// Less implements heap.Interface.
func (pq *priorityQueue) Less(i, j int) bool {
	return pq.entries[i].cost < pq.entries[j].cost
}

// Swap implements heap.Interface.
func (pq *priorityQueue) Swap(i, j int) {
	entries := pq.entries
	entries[i], entries[j] = entries[j], entries[i]
	pq.idx[entries[i].pos] = i
	pq.idx[entries[j].pos] = j
}

// Push implements heap.Interface.
func (pq *priorityQueue) Push(x interface{}) {
	entry := x.(*bestPath)
	pq.idx[entry.pos] = len(pq.entries)
	pq.entries = append(pq.entries, entry)
}

// Pop implements heap.Interface.
func (pq *priorityQueue) Pop() interface{} {
	n := pq.Len()
	x := pq.entries[n-1]
	pq.entries = pq.entries[0 : n-1]
	delete(pq.idx, x.pos)
	return x
}

const maxStack = 48

func findEdges(cmds []enCmd) []edge {
	if len(cmds) == 0 {
		return nil
	}

	var edges []edge

	if cmds[0].Op == OpLineTo {
		// {dx dy}+  rlineto
		var code [][]byte
		pos := 0
		for pos < len(cmds) && cmds[pos].Op == OpLineTo && len(code)+2 <= maxStack {
			code = append(code, cmds[pos].Args[0].Code)
			code = append(code, cmds[pos].Args[1].Code)
			edges = append(edges, edge{
				code: copyOp(code, t2rlineto),
				skip: pos + 1,
			})
			pos++
		}

		// {dx dy}+ xb yb xc yc xd yd  rlinecurve
		if pos < len(cmds) && cmds[pos].Op == OpCurveTo && len(code)+6 <= maxStack {
			edges = append(edges, edge{
				code: copyOp(code, t2rlinecurve, cmds[pos].Args...),
				skip: pos + 1,
			})
		}

		// dx {dy dx}* dy?  hlineto
		// dy {dx dy}* dx?  vlineto
		code = nil
		var aligned []int // +1=horizontal, -1=vertical
		for _, cmd := range cmds {
			if cmd.Op != OpLineTo {
				break
			}
			dir := 0
			if cmd.Args[1].IsZero() {
				dir = 1
			} else if cmd.Args[0].IsZero() {
				dir = -1
			}
			aligned = append(aligned, dir)
		}

		sign := aligned[0]
		if sign != 0 {
			op := []t2op{t2hlineto, t2vlineto}[(1-sign)/2]
			pos = 0
			sign = 1
			for pos < len(aligned) && sign*aligned[pos] > 0 {
				code = append(code, cmds[pos].Args[(1-sign)/2].Code)
				sign = -sign
				pos++
			}

			edges = append(edges, edge{
				code: copyOp(code, op),
				skip: pos,
			})
		}
	} else { // Cmds[0].Op == CmdCurveTo
		// (dxa dya dxb dyb dxc dyc)+ rrcurveto
		pos := 0
		var code [][]byte
		if pos < len(cmds) && cmds[pos].Op == OpCurveTo && len(code)+6 <= maxStack {
			code = cmds[pos].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rrcurveto),
				skip: pos + 1,
			})
			pos++
		}

		// (dxa dya dxb dyb dxc dyc)+ dxd dyd rcurveline
		if pos < len(cmds) && cmds[pos].Op == OpLineTo && len(code)+2 <= maxStack {
			code = cmds[pos].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rcurveline),
				skip: pos + 1,
			})
		}

		// dya? (dxa dxb dyb dxc)+ hhcurveto
		// dxa? (dya dxb dyb dyc)+ vvcurveto
		hhvv := []struct {
			op   t2op
			offs int
		}{
			{t2hhcurveto, 1},
			{t2vvcurveto, 0},
		}
		for _, hv := range hhvv {
			code = nil
			pos = 0
			// 0=dxa 1=dya   2=dxb 3=dyb   4=dxc 5=dyc
			for pos < len(cmds) && cmds[pos].Op == OpCurveTo && len(code)+4 <= maxStack {
				if !cmds[pos].Args[4+hv.offs].IsZero() {
					break
				}
				if !cmds[pos].Args[0+hv.offs].IsZero() {
					if pos == 0 && len(code)+5 <= maxStack {
						code = append(code, cmds[0].Args[0+hv.offs].Code)
					} else {
						break
					}
				}
				code = append(code, cmds[pos].Args[1-hv.offs].Code)
				code = append(code, cmds[pos].Args[2].Code)
				code = append(code, cmds[pos].Args[3].Code)
				code = append(code, cmds[pos].Args[5-hv.offs].Code)
				edges = append(edges, edge{
					code: copyOp(code, hv.op),
					skip: pos + 1,
				})
				pos++
			}
		}

		// dx1 dx2 dy2 dy3 (dya dxb dyb dxc  dxd dxe dye dyf)* dxf?  hvcurveto
		// ... vhcurveto
		for offs, op := range []t2op{t2hvcurveto, t2vhcurveto} {
			code = nil

			origOffs := offs

			// Args: 0=dxa 1=dya   2=dxb 3=dyb   4=dxc 5=dyc
			pos = 0
			for pos < len(cmds) && cmds[pos].Op == OpCurveTo {
				if !cmds[pos].Args[1-offs].IsZero() {
					break
				}
				lastIsAligned := cmds[pos].Args[4+offs].IsZero()
				if offs != origOffs && !lastIsAligned {
					break
				}

				if len(code)+4 > maxStack || !lastIsAligned && len(code)+5 > maxStack {
					break
				}
				code = append(code, cmds[pos].Args[offs].Code)
				code = append(code, cmds[pos].Args[2].Code)
				code = append(code, cmds[pos].Args[3].Code)
				code = append(code, cmds[pos].Args[5-offs].Code)
				if !lastIsAligned {
					code = append(code, cmds[pos].Args[4+offs].Code)
				}
				pos++

				offs = 1 - offs

				if offs == origOffs {
					continue
				}

				edges = append(edges, edge{
					code: copyOp(code, op),
					skip: pos,
				})
				if !lastIsAligned {
					break
				}
			}
		}

		if len(cmds) >= 2 &&
			cmds[0].Op == OpCurveTo && cmds[1].Op == OpCurveTo &&
			cmds[0].Args[5].IsZero() && cmds[1].Args[1].IsZero() {
			// Args: 0=dxa 1=dya   2=dxb 3=dyb   4=dxc 5=dyc

			code = nil

			dy := cmds[0].Args[3].Val + cmds[1].Args[3].Val
			if cmds[0].Args[1].IsZero() && cmds[1].Args[5].IsZero() &&
				math.Abs(dy) < 0.5/65536 {
				// dx1  dx2 dy2  dx3  dx4  dx5  dx6  hflex
				code = append(code, cmds[0].Args[0].Code)
				code = append(code, cmds[0].Args[2].Code)
				code = append(code, cmds[0].Args[3].Code)
				code = append(code, cmds[0].Args[4].Code)
				code = append(code, cmds[1].Args[0].Code)
				code = append(code, cmds[1].Args[2].Code)
				code = append(code, cmds[1].Args[4].Code)
				code = append(code, t2hflex.Bytes())
				edges = append(edges, edge{
					code: code,
					skip: 2,
				})
			} else if math.Abs(dy+cmds[0].Args[1].Val+cmds[1].Args[5].Val) < 0.5/65536 {
				// dx1 dy1 dx2 dy2 dx3 dx4 dx5 dy5 dx6  hflex1
				code = append(code, cmds[0].Args[0].Code)
				code = append(code, cmds[0].Args[1].Code)
				code = append(code, cmds[0].Args[2].Code)
				code = append(code, cmds[0].Args[3].Code)
				code = append(code, cmds[0].Args[4].Code)
				code = append(code, cmds[1].Args[0].Code)
				code = append(code, cmds[1].Args[2].Code)
				code = append(code, cmds[1].Args[3].Code)
				code = append(code, cmds[1].Args[4].Code)
				code = append(code, t2hflex1.Bytes())
				edges = append(edges, edge{
					code: code,
					skip: 2,
				})
			}

			// We don't generate t2flex and t2flex1 commands.
			// dx1 dy1 dx2 dy2 dx3 dy3 dx4 dy4 dx5 dy5 dx6 dy6 fd  flex
			// dx1 dy1 dx2 dy2 dx3 dy3 dx4 dy4 dx5 dy5 d6  flex1
		}
	}

	return edges
}

type edge struct {
	code [][]byte
	skip int
}

func (e edge) String() string {
	return fmt.Sprintf("edge % x %+d", e.code, e.skip)
}

// GlyphOp is a CFF glyph drawing command.
type GlyphOp struct {
	Op   GlyphOpType
	Args []float64 // TODO(voss): use 16.16 fixed point?
}

// GlyphOpType is the type of a CFF glyph drawing command.
type GlyphOpType byte

func (op GlyphOpType) String() string {
	switch op {
	case OpMoveTo:
		return "moveto"
	case OpLineTo:
		return "lineto"
	case OpCurveTo:
		return "curveto"
	case OpHintMask:
		return "hintmask"
	case OpCntrMask:
		return "cntrmask"
	default:
		return fmt.Sprintf("CommandType(%d)", op)
	}
}

const (
	// OpMoveTo closes the previous subpath and starts a new one at the given point.
	OpMoveTo GlyphOpType = iota + 1

	// OpLineTo appends a straight line segment from the previous point to the given point.
	OpLineTo

	// OpCurveTo appends a Bezier curve segment from the previous point to the given point.
	OpCurveTo

	// OpHintMask adds a CFF hintmask command.
	OpHintMask

	// OpCntrMask adds a CFF cntrmask command.
	OpCntrMask
)

func (c GlyphOp) String() string {
	return fmt.Sprint("cmd", c.Args, c.Op)
}

// enCmd encodes a single command, using relative coordinates for the arguments
// and storing the argument values as EncodedNumbers.
type enCmd struct {
	Op   GlyphOpType
	Args []encodedNumber
}

func (c enCmd) String() string {
	return fmt.Sprint("cmd ", c.Args, c.Op)
}

func (c enCmd) appendArgs(code [][]byte) [][]byte {
	for _, a := range c.Args {
		code = append(code, a.Code)
	}
	return code
}

// encodedNumber is a number together with the Type2 charstring encoding of that number.
type encodedNumber struct {
	Val  float64
	Code []byte
}

func (x encodedNumber) String() string {
	return fmt.Sprintf("%g (% x)", x.Val, x.Code)
}

// encodeNumber encodes the given number into a CFF encoding.
func encodeNumber(x float64) encodedNumber {
	var code []byte
	var val float64

	xInt := math.Round(x)
	if math.Abs(x-xInt) > eps {
		z := int32(math.Round(x * 65536))
		val = float64(z) / 65536
		code = []byte{255, byte(z >> 24), byte(z >> 16), byte(z >> 8), byte(z)}
	} else {
		// encode as an integer
		z := int16(xInt)
		code = encodeInt(z)
		val = float64(z)
	}
	return encodedNumber{
		Val:  val,
		Code: code,
	}
}

func encodeInt(x int16) []byte {
	switch {
	case x >= -107 && x <= 107:
		return []byte{byte(x + 139)}
	case x > 107 && x <= 1131:
		x -= 108
		b1 := byte(x)
		x >>= 8
		b0 := byte(x + 247)
		return []byte{b0, b1}
	case x < -107 && x >= -1131:
		x = -108 - x
		b1 := byte(x)
		x >>= 8
		b0 := byte(x + 251)
		return []byte{b0, b1}
	default:
		return []byte{28, byte(x >> 8), byte(x)}
	}
}

// IsZero returns true if the encoded number is zero.
func (x encodedNumber) IsZero() bool {
	return x.Val == 0
}

func copyOp(data [][]byte, op t2op, args ...encodedNumber) [][]byte {
	res := make([][]byte, len(data)+len(args)+1)
	pos := copy(res, data)
	for _, arg := range args {
		res[pos] = arg.Code
		pos++
	}
	if op > 255 {
		res[pos] = []byte{byte(op >> 8), byte(op)}
	} else {
		res[pos] = []byte{byte(op)}
	}
	return res
}

const eps = 6.0 / 65536