package cff

import "math"

const eps = 6.0 / 65536

type GlyphMaker struct {
	width      int
	posX, posY float64
	cmds       []cmd
}

func (gm *GlyphMaker) SetWidth(w int) {
	gm.width = w
}

func (gm *GlyphMaker) MoveTo(x, y float64) {
	dx := encode(x - gm.posX)
	dy := encode(y - gm.posY)
	gm.cmds = append(gm.cmds, cmd{
		args: []encodedNumber{dx, dy},
		op:   t2rmoveto,
	})
	gm.posX += dx.val
	gm.posY += dy.val
}

func (gm *GlyphMaker) LineTo(x, y float64) {
	dx := encode(x - gm.posX)
	dy := encode(y - gm.posY)
	gm.cmds = append(gm.cmds, cmd{
		args: []encodedNumber{dx, dy},
		op:   t2rlineto,
	})
	gm.posX += dx.val
	gm.posY += dy.val
}

func (gm *GlyphMaker) CurveTo(xa, ya, xb, yb, xc, yc float64) {
	dxa := encode(xa - gm.posX)
	dya := encode(xa - gm.posY)
	dxb := encode(xb - xa)
	dyb := encode(yb - ya)
	dxc := encode(xc - xb)
	dyc := encode(yc - yb)
	gm.cmds = append(gm.cmds, cmd{
		args: []encodedNumber{dxa, dxb, dya, dyb, dxc, dyc},
		op:   t2rrcurveto,
	})
	gm.posX += dxa.val + dxb.val + dxc.val
	gm.posY += dya.val + dyb.val + dyc.val
}

func (gm *GlyphMaker) encode(defWidth, nomWidth int) []byte {
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
		mov, cmds := cmds[0], cmds[1:]
		if mov.args[0].isZero() {
			code = append(code, mov.args[1].code...)
			code = pushOp(code, t2vmoveto)
		} else if mov.args[1].isZero() {
			code = append(code, mov.args[0].code...)
			code = pushOp(code, t2hmoveto)
		} else {
			code = mov.pushArgs(code)
			code = pushOp(code, t2rmoveto)
		}

		k := 1
		for k < len(cmds) && cmds[k].op != t2rmoveto {
			k++
		}
		path, cmds := cmds[:k], cmds[k:]

		_ = path // TODO(voss): implement
	}

	code = pushOp(code, t2endchar)
	return code
}

type cmd struct {
	args []encodedNumber
	op   t2op
}

func (c cmd) pushArgs(code []byte) []byte {
	for _, a := range c.args {
		code = append(code, a.code...)
	}
	return code
}

type edge struct {
	code []byte
	skip int
}

func edges(cmds []cmd) []edge {
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
		for i := range cmds {
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
				code: pushOp(code, t2rlineto),
				skip: i,
			})
		}

		// {dx dy}+ xb yb xc yc xd yd  rlinecurve
		if numLines < len(cmds) {
			code = cmds[numLines].pushArgs(code)
			edges = append(edges, edge{
				code: pushOp(code, t2rlinecurve),
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
					if !vertical[k] {
						break
					}
					args = append(args, cmds[k].args[0].code...)
				}
				k++
			}
			edges = append(edges, edge{
				code: pushOp(args, t2hlineto),
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
				code: pushOp(args, t2vlineto),
				skip: k,
			})
		}
	} else {
		numCurves := 0
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
			code = cmds[i-1].pushArgs(code)
			edges = append(edges, edge{
				code: pushOp(code, t2rrcurveto),
				skip: i,
			})
		}

		// (dxa dya dxb dyb dxc dyc)+ dxd dyd rcurveline
		if numCurves < len(cmds) {
			code = cmds[numCurves].pushArgs(code)
			edges = append(edges, edge{
				code: pushOp(code, t2rcurveline),
				skip: numCurves + 1,
			})
		}
	}

	return edges
}

type encodedNumber struct {
	val  float64
	code []byte
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

func (enc encodedNumber) isZero() bool {
	return enc.val == 0
}

func pushOp(data []byte, op t2op) []byte {
	if op > 255 {
		return append(data, byte(op>>8), byte(op))
	}
	return append(data, byte(op))
}
