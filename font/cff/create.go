package cff

import (
	"container/heap"
	"fmt"
	"math"

	"seehuhn.de/go/pdf/font"
)

// Glyph represents a glyph in a CFF font.
type Glyph struct {
	Name  string
	Width int32
	Cmds  []Command
}

func (g *Glyph) MoveTo(x, y float64) {
	g.Cmds = append(g.Cmds, Command{Op: CmdMoveTo, Args: []float64{x, y}})
}

func (g *Glyph) LineTo(x, y float64) {
	g.Cmds = append(g.Cmds, Command{Op: CmdLineTo, Args: []float64{x, y}})
}

func (g *Glyph) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	g.Cmds = append(g.Cmds, Command{Op: CmdCurveTo, Args: []float64{x1, y1, x2, y2, x3, y3}})
}

// Extent computes the Glyph extent in font design units
func (g *Glyph) Extent() font.Rect {
	var left, right, top, bottom float64
	first := true
cmdLoop:
	for _, cmd := range g.Cmds {
		var x, y float64
		switch cmd.Op {
		case CmdMoveTo, CmdLineTo:
			x = cmd.Args[0]
			y = cmd.Args[1]
		case CmdCurveTo:
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
		LLx: int(math.Floor(left)),
		LLy: int(math.Floor(bottom)),
		URx: int(math.Ceil(right)),
		URy: int(math.Ceil(top)),
	}
}

// Command is a CFF glyph drawing command.
type Command struct {
	Op   CommandType
	Args []float64
}

// enCmd encodes a single command, using relative coordinates for the arguments
// and storing the argument values as EncodedNumbers.
type enCmd struct {
	Op   CommandType
	Args []encodedNumber
}

func (c enCmd) String() string {
	return fmt.Sprint("cmd ", c.Args, c.Op)
}

func encodeArgs(cmds []Command) []enCmd {
	res := make([]enCmd, len(cmds))

	posX := 0.0
	posY := 0.0
	for i, cmd := range cmds {
		switch cmd.Op {
		case CmdMoveTo, CmdLineTo:
			dx := encode(cmd.Args[0] - posX)
			dy := encode(cmd.Args[1] - posY)
			res[i] = enCmd{
				Args: []encodedNumber{dx, dy},
				Op:   cmd.Op,
			}
			posX += dx.Val
			posY += dy.Val
		case CmdCurveTo:
			dxa := encode(cmd.Args[0] - posX)
			dya := encode(cmd.Args[1] - posY)
			dxb := encode(cmd.Args[2] - cmd.Args[0])
			dyb := encode(cmd.Args[3] - cmd.Args[1])
			dxc := encode(cmd.Args[4] - cmd.Args[2])
			dyc := encode(cmd.Args[5] - cmd.Args[3])
			res[i] = enCmd{
				Args: []encodedNumber{dxa, dya, dxb, dyb, dxc, dyc},
				Op:   CmdCurveTo,
			}
			posX += dxa.Val + dxb.Val + dxc.Val
			posY += dya.Val + dyb.Val + dyc.Val
		}
	}
	return res
}

func (c enCmd) appendArgs(code []byte) []byte {
	for _, a := range c.Args {
		code = append(code, a.Code...)
	}
	return code
}

func encodeCommands(cmds []enCmd) [][]byte {
	var res [][]byte

	for len(cmds) > 1 {
		var mov enCmd
		if cmds[0].Op == CmdMoveTo {
			mov = cmds[0]
			cmds = cmds[1:]
		} else {
			mov = enCmd{
				Args: []encodedNumber{encode(0), encode(0)},
				Op:   CmdMoveTo,
			}
		}

		if mov.Args[0].IsZero() {
			res = append(res, mov.Args[1].Code, t2vmoveto.Bytes())
		} else if mov.Args[1].IsZero() {
			res = append(res, mov.Args[0].Code, t2hmoveto.Bytes())
		} else {
			res = append(res, mov.Args[0].Code, mov.Args[1].Code, t2rmoveto.Bytes())
		}

		k := 1
		for k < len(cmds) && cmds[k].Op != CmdMoveTo {
			k++
		}
		path := cmds[:k]
		cmds = cmds[k:]

		res = append(res, encodePath(path)...)
	}

	res = append(res, t2endchar.Bytes())
	return res
}

func encodePath(cmds []enCmd) [][]byte {
	for _, c := range cmds {
		fmt.Println(c)
	}

	n := len(cmds)
	done := make([]bool, n+1)
	best := &priorityQueue{
		dir: make(map[int]int),
	}
	heap.Push(best, &pqEntry{state: 0, code: nil})
	for {
		v := heap.Pop(best).(*pqEntry)
		from := v.state
		if from == n {
			fmt.Println()
			return v.code
		}
		for _, edge := range findEdges(cmds[from:]) {
			fmt.Println(".", from, edge)
			to := from + edge.step
			if done[to] {
				continue
			}
			best.Update(to, v, edge.code)
		}

		done[from] = true
	}
}

type pqEntry struct {
	state int
	code  [][]byte
	cost  int
}

type priorityQueue struct {
	entries []*pqEntry
	dir     map[int]int
}

func (pq *priorityQueue) Len() int {
	return len(pq.entries)
}

func (pq *priorityQueue) Less(i, j int) bool {
	return pq.entries[i].cost < pq.entries[j].cost
}

func (pq *priorityQueue) Swap(i, j int) {
	entries := pq.entries
	entries[i], entries[j] = entries[j], entries[i]
	pq.dir[entries[i].state] = i
	pq.dir[entries[j].state] = j
}

func (pq *priorityQueue) Push(x interface{}) {
	entry := x.(*pqEntry)
	pq.dir[entry.state] = len(pq.entries)
	pq.entries = append(pq.entries, entry)
}

func (pq *priorityQueue) Pop() interface{} {
	n := pq.Len()
	x := pq.entries[n-1]
	pq.entries = pq.entries[0 : n-1]
	delete(pq.dir, x.state)
	return x
}

func (pq *priorityQueue) Update(state int, head *pqEntry, tail []byte) {
	var e *pqEntry

	idx, ok := pq.dir[state]
	if ok {
		e = pq.entries[idx]
	}
	cost := head.cost + len(tail)
	if ok && len(e.code) <= cost {
		return
	}

	code := make([][]byte, len(head.code)+1)
	copy(code, head.code)
	code[len(head.code)] = tail
	if ok {
		e.code = code
		e.cost = cost
		heap.Fix(pq, idx)
	} else {
		e = &pqEntry{state: state, code: code, cost: cost}
		heap.Push(pq, e)
	}
}

const maxStack = 48

func findEdges(cmds []enCmd) []edge {
	// TODO(voss): use at most 48 slots on the argument stack.

	if len(cmds) == 0 {
		return nil
	}

	var edges []edge

	numLines := 0
	for numLines < len(cmds) && cmds[numLines].Op == CmdLineTo {
		numLines++
	}
	if numLines > 0 {
		// candidates:
		//   - (dx dy)+  rlineto
		//   - dx (dy dx)* dy?  hlineto
		//   - dy (dx dy)* dx?  vlineto
		//   - (dx dy)+ xb yb xc yc xd yd  rlinecurve

		horizontal := make([]bool, numLines)
		vertical := make([]bool, numLines)
		for i := 0; i < numLines; i++ {
			horizontal[i] = cmds[i].Args[1].IsZero()
			vertical[i] = cmds[i].Args[0].IsZero()
		}

		// {dx dy}+  rlineto
		var code []byte
		for i := 1; i <= numLines; i++ {
			code = append(code, cmds[i-1].Args[0].Code...)
			code = append(code, cmds[i-1].Args[1].Code...)
			if i < numLines && !horizontal[i] && !vertical[i] {
				continue
			}
			edges = append(edges, edge{
				code: copyOp(code, t2rlineto),
				step: i,
			})
		}

		// {dx dy}+ xb yb xc yc xd yd  rlinecurve
		if numLines < len(cmds) {
			code = cmds[numLines].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rlinecurve),
				step: numLines + 1,
			})
		}

		// dx {dy dx}* dy?  hlineto
		if horizontal[0] {
			args := cmds[0].Args[0].Code
			k := 1
			for k < numLines {
				if k%2 == 1 {
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].Args[1].Code...)
				} else {
					if !horizontal[k] {
						break
					}
					args = append(args, cmds[k].Args[0].Code...)
				}
				k++
			}
			edges = append(edges, edge{
				code: copyOp(args, t2hlineto),
				step: k,
			})
		}

		// dy {dx dy}* dx?  vlineto
		if vertical[0] {
			args := cmds[0].Args[1].Code
			k := 1
			for k < numLines {
				if k%2 == 0 {
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].Args[1].Code...)
				} else {
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].Args[0].Code...)
				}
				k++
			}
			edges = append(edges, edge{
				code: copyOp(args, t2vlineto),
				step: k,
			})
		}
	} else {
		numCurves := 1 // we know that cmds[0] is a curve
		for numCurves < len(cmds) && cmds[numCurves].Op == CmdCurveTo {
			numCurves++
		}

		// candidates:
		//   - (dxa dya dxb dyb dxc dyc)+ rrcurveto
		//   - (dxa dya dxb dyb dxc dyc)+ dxd dyd rcurveline
		//   - dy1? (dxa dxb dyb dxc)+ hhcurveto
		//   - dx1? (dya dxb dyb dyc)+ vvcurveto
		//   - ... hvcurveto
		//   - ... vhcurveto
		//   - ... flex
		//   - ... flex1
		//   - ... hflex
		//   - ... hflex1

		// (dxa dya dxb dyb dxc dyc)+ rrcurveto
		var code []byte
		for i := 1; i <= numCurves; i++ {
			code = cmds[i-1].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rrcurveto),
				step: i,
			})
		}

		// (dxa dya dxb dyb dxc dyc)+ dxd dyd rcurveline
		if numCurves < len(cmds) {
			code = cmds[numCurves].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rcurveline),
				step: numCurves + 1,
			})
		}

		// dy1? (dxa dxb dyb dxc)+ hhcurveto
		code = nil
		for i := 0; i < numCurves; i++ {
			if !cmds[i].Args[5].IsZero() {
				break
			}
			if !cmds[i].Args[1].IsZero() {
				if i > 0 {
					break
				} else {
					code = append(code, cmds[0].Args[1].Code...)
				}
			}
			code = append(code, cmds[i].Args[0].Code...)
			code = append(code, cmds[i].Args[2].Code...)
			code = append(code, cmds[i].Args[3].Code...)
			code = append(code, cmds[i].Args[4].Code...)
			edges = append(edges, edge{
				code: copyOp(code, t2hhcurveto),
				step: i + 1,
			})
		}

		// dx1? (dya dxb dyb dyc)+ vvcurveto
		code = nil
		for i := 0; i < numCurves; i++ {
			if !cmds[i].Args[4].IsZero() {
				break
			}
			if !cmds[i].Args[0].IsZero() {
				if i > 0 {
					break
				} else {
					code = append(code, cmds[0].Args[0].Code...)
				}
			}
			code = append(code, cmds[i].Args[1].Code...)
			code = append(code, cmds[i].Args[2].Code...)
			code = append(code, cmds[i].Args[3].Code...)
			code = append(code, cmds[i].Args[5].Code...)
			edges = append(edges, edge{
				code: copyOp(code, t2vvcurveto),
				step: i + 1,
			})
		}

		// TODO(voss): implement the missing operators
		//   - ... hvcurveto
		//   - ... vhcurveto
		//   - ... flex
		//   - ... flex1
		//   - ... hflex
		//   - ... hflex1
	}

	return edges
}

type edge struct {
	code []byte
	step int
}

func (e edge) String() string {
	return fmt.Sprintf("edge (% x) %+d", e.code, e.step)
}

// CommandType is the type of a CFF glyph drawing command.
type CommandType byte

func (op CommandType) String() string {
	switch op {
	case CmdMoveTo:
		return "moveto"
	case CmdLineTo:
		return "lineto"
	case CmdCurveTo:
		return "curveto"
	default:
		return fmt.Sprintf("CommandType(%d)", op)
	}
}

const (
	// CmdMoveTo closes the previous subpath and starts a new one at the given point.
	CmdMoveTo CommandType = iota + 1

	// CmdLineTo appends a straight line segment from the previous point to the given point.
	CmdLineTo

	// CmdCurveTo appends a Bezier curve segment from the previous point to the given point.
	CmdCurveTo
)

func (c Command) String() string {
	return fmt.Sprint("cmd", c.Args, c.Op)
}

// encodedNumber is a number together with the Type2 charstring encoding of that number.
type encodedNumber struct {
	Val  float64
	Code []byte
}

func (x encodedNumber) String() string {
	return fmt.Sprintf("%g (% x)", x.Val, x.Code)
}

// encode encodes the given number into a CFF encoding.
func encode(x float64) encodedNumber {
	var code []byte
	var val float64

	xInt := math.Round(x)
	if math.Abs(x-xInt) > eps {
		z := int32(math.Round(x * 65536))
		val = float64(z) / 65536
		code = []byte{byte(z >> 24), byte(z >> 16), byte(z >> 8), byte(z)}
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
		return []byte{28, byte(x >> 8), byte(x >> 8)}
	}
}

// IsZero returns true if the encoded number is zero.
func (x encodedNumber) IsZero() bool {
	return x.Val == 0
}

func appendOp(data []byte, op t2op) []byte {
	if op > 255 {
		return append(data, byte(op>>8), byte(op))
	}
	return append(data, byte(op))
}

func copyOp(data []byte, op t2op) []byte {
	if op > 255 {
		res := make([]byte, len(data)+2)
		copy(res, data)
		res[len(data)] = byte(op >> 8)
		res[len(data)+1] = byte(op)
		return res
	}
	res := make([]byte, len(data)+1)
	copy(res, data)
	res[len(data)] = byte(op)
	return res
}

const eps = 6.0 / 65536
