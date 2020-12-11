package pdflib

import (
	"bytes"
	"fmt"
)

// https://www.adobe.com/content/dam/acom/en/devnet/pdf/pdfs/PDF32000_2008.pdf
// https://www.youtube.com/watch?v=HxaD_trXwRE

type itemType int

const (
	itemError itemType = iota
	itemHeader
	itemEOL
	itemEOF
	itemFileError
)

type item struct {
	typ itemType
	val []byte
}

func (i item) String() string {
	switch i.typ {
	case itemError:
		return fmt.Sprintf("ERROR %q", string(i.val))
	default:
		return fmt.Sprintf("item<%d> %q", i.typ, string(i.val))
	}
}

type lexer struct {
	file  *File
	start int64
	pos   int64
	items chan item
}

func (l *lexer) emit(typ itemType) {
	buf, err := l.file.Get(l.start, l.pos, false)
	if err != nil {
		l.items <- item{itemFileError, []byte(err.Error())}
		return
	}
	l.items <- item{typ, buf}
	l.start = l.pos
}

type stateFn func(*lexer) stateFn

func lexHeader(l *lexer) stateFn {
	next, err := l.file.Get(0, 8, false)
	if err != nil {
		l.items <- item{itemFileError, []byte(err.Error())}
		return nil
	}
	if !bytes.HasPrefix(next, []byte("%PDF-1.")) ||
		next[7] < '0' ||
		next[7] > '9' {
		l.emit(itemError)
		return nil
	}
	l.pos += 8
	l.emit(itemHeader)
	return lexEOL
}

func lexEOL(l *lexer) stateFn {
	next, err := l.file.Get(l.pos, l.pos+2, true)
	if err != nil {
		l.items <- item{itemFileError, []byte(err.Error())}
		return nil
	}
	if len(next) < 1 || (next[0] != 0x0D && next[0] != 0x0A) {
		l.emit(itemError)
		return nil
	}
	skip := 1
	if len(next) > 1 && next[0] == 0x0D && next[1] == 0x0A {
		skip = 2
	}
	l.pos += int64(skip)
	l.emit(itemEOL) // TODO(voss): maybe drop this instead?
	return lexCommentMaybe
}

func lexCommentMaybe(l *lexer) stateFn {
	return nil
}

func (l *lexer) run() {
	state := lexHeader
	for state != nil {
		state = state(l)
	}
	close(l.items)
}

func isWhiteSpace(c byte) bool {
	switch c {
	case 0, 9, 10, 12, 13, 32:
		return true
	default:
		return false
	}
}

func isEOL(c byte) bool {
	// TODO(voss): CRLF counts as one EOL marker
	return c == 0x0D || c == 0x0A
}
