package cff

import (
	"container/heap"
	"fmt"
	"math"
)

const eps = 6.0 / 65536

type glyphMaker struct {
	width      int32
	posX, posY float64
	cmds       []cmd
}

func (gm *glyphMaker) SetWidth(w int32) {
	gm.width = w
}

func (gm *glyphMaker) MoveTo(x, y float64) {
	dx := encode(x - gm.posX)
	dy := encode(y - gm.posY)
	gm.cmds = append(gm.cmds, cmd{
		args: []encodedNumber{dx, dy},
		op:   t2rmoveto,
	})
	gm.posX += dx.val
	gm.posY += dy.val
}

func (gm *glyphMaker) LineTo(x, y float64) {
	dx := encode(x - gm.posX)
	dy := encode(y - gm.posY)
	gm.cmds = append(gm.cmds, cmd{
		args: []encodedNumber{dx, dy},
		op:   t2rlineto,
	})
	gm.posX += dx.val
	gm.posY += dy.val
}

func (gm *glyphMaker) CurveTo(xa, ya, xb, yb, xc, yc float64) {
	dxa := encode(xa - gm.posX)
	dya := encode(ya - gm.posY)
	dxb := encode(xb - xa)
	dyb := encode(yb - ya)
	dxc := encode(xc - xb)
	dyc := encode(yc - yb)
	gm.cmds = append(gm.cmds, cmd{
		args: []encodedNumber{dxa, dya, dxb, dyb, dxc, dyc},
		op:   t2rrcurveto,
	})
	gm.posX += dxa.val + dxb.val + dxc.val
	gm.posY += dya.val + dyb.val + dyc.val
}

func (gm *glyphMaker) encode(defWidth, nomWidth int32) []byte {
	var code []byte
	if gm.width != defWidth {
		w := encode(float64(gm.width - nomWidth))
		code = append(code, w.code...)
	}

	cmds := gm.cmds
	if len(cmds) > 0 && cmds[0].op != t2rmoveto {
		tmp := make([]cmd, len(cmds)+1)
		tmp[0] = cmd{
			args: []encodedNumber{encode(0), encode(0)},
			op:   t2rmoveto,
		}
		copy(tmp[1:], cmds)
		cmds = tmp
	}

	for len(cmds) > 1 {
		mov := cmds[0]
		fmt.Println(mov)
		cmds = cmds[1:]
		if mov.args[0].isZero() {
			code = append(code, mov.args[1].code...)
			code = appendOp(code, t2vmoveto)
		} else if mov.args[1].isZero() {
			code = append(code, mov.args[0].code...)
			code = appendOp(code, t2hmoveto)
		} else {
			code = mov.appendArgs(code)
			code = appendOp(code, t2rmoveto)
		}

		k := 1
		for k < len(cmds) && cmds[k].op != t2rmoveto {
			k++
		}
		path := cmds[:k]
		cmds = cmds[k:]

		code = append(code, getCode(path)...)
		fmt.Println()
	}

	code = appendOp(code, t2endchar)
	return code
}

type pqEntry struct {
	state int
	code  []byte
}

type priorityQueue struct {
	entries []*pqEntry
	dir     map[int]int
}

func (pq *priorityQueue) Len() int {
	return len(pq.entries)
}

func (pq *priorityQueue) Less(i, j int) bool {
	return len(pq.entries[i].code) < len(pq.entries[j].code)
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

func (pq *priorityQueue) Update(state int, head, tail []byte) {
	var e *pqEntry

	idx, ok := pq.dir[state]
	if ok {
		e = pq.entries[idx]
	}
	cost := len(head) + len(tail)
	if ok && len(e.code) <= cost {
		return
	}

	code := make([]byte, cost)
	copy(code, head)
	copy(code[len(head):], tail)
	if ok {
		e.code = code
		heap.Fix(pq, idx)
	} else {
		e = &pqEntry{state: state, code: code}
		heap.Push(pq, e)
	}
}

func getCode(cmds []cmd) []byte {
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
			return v.code
		}
		for _, edge := range findEdges(cmds[from:]) {
			fmt.Println(".", from, edge)
			to := from + edge.skip
			if done[to] {
				continue
			}
			best.Update(to, v.code, edge.code)
		}

		done[from] = true
	}
}

func findEdges(cmds []cmd) []edge {
	if len(cmds) == 0 {
		return nil
	}

	var edges []edge

	numLines := 0
	for numLines < len(cmds) && cmds[numLines].op == t2rlineto {
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
			horizontal[i] = cmds[i].args[1].isZero()
			vertical[i] = cmds[i].args[0].isZero()
		}

		// {dx dy}+  rlineto
		var code []byte
		for i := 1; i <= numLines; i++ {
			code = append(code, cmds[i-1].args[0].code...)
			code = append(code, cmds[i-1].args[1].code...)
			if i < numLines && !horizontal[i] && !vertical[i] {
				continue
			}
			edges = append(edges, edge{
				code: copyOp(code, t2rlineto),
				skip: i,
			})
		}

		// {dx dy}+ xb yb xc yc xd yd  rlinecurve
		if numLines < len(cmds) {
			code = cmds[numLines].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rlinecurve),
				skip: numLines + 1,
			})
		}

		// dx {dy dx}* dy?  hlineto
		if horizontal[0] {
			args := cmds[0].args[0].code
			k := 1
			for k < numLines {
				if k%2 == 1 {
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].args[1].code...)
				} else {
					if !horizontal[k] {
						break
					}
					args = append(args, cmds[k].args[0].code...)
				}
				k++
			}
			edges = append(edges, edge{
				code: copyOp(args, t2hlineto),
				skip: k,
			})
		}

		// dy {dx dy}* dx?  vlineto
		if vertical[0] {
			args := cmds[0].args[1].code
			k := 1
			for k < numLines {
				if k%2 == 0 {
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].args[1].code...)
				} else {
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].args[0].code...)
				}
				k++
			}
			edges = append(edges, edge{
				code: copyOp(args, t2vlineto),
				skip: k,
			})
		}
	} else {
		numCurves := 1 // we know that cmds[0] is a curve
		for numCurves < len(cmds) && cmds[numCurves].op == t2rrcurveto {
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
				skip: i,
			})
		}

		// (dxa dya dxb dyb dxc dyc)+ dxd dyd rcurveline
		if numCurves < len(cmds) {
			code = cmds[numCurves].appendArgs(code)
			edges = append(edges, edge{
				code: copyOp(code, t2rcurveline),
				skip: numCurves + 1,
			})
		}

		// dy1? (dxa dxb dyb dxc)+ hhcurveto
		code = nil
		for i := 0; i < numCurves; i++ {
			if !cmds[i].args[5].isZero() {
				break
			}
			if !cmds[i].args[1].isZero() {
				if i > 0 {
					break
				} else {
					code = append(code, cmds[0].args[1].code...)
				}
			}
			code = append(code, cmds[i].args[0].code...)
			code = append(code, cmds[i].args[2].code...)
			code = append(code, cmds[i].args[3].code...)
			code = append(code, cmds[i].args[4].code...)
			edges = append(edges, edge{
				code: copyOp(code, t2hhcurveto),
				skip: i + 1,
			})
		}

		// dx1? (dya dxb dyb dyc)+ vvcurveto
		code = nil
		for i := 0; i < numCurves; i++ {
			if !cmds[i].args[4].isZero() {
				break
			}
			if !cmds[i].args[0].isZero() {
				if i > 0 {
					break
				} else {
					code = append(code, cmds[0].args[0].code...)
				}
			}
			code = append(code, cmds[i].args[1].code...)
			code = append(code, cmds[i].args[2].code...)
			code = append(code, cmds[i].args[3].code...)
			code = append(code, cmds[i].args[5].code...)
			edges = append(edges, edge{
				code: copyOp(code, t2vvcurveto),
				skip: i + 1,
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
	skip int
}

func (e edge) String() string {
	return fmt.Sprintf("edge (% x) %+d", e.code, e.skip)
}

type cmd struct {
	args []encodedNumber
	op   t2op
}

func (c cmd) String() string {
	return fmt.Sprint("cmd", c.args, c.op)
}

func (c cmd) appendArgs(code []byte) []byte {
	for _, a := range c.args {
		code = append(code, a.code...)
	}
	return code
}

type encodedNumber struct {
	val  float64
	code []byte
}

func (x encodedNumber) String() string {
	return fmt.Sprintf("%g (% x)", x.val, x.code)
}

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
		val = float64(z)
		switch {
		case z >= -107 && z <= 107:
			code = []byte{byte(z + 139)}
		case z > 107 && z <= 1131:
			z -= 108
			b1 := byte(z)
			z >>= 8
			b0 := byte(z + 247)
			code = []byte{b0, b1}
		case z < -107 && z >= -1131:
			z = -108 - z
			b1 := byte(z)
			z >>= 8
			b0 := byte(z + 251)
			code = []byte{b0, b1}
		default:
			code = []byte{28, byte(z >> 8), byte(z >> 8)}
		}
	}
	return encodedNumber{
		val:  val,
		code: code,
	}
}

func (x encodedNumber) isZero() bool {
	return x.val == 0
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
