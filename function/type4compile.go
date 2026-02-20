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
	"fmt"
	"strconv"
)

// operator name → opcode for the allowed Type 4 operators (PDF spec Table 42)
var opNames = map[string]opCode{
	"abs": opAbs, "add": opAdd, "atan": opAtan, "ceiling": opCeiling,
	"cos": opCos, "cvi": opCvi, "cvr": opCvr, "div": opDiv,
	"exp": opExp, "floor": opFloor, "idiv": opIdiv, "ln": opLn,
	"log": opLog, "mod": opMod, "mul": opMul, "neg": opNeg,
	"round": opRound, "sin": opSin, "sqrt": opSqrt, "sub": opSub,
	"truncate": opTruncate,
	"and":      opAnd, "bitshift": opBitshift, "eq": opEq, "ge": opGe,
	"gt": opGt, "le": opLe, "lt": opLt, "ne": opNe, "not": opNot,
	"or": opOr, "xor": opXor,
	"copy": opCopy, "dup": opDup, "exch": opExch, "index": opIndex,
	"pop": opPop, "roll": opRoll,
}

// compile converts a Type 4 PostScript program to bytecode.
func compile(program string) ([]instruction, error) {
	tokens, err := tokenize(program)
	if err != nil {
		return nil, err
	}
	return compileTokens(tokens)
}

// token types
const (
	tokInt   = iota // ival holds the integer
	tokReal         // fval holds the float
	tokTrue         // boolean true
	tokFalse        // boolean false
	tokName         // sval holds the operator name
	tokOpen         // {
	tokClose        // }
)

type token struct {
	typ  int
	ival int
	fval float64
	sval string
}

// tokenize scans a Type 4 PostScript program into tokens.
func tokenize(src string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(src) {
		c := src[i]

		// whitespace
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\x00' {
			i++
			continue
		}

		// comment
		if c == '%' {
			for i < len(src) && src[i] != '\n' && src[i] != '\r' {
				i++
			}
			continue
		}

		// braces
		if c == '{' {
			tokens = append(tokens, token{typ: tokOpen})
			i++
			continue
		}
		if c == '}' {
			tokens = append(tokens, token{typ: tokClose})
			i++
			continue
		}

		// number or name: scan until delimiter
		start := i
		for i < len(src) {
			ch := src[i]
			if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '\f' || ch == '\x00' ||
				ch == '{' || ch == '}' || ch == '%' {
				break
			}
			i++
		}
		word := src[start:i]

		// try integer
		if iv, err := strconv.ParseInt(word, 10, 64); err == nil {
			tokens = append(tokens, token{typ: tokInt, ival: int(iv)})
			continue
		}

		// try real
		if fv, err := strconv.ParseFloat(word, 64); err == nil {
			tokens = append(tokens, token{typ: tokReal, fval: fv})
			continue
		}

		// keywords
		if word == "true" {
			tokens = append(tokens, token{typ: tokTrue})
			continue
		}
		if word == "false" {
			tokens = append(tokens, token{typ: tokFalse})
			continue
		}

		// operator name (including "if" and "ifelse")
		tokens = append(tokens, token{typ: tokName, sval: word})
	}
	return tokens, nil
}

// compileTokens translates a token stream to bytecode instructions.
func compileTokens(tokens []token) ([]instruction, error) {
	code, _, err := compileBlock(tokens, 0, false)
	return code, err
}

// compileBlock compiles tokens starting at pos. If inBlock is true, it stops
// at the matching '}'. Returns the compiled instructions and the next token
// position.
func compileBlock(tokens []token, pos int, inBlock bool) ([]instruction, int, error) {
	var code []instruction

	// pending holds compiled blocks collected from { ... } that have not
	// yet been consumed by "if" or "ifelse".
	var pending [][]instruction

	for pos < len(tokens) {
		tok := tokens[pos]
		pos++

		switch tok.typ {
		case tokInt:
			code = append(code, instruction{op: opPushInt, ival: tok.ival})
		case tokReal:
			code = append(code, instruction{op: opPushReal, fval: tok.fval})
		case tokTrue:
			code = append(code, instruction{op: opPushTrue})
		case tokFalse:
			code = append(code, instruction{op: opPushFalse})

		case tokOpen:
			// compile the sub-block
			block, next, err := compileBlock(tokens, pos, true)
			if err != nil {
				return nil, 0, err
			}
			pos = next
			pending = append(pending, block)

		case tokClose:
			if !inBlock {
				return nil, 0, fmt.Errorf("unexpected '}'")
			}
			if len(pending) > 0 {
				return nil, 0, fmt.Errorf("unused procedure body in block")
			}
			return code, pos, nil

		case tokName:
			name := tok.sval
			switch name {
			case "if":
				if len(pending) < 1 {
					return nil, 0, fmt.Errorf("'if' requires one procedure body")
				}
				body := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				// emit: jumpIfFalse over body
				code = append(code, instruction{op: opJumpIfFalse, ival: len(body)})
				code = append(code, body...)

			case "ifelse":
				if len(pending) < 2 {
					return nil, 0, fmt.Errorf("'ifelse' requires two procedure bodies")
				}
				falseBody := pending[len(pending)-1]
				trueBody := pending[len(pending)-2]
				pending = pending[:len(pending)-2]
				// emit: jumpIfFalse (skip trueBody + jump), trueBody, jump (skip falseBody), falseBody
				code = append(code, instruction{op: opJumpIfFalse, ival: len(trueBody) + 1})
				code = append(code, trueBody...)
				code = append(code, instruction{op: opJump, ival: len(falseBody)})
				code = append(code, falseBody...)

			default:
				op, ok := opNames[name]
				if !ok {
					return nil, 0, fmt.Errorf("unknown operator %q", name)
				}
				code = append(code, instruction{op: op})
			}
		}
	}

	if inBlock {
		return nil, 0, fmt.Errorf("unterminated '{'")
	}
	if len(pending) > 0 {
		return nil, 0, fmt.Errorf("unused procedure body at end of program")
	}
	return code, pos, nil
}
