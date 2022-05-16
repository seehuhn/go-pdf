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

package builder

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

func lex(input string) (*lexer, <-chan item) {
	c := make(chan item)
	l := &lexer{
		input: input,
		items: c,
		line:  1,
	}
	go l.run()
	return l, c
}

// itemType identifies the type of lexer items.
type itemType int

const (
	itemError itemType = iota
	itemEOF
	itemEOL
	itemArrow
	itemAt
	itemBar
	itemColon
	itemComma
	itemEqual
	itemHyphen
	itemIdentifier
	itemInteger
	itemOr
	itemSemicolon
	itemSlash
	itemSquareBracketClose
	itemSquareBracketOpen
	itemString
)

type item struct {
	typ itemType
	val string
}

func (i item) String() string {
	switch i.typ {
	case itemError:
		return i.val
	case itemEOF:
		return "EOF"
	}
	if len(i.val) > 10 {
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

type lexer struct {
	input string
	line  int
	start int // start position of current item
	pos   int // current position in input
	width int // width of last rune read from input
	items chan<- item
}

func (l *lexer) run() {
	for state := lexStart; state != nil; {
		state = state(l)
	}
	close(l.items)
}

func (l *lexer) emit(t itemType) {
	l.items <- item{typ: t, val: l.input[l.start:l.pos]}
	l.start = l.pos
}

func (l *lexer) next() (r rune) {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) backup() {
	l.pos -= l.width
}

func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{
		typ: itemError,
		val: fmt.Sprintf(format, args...),
	}
	return nil
}

const eof = rune(0)

// stateFn represents the state of the scanner
// as a function that returns the next state.
type stateFn func(*lexer) stateFn

var singleCharTokens = map[rune]itemType{
	',': itemComma,
	';': itemSemicolon,
	':': itemColon,
	'[': itemSquareBracketOpen,
	']': itemSquareBracketClose,
	'@': itemAt,
	'/': itemSlash,
	'=': itemEqual,
}

func isSingleCharToken(r rune) bool {
	_, ok := singleCharTokens[r]
	return ok
}

func lexStart(l *lexer) stateFn {
	// Skip whitespace.
	for {
		r := l.next()
		if r == eof {
			l.emit(itemEOF)
			return nil
		}
		if r == '\n' || !unicode.IsSpace(r) {
			l.backup()
			break
		}
	}
	l.ignore()

	// At least one more rune is available, and it is not a white space character.

	r := l.next()
	switch {
	case r == '\n':
		l.emit(itemEOL)
		return lexStart
	case unicode.IsLetter(r) || r == '.' || r == '_':
		return lexIdentifier
	case r == '"':
		return lexString
	case r >= '0' && r <= '9':
		return lexInteger
	case isSingleCharToken(r):
		l.emit(singleCharTokens[r])
		return lexStart
	case r == '-':
		r := l.next()
		if r == '>' {
			l.emit(itemArrow)
		} else {
			l.backup()
			l.emit(itemHyphen)
		}
		return lexStart
	case r == '|':
		r := l.next()
		if r == '|' {
			l.emit(itemOr)
		} else {
			l.backup()
			l.emit(itemBar)
		}
		return lexStart
	case r == '#':
		return lexComment
	default:
		return l.errorf("unexpected character %#U", r)
	}
}

func lexIdentifier(l *lexer) stateFn {
	for {
		r := l.next()
		if !(unicode.IsLetter(r) || r == '.' || r == '_' || unicode.IsDigit(r)) {
			if r != eof {
				l.backup()
			}
			l.emit(itemIdentifier)
			return lexStart
		}
	}
}

func lexString(l *lexer) stateFn {
	escape := false
	for {
		r := l.next()
		if r == eof || r == '\n' {
			return l.errorf("unterminated string")
		}
		if escape {
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			continue
		}
		if r == '"' {
			break
		}
	}
	l.emit(itemString)
	return lexStart
}

func lexInteger(l *lexer) stateFn {
	for {
		r := l.next()
		if r < '0' || r > '9' {
			l.backup()
			break
		}
	}
	l.emit(itemInteger)
	return lexStart
}

func lexComment(l *lexer) stateFn {
	for {
		r := l.next()
		if r == eof || r == '\n' {
			break
		}
	}
	l.backup()
	l.ignore()
	return lexStart
}
