package cff

import "fmt"

// Determine the indices of local and global subroutines used by a charstring.
func charStringDependencies(cc []byte) (subr, gsubr []int32) {
	local := make(map[int32]bool)
	global := make(map[int32]bool)
	var stack []int32
	storage := make(map[int32]int32)
	var nStems int

	for len(cc) > 0 {
		op := t2op(cc[0])
		cc = cc[1:]

		if op >= 32 && op <= 246 {
			stack = append(stack, int32(op)-139)
			continue
		} else if op >= 247 && op <= 250 {
			if len(cc) < 1 {
				break
			}
			stack = append(stack, int32(op)*256+int32(cc[0])+(108-247*256))
			cc = cc[1:]
			continue
		} else if op >= 251 && op <= 254 {
			if len(cc) < 2 {
				break
			}
			stack = append(stack, -int32(op)*256-int32(cc[0])-(108-251*256))
			cc = cc[2:]
			continue
		} else if op == 255 {
			if len(cc) < 4 {
				break
			}
			x := int32(uint32(cc[0])<<24 + uint32(cc[1])<<16 + uint32(cc[2])<<8 + uint32(cc[3]))
			stack = append(stack, x) // TODO(voss): this is a float
			cc = cc[4:]
			continue
		} else if op == 0x0c {
			if len(cc) < 1 {
				break
			}
			op = op<<8 | t2op(cc[0])
			cc = cc[1:]
		}

		fmt.Println(stack, op, cc)

		switch op {
		case t2rmoveto, t2hmoveto, t2vmoveto, t2rlineto, t2hlineto, t2vlineto,
			t2rrcurveto, t2hhcurveto, t2hvcurveto, t2rcurveline, t2rlinecurve,
			t2vhcurveto, t2vvcurveto, t2flex, t2hflex, t2hflex1, t2flex1:
			// all path construction operators clear the stack
			stack = stack[:0]

		case t2dotsection: // deprecated
			stack = stack[:0]

		case t2hstem, t2vstem, t2hstemhm, t2vstemhm:
			nStems += len(stack) / 2
			stack = stack[:0]
		case t2hintmask, t2cntrmask:
			cc = cc[(nStems+7)/8:]
			stack = stack[:0]

		case t2abs:
			k := len(stack) - 1
			if k >= 0 && stack[k] < 0 {
				stack[k] = -stack[k]
			}
		case t2add:
			k := len(stack) - 2
			if k >= 0 {
				stack[k] += stack[k+1]
				stack = stack[:k+1]
			}
		case t2sub:
			k := len(stack) - 2
			if k >= 0 {
				stack[k] -= stack[k+1]
				stack = stack[:k+1]
			}
		case t2div:
			k := len(stack) - 2
			if k >= 0 {
				if stack[k+1] != 0 {
					stack[k] /= stack[k+1]
				}
				stack = stack[:k+1]
			}
		case t2neg:
			k := len(stack) - 1
			if k >= 0 {
				stack[k] = -stack[k]
			}
		case t2random, t2sqrt:
			// not implemented
			stack = append(stack, 0)
		case t2mul:
			k := len(stack) - 2
			if k >= 0 {
				stack[k] *= stack[k+1]
				stack = stack[:k+1]
			}
		case t2drop:
			stack = stack[:len(stack)-1]
		case t2exch:
			k := len(stack) - 2
			if k >= 0 {
				stack[k], stack[k+1] = stack[k+1], stack[k]
			}
		case t2index:
			k := len(stack) - 1
			if k > 0 {
				idx := int(stack[k])
				if idx < 0 || k-idx-1 < 0 {
					stack[k] = stack[k-1]
				} else {
					stack[k] = stack[k-idx-1]
				}
			}
		case t2roll:
			k := len(stack) - 2
			if k >= 0 {
				n := int(stack[k])
				j := int(stack[k+1])
				if n > 0 && n <= k {
					roll(stack[k-n:k], j)
				}
				stack = stack[:k]
			}
		case t2dup:
			k := len(stack) - 1
			if k >= 0 {
				stack = append(stack, stack[k])
			}

		case t2put:
			k := len(stack) - 2
			if k >= 0 {
				storage[stack[k+1]] = stack[k]
			}
			stack = stack[:k]
		case t2get:
			k := len(stack) - 1
			if k >= 0 {
				stack[k] = storage[stack[k]]
			}

		case t2and:
			k := len(stack) - 2
			if k >= 0 {
				var val int32
				if stack[k] != 0 && stack[k+1] != 0 {
					val = 1
				}
				stack = append(stack[:k], val)
			}
		case t2or:
			k := len(stack) - 2
			if k >= 0 {
				var val int32
				if stack[k] != 0 || stack[k+1] != 0 {
					val = 1
				}
				stack = append(stack[:k], val)
			}
		case t2not:
			k := len(stack) - 1
			if k >= 0 {
				var val int32
				if stack[k] == 0 {
					val = 1
				}
				stack[k] = val
			}
		case t2eq:
			k := len(stack) - 2
			if k >= 0 {
				var val int32
				if stack[k] == stack[k+1] {
					val = 1
				}
				stack = append(stack[:k], val)
			}
		case t2ifelse:
			k := len(stack) - 4
			if k >= 0 {
				val := stack[k]
				if stack[k+2] > stack[k+3] {
					val = stack[k+1]
				}
				stack = append(stack[:k], val)
			}

		case t2callsubr:
			k := len(stack) - 1
			if k >= 0 {
				local[stack[k]] = true
				stack = stack[:k]
			}

		case t2callgsubr:
			k := len(stack) - 1
			if k >= 0 {
				global[stack[k]] = true
				stack = stack[:k]
			}

		case t2endchar, t2return:
			break

		default:
		}
	}

	for key, used := range local {
		if used {
			subr = append(subr, key)
		}
	}
	for key, used := range global {
		if used {
			gsubr = append(gsubr, key)
		}
	}
	return
}

func roll(data []int32, j int) {
	n := len(data)
	j = j % n
	if j < 0 {
		j += n
	}

	tmp := make([]int32, j)
	copy(tmp, data[n-j:])
	copy(data[j:], data[:n-j])
	copy(data[:j], tmp)
}

type t2op uint16

func (op t2op) String() string {
	switch op {
	case t2hstem:
		return "t2hstem"
	case t2vstem:
		return "t2vstem"
	case t2vmoveto:
		return "t2vmoveto"
	case t2rlineto:
		return "t2rlineto"
	case t2hlineto:
		return "t2hlineto"
	case t2vlineto:
		return "t2vlineto"
	case t2rrcurveto:
		return "t2rrcurveto"
	case t2callsubr:
		return "t2callsubr"
	case t2return:
		return "t2return"
	case t2endchar:
		return "t2endchar"
	case t2hstemhm:
		return "t2hstemhm"
	case t2hintmask:
		return "t2hintmask"
	case t2cntrmask:
		return "t2cntrmask"
	case t2rmoveto:
		return "t2rmoveto"
	case t2hmoveto:
		return "t2hmoveto"
	case t2vstemhm:
		return "t2vstemhm"
	case t2rcurveline:
		return "t2rcurveline"
	case t2rlinecurve:
		return "t2rlinecurve"
	case t2vvcurveto:
		return "t2vvcurveto"
	case t2hhcurveto:
		return "t2hhcurveto"
	case t2shortint:
		return "t2shortint"
	case t2callgsubr:
		return "t2callgsubr"
	case t2vhcurveto:
		return "t2vhcurveto"
	case t2hvcurveto:
		return "t2hvcurveto"
	case t2dotsection:
		return "t2dotsection"
	case t2and:
		return "t2and"
	case t2or:
		return "t2or"
	case t2not:
		return "t2not"
	case t2abs:
		return "t2abs"
	case t2add:
		return "t2add"
	case t2sub:
		return "t2sub"
	case t2div:
		return "t2div"
	case t2neg:
		return "t2neg"
	case t2eq:
		return "t2eq"
	case t2drop:
		return "t2drop"
	case t2put:
		return "t2put"
	case t2get:
		return "t2get"
	case t2ifelse:
		return "t2ifelse"
	case t2random:
		return "t2random"
	case t2mul:
		return "t2mul"
	case t2sqrt:
		return "t2sqrt"
	case t2dup:
		return "t2dup"
	case t2exch:
		return "t2exch"
	case t2index:
		return "t2index"
	case t2roll:
		return "t2roll"
	case t2hflex:
		return "t2hflex"
	case t2flex:
		return "t2flex"
	case t2hflex1:
		return "t2hflex1"
	case t2flex1:
		return "t2flex1"
	}
	return fmt.Sprintf("t2op(%d)", op)
}

const (
	t2hstem      t2op = 0x0001
	t2vstem      t2op = 0x0003
	t2vmoveto    t2op = 0x0004
	t2rlineto    t2op = 0x0005
	t2hlineto    t2op = 0x0006
	t2vlineto    t2op = 0x0007
	t2rrcurveto  t2op = 0x0008
	t2callsubr   t2op = 0x000a
	t2return     t2op = 0x000b
	t2endchar    t2op = 0x000e
	t2hstemhm    t2op = 0x0012
	t2hintmask   t2op = 0x0013
	t2cntrmask   t2op = 0x0014
	t2rmoveto    t2op = 0x0015
	t2hmoveto    t2op = 0x0016
	t2vstemhm    t2op = 0x0017
	t2rcurveline t2op = 0x0018
	t2rlinecurve t2op = 0x0019
	t2vvcurveto  t2op = 0x001a
	t2hhcurveto  t2op = 0x001b
	t2shortint   t2op = 0x001c
	t2callgsubr  t2op = 0x001d
	t2vhcurveto  t2op = 0x001e
	t2hvcurveto  t2op = 0x001f

	t2dotsection t2op = 0x0c00
	t2and        t2op = 0x0c03
	t2or         t2op = 0x0c04
	t2not        t2op = 0x0c05
	t2abs        t2op = 0x0c09
	t2add        t2op = 0x0c0a
	t2sub        t2op = 0x0c0b
	t2div        t2op = 0x0c0c
	t2neg        t2op = 0x0c0e
	t2eq         t2op = 0x0c0f
	t2drop       t2op = 0x0c12
	t2put        t2op = 0x0c14
	t2get        t2op = 0x0c15
	t2ifelse     t2op = 0x0c16
	t2random     t2op = 0x0c17
	t2mul        t2op = 0x0c18
	t2sqrt       t2op = 0x0c1a
	t2dup        t2op = 0x0c1b
	t2exch       t2op = 0x0c1c
	t2index      t2op = 0x0c1d
	t2roll       t2op = 0x0c1e
	t2hflex      t2op = 0x0c22
	t2flex       t2op = 0x0c23
	t2hflex1     t2op = 0x0c24
	t2flex1      t2op = 0x0c25
)
