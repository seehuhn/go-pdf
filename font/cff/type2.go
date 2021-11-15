package cff

// Determine the indices of local and global subroutines used by a charstring.
func charStringDependencies(cc []byte) (subr, gsubr []int32) {
	local := make(map[int32]bool)
	global := make(map[int32]bool)
	var stack []int32
	storage := make(map[int32]int32)
	var nStems int

	i := 0
	for i < len(cc) {
		op := type2op(cc[i])
		i++
		if op == 0x0c && i < len(cc)-1 {
			op = op<<8 | type2op(cc[i])
			i++
		}
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
			i += (nStems + 7) / 8
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

type type2op uint16

const (
	t2hstem      type2op = 0x0001
	t2vstem      type2op = 0x0003
	t2vmoveto    type2op = 0x0004
	t2rlineto    type2op = 0x0005
	t2hlineto    type2op = 0x0006
	t2vlineto    type2op = 0x0007
	t2rrcurveto  type2op = 0x0008
	t2callsubr   type2op = 0x000a
	t2return     type2op = 0x000b
	t2endchar    type2op = 0x000e
	t2hstemhm    type2op = 0x0012
	t2hintmask   type2op = 0x0013
	t2cntrmask   type2op = 0x0014
	t2rmoveto    type2op = 0x0015
	t2hmoveto    type2op = 0x0016
	t2vstemhm    type2op = 0x0017
	t2rcurveline type2op = 0x0018
	t2rlinecurve type2op = 0x0019
	t2vvcurveto  type2op = 0x001a
	t2hhcurveto  type2op = 0x001b
	t2shortint   type2op = 0x001c
	t2callgsubr  type2op = 0x001d
	t2vhcurveto  type2op = 0x001e
	t2hvcurveto  type2op = 0x001f

	t2dotsection type2op = 0x0c00
	t2and        type2op = 0x0c03
	t2or         type2op = 0x0c04
	t2not        type2op = 0x0c05
	t2abs        type2op = 0x0c09
	t2add        type2op = 0x0c0a
	t2sub        type2op = 0x0c0b
	t2div        type2op = 0x0c0c
	t2neg        type2op = 0x0c0e
	t2eq         type2op = 0x0c0f
	t2drop       type2op = 0x0c12
	t2put        type2op = 0x0c14
	t2get        type2op = 0x0c15
	t2ifelse     type2op = 0x0c16
	t2random     type2op = 0x0c17
	t2mul        type2op = 0x0c18
	t2sqrt       type2op = 0x0c1a
	t2dup        type2op = 0x0c1b
	t2exch       type2op = 0x0c1c
	t2index      type2op = 0x0c1d
	t2roll       type2op = 0x0c1e
	t2hflex      type2op = 0x0c22
	t2flex       type2op = 0x0c23
	t2hflex1     type2op = 0x0c24
	t2flex1      type2op = 0x0c25
)
