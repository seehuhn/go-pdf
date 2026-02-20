// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2026  Jochen Voss <voss@seehuhn.de>
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

package function

import (
	"errors"
	"fmt"
	"math"
)

// opCode identifies a bytecode instruction for the Type 4 VM.
type opCode uint8

const (
	opPushInt opCode = iota
	opPushReal
	opPushTrue
	opPushFalse

	// arithmetic operators
	opAbs
	opAdd
	opAtan
	opCeiling
	opCos
	opCvi
	opCvr
	opDiv
	opExp
	opFloor
	opIdiv
	opLn
	opLog
	opMod
	opMul
	opNeg
	opRound
	opSin
	opSqrt
	opSub
	opTruncate

	// relational, boolean, and bitwise operators
	opAnd
	opBitshift
	opEq
	opGe
	opGt
	opLe
	opLt
	opNe
	opNot
	opOr
	opXor

	// stack operators
	opCopy
	opDup
	opExch
	opIndex
	opPop
	opRoll

	// control flow
	opJumpIfFalse
	opJump
)

// instruction is a single bytecode instruction.
type instruction struct {
	op   opCode
	ival int
	fval float64
}

// valueTag identifies the type of a stack value.
type valueTag uint8

const (
	tagInt  valueTag = iota
	tagReal          // float64
	tagBool          // stored as ival: 0=false, 1=true
)

// value is a tagged union representing a stack element.
type value struct {
	tag  valueTag
	ival int
	fval float64
}

func intVal(n int) value      { return value{tag: tagInt, ival: n} }
func realVal(f float64) value { return value{tag: tagReal, fval: f} }
func boolVal(b bool) value {
	v := value{tag: tagBool}
	if b {
		v.ival = 1
	}
	return v
}

// asFloat converts int or real values to float64.
func (v value) asFloat() float64 {
	if v.tag == tagInt {
		return float64(v.ival)
	}
	return v.fval
}

// execute runs compiled bytecode on the given stack.
func execute(code []instruction, stack []value) ([]value, error) {
	pc := 0
	for pc < len(code) {
		inst := code[pc]
		pc++

		switch inst.op {
		case opPushInt:
			stack = append(stack, intVal(inst.ival))
		case opPushReal:
			stack = append(stack, realVal(inst.fval))
		case opPushTrue:
			stack = append(stack, boolVal(true))
		case opPushFalse:
			stack = append(stack, boolVal(false))

		case opAbs:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				if v.ival == math.MinInt {
					*v = realVal(-float64(v.ival))
				} else if v.ival < 0 {
					v.ival = -v.ival
				}
			case tagReal:
				v.fval = math.Abs(v.fval)
			default:
				return nil, errTypeMismatch
			}

		case opAdd:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-1]
			r, err := vmAdd(a, b)
			if err != nil {
				return nil, err
			}
			stack[len(stack)-1] = r

		case opSub:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-1]
			r, err := vmSub(a, b)
			if err != nil {
				return nil, err
			}
			stack[len(stack)-1] = r

		case opMul:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-1]
			r, err := vmMul(a, b)
			if err != nil {
				return nil, err
			}
			stack[len(stack)-1] = r

		case opDiv:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if numTag(a.tag) && numTag(b.tag) {
				fv := b.asFloat()
				if fv == 0 {
					return nil, errDivByZero
				}
				stack = stack[:len(stack)-1]
				stack[len(stack)-1] = realVal(a.asFloat() / fv)
			} else {
				return nil, errTypeMismatch
			}

		case opIdiv:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if a.tag != tagInt || b.tag != tagInt {
				return nil, errTypeMismatch
			}
			if b.ival == 0 {
				return nil, errDivByZero
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = intVal(a.ival / b.ival)

		case opMod:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if a.tag != tagInt || b.tag != tagInt {
				return nil, errTypeMismatch
			}
			if b.ival == 0 {
				return nil, errDivByZero
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = intVal(a.ival % b.ival)

		case opNeg:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				if v.ival == math.MinInt {
					*v = realVal(-float64(v.ival))
				} else {
					v.ival = -v.ival
				}
			case tagReal:
				v.fval = -v.fval
			default:
				return nil, errTypeMismatch
			}

		case opCeiling:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				// no-op
			case tagReal:
				v.fval = math.Ceil(v.fval)
			default:
				return nil, errTypeMismatch
			}

		case opFloor:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				// no-op
			case tagReal:
				v.fval = math.Floor(v.fval)
			default:
				return nil, errTypeMismatch
			}

		case opRound:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				// no-op
			case tagReal:
				val := v.fval
				if val >= 0 {
					v.fval = math.Floor(val + 0.5)
				} else {
					floor := math.Floor(val)
					ceil := math.Ceil(val)
					if math.Abs(val-floor) == math.Abs(val-ceil) {
						v.fval = ceil
					} else {
						v.fval = math.Round(val)
					}
				}
			default:
				return nil, errTypeMismatch
			}

		case opTruncate:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				// no-op
			case tagReal:
				v.fval = math.Trunc(v.fval)
			default:
				return nil, errTypeMismatch
			}

		case opSqrt:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			if !numTag(v.tag) {
				return nil, errTypeMismatch
			}
			f := v.asFloat()
			if f < 0 {
				return nil, errors.New("sqrt of negative number")
			}
			*v = realVal(math.Sqrt(f))

		case opExp:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			exp := stack[len(stack)-1]
			base := stack[len(stack)-2]
			if !numTag(base.tag) || !numTag(exp.tag) {
				return nil, errTypeMismatch
			}
			r := math.Pow(base.asFloat(), exp.asFloat())
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = realVal(r)

		case opLn:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			if !numTag(v.tag) {
				return nil, errTypeMismatch
			}
			f := v.asFloat()
			if f <= 0 {
				return nil, errors.New("ln of non-positive number")
			}
			*v = realVal(math.Log(f))

		case opLog:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			if !numTag(v.tag) {
				return nil, errTypeMismatch
			}
			f := v.asFloat()
			if f <= 0 {
				return nil, errors.New("log of non-positive number")
			}
			*v = realVal(math.Log10(f))

		case opSin:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			if !numTag(v.tag) {
				return nil, errTypeMismatch
			}
			rad := v.asFloat() * math.Pi / 180
			*v = realVal(math.Sin(rad))

		case opCos:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			if !numTag(v.tag) {
				return nil, errTypeMismatch
			}
			rad := v.asFloat() * math.Pi / 180
			*v = realVal(math.Cos(rad))

		case opAtan:
			// num den atan → angle (atan2 semantics, result in degrees)
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			den := stack[len(stack)-1]
			num := stack[len(stack)-2]
			if !numTag(num.tag) || !numTag(den.tag) {
				return nil, errTypeMismatch
			}
			nf := num.asFloat()
			df := den.asFloat()
			if nf == 0 && df == 0 {
				return nil, errors.New("atan: both arguments zero")
			}
			deg := math.Atan2(nf, df) * 180 / math.Pi
			if deg < 0 {
				deg += 360
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = realVal(deg)

		case opCvi:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				// no-op
			case tagReal:
				t := math.Trunc(v.fval)
				if t > math.MaxInt64 || t < math.MinInt64 {
					return nil, errors.New("cvi: value out of integer range")
				}
				*v = intVal(int(t))
			default:
				return nil, errTypeMismatch
			}

		case opCvr:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagInt:
				*v = realVal(float64(v.ival))
			case tagReal:
				// no-op
			default:
				return nil, errTypeMismatch
			}

		case opEq:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = boolVal(vmEqual(a, b))

		case opNe:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = boolVal(!vmEqual(a, b))

		case opGe:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if !numTag(a.tag) || !numTag(b.tag) {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = boolVal(a.asFloat() >= b.asFloat())

		case opGt:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if !numTag(a.tag) || !numTag(b.tag) {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = boolVal(a.asFloat() > b.asFloat())

		case opLe:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if !numTag(a.tag) || !numTag(b.tag) {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = boolVal(a.asFloat() <= b.asFloat())

		case opLt:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			if !numTag(a.tag) || !numTag(b.tag) {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = boolVal(a.asFloat() < b.asFloat())

		case opAnd:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			r, err := vmBoolOrBitwise(a, b, func(x, y bool) bool { return x && y }, func(x, y int) int { return x & y })
			if err != nil {
				return nil, err
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = r

		case opOr:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			r, err := vmBoolOrBitwise(a, b, func(x, y bool) bool { return x || y }, func(x, y int) int { return x | y })
			if err != nil {
				return nil, err
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = r

		case opXor:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			b := stack[len(stack)-1]
			a := stack[len(stack)-2]
			r, err := vmBoolOrBitwise(a, b, func(x, y bool) bool { return x != y }, func(x, y int) int { return x ^ y })
			if err != nil {
				return nil, err
			}
			stack = stack[:len(stack)-1]
			stack[len(stack)-1] = r

		case opNot:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			v := &stack[len(stack)-1]
			switch v.tag {
			case tagBool:
				v.ival ^= 1
			case tagInt:
				v.ival = ^v.ival
			default:
				return nil, errTypeMismatch
			}

		case opBitshift:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			shift := stack[len(stack)-1]
			val := stack[len(stack)-2]
			if val.tag != tagInt || shift.tag != tagInt {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			if shift.ival >= 0 {
				stack[len(stack)-1] = intVal(val.ival << uint(shift.ival))
			} else {
				stack[len(stack)-1] = intVal(val.ival >> uint(-shift.ival))
			}

		case opDup:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			stack = append(stack, stack[len(stack)-1])

		case opExch:
			n := len(stack)
			if n < 2 {
				return nil, errStackUnderflow
			}
			stack[n-1], stack[n-2] = stack[n-2], stack[n-1]

		case opPop:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			stack = stack[:len(stack)-1]

		case opIndex:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			idx := stack[len(stack)-1]
			if idx.tag != tagInt {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			i := idx.ival
			if i < 0 || i >= len(stack) {
				return nil, fmt.Errorf("index %d out of range (stack depth %d)", i, len(stack))
			}
			stack = append(stack, stack[len(stack)-1-i])

		case opCopy:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			top := stack[len(stack)-1]
			if top.tag != tagInt {
				return nil, errTypeMismatch
			}
			n := top.ival
			stack = stack[:len(stack)-1]
			if n < 0 || n > len(stack) {
				return nil, fmt.Errorf("copy count %d out of range (stack depth %d)", n, len(stack))
			}
			stack = append(stack, stack[len(stack)-n:]...)

		case opRoll:
			if len(stack) < 2 {
				return nil, errStackUnderflow
			}
			jv := stack[len(stack)-1]
			nv := stack[len(stack)-2]
			if nv.tag != tagInt || jv.tag != tagInt {
				return nil, errTypeMismatch
			}
			n := nv.ival
			j := jv.ival
			stack = stack[:len(stack)-2]
			if n < 0 || n > len(stack) {
				return nil, fmt.Errorf("roll count %d out of range (stack depth %d)", n, len(stack))
			}
			if n == 0 {
				break
			}
			j %= n
			if j < 0 {
				j += n
			}
			if j == 0 {
				break
			}
			data := stack[len(stack)-n:]
			tmp := make([]value, j)
			copy(tmp, data[n-j:])
			copy(data[j:], data[:n-j])
			copy(data, tmp)

		case opJumpIfFalse:
			if len(stack) < 1 {
				return nil, errStackUnderflow
			}
			cond := stack[len(stack)-1]
			if cond.tag != tagBool {
				return nil, errTypeMismatch
			}
			stack = stack[:len(stack)-1]
			if cond.ival == 0 {
				pc += inst.ival
			}

		case opJump:
			pc += inst.ival

		default:
			return nil, fmt.Errorf("unknown opcode %d", inst.op)
		}

		if len(stack) > maxStackDepth {
			return nil, errStackOverflow
		}
	}

	return stack, nil
}

// maxStackDepth is the maximum operand stack depth,
// matching the go-postscript interpreter limit.
const maxStackDepth = 500

func numTag(t valueTag) bool {
	return t == tagInt || t == tagReal
}

// vmAdd implements type-preserving addition with integer overflow promotion.
func vmAdd(a, b value) (value, error) {
	if !numTag(a.tag) || !numTag(b.tag) {
		return value{}, errTypeMismatch
	}
	if a.tag == tagReal || b.tag == tagReal {
		return realVal(a.asFloat() + b.asFloat()), nil
	}
	c := a.ival + b.ival
	if (a.ival < 0 && b.ival < 0 && c >= 0) || (a.ival > 0 && b.ival > 0 && c <= 0) {
		return realVal(float64(a.ival) + float64(b.ival)), nil
	}
	return intVal(c), nil
}

// vmSub implements type-preserving subtraction with integer overflow promotion.
func vmSub(a, b value) (value, error) {
	if !numTag(a.tag) || !numTag(b.tag) {
		return value{}, errTypeMismatch
	}
	if a.tag == tagReal || b.tag == tagReal {
		return realVal(a.asFloat() - b.asFloat()), nil
	}
	c := a.ival - b.ival
	if (a.ival < 0 && b.ival > 0 && c >= 0) || (a.ival > 0 && b.ival < 0 && c <= 0) {
		return realVal(float64(a.ival) - float64(b.ival)), nil
	}
	return intVal(c), nil
}

// vmMul implements type-preserving multiplication with integer overflow promotion.
func vmMul(a, b value) (value, error) {
	if !numTag(a.tag) || !numTag(b.tag) {
		return value{}, errTypeMismatch
	}
	if a.tag == tagReal || b.tag == tagReal {
		return realVal(a.asFloat() * b.asFloat()), nil
	}
	c := a.ival * b.ival
	if a.ival != 0 && c/a.ival != b.ival {
		return realVal(float64(a.ival) * float64(b.ival)), nil
	}
	return intVal(c), nil
}

// vmEqual compares two values for equality. Mixed int/real compares as float64.
func vmEqual(a, b value) bool {
	if numTag(a.tag) && numTag(b.tag) {
		return a.asFloat() == b.asFloat()
	}
	if a.tag == tagBool && b.tag == tagBool {
		return a.ival == b.ival
	}
	return false
}

// vmBoolOrBitwise dispatches and/or/xor for both bool and int operand pairs.
func vmBoolOrBitwise(a, b value, boolFn func(bool, bool) bool, intFn func(int, int) int) (value, error) {
	if a.tag == tagBool && b.tag == tagBool {
		return boolVal(boolFn(a.ival != 0, b.ival != 0)), nil
	}
	if a.tag == tagInt && b.tag == tagInt {
		return intVal(intFn(a.ival, b.ival)), nil
	}
	return value{}, errTypeMismatch
}

var (
	errStackUnderflow = errors.New("stack underflow")
	errStackOverflow  = errors.New("stack overflow")
	errTypeMismatch   = errors.New("type mismatch")
	errDivByZero      = errors.New("division by zero")
)
