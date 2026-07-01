// Package diag defines positioned compile errors: file:line:col diagnostics
// carried as a concrete type so positions can be filled in as outer layers
// learn them (the lexer/parser know lines, the evaluator knows statements and
// files, the public API knows the top-level filename).
package diag

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Error is a compile error anchored to a source position. Line and Col are
// 1-based; 0 means unknown. File may be empty until an enclosing layer fills
// it in (see SetFile / WrapPos).
type Error struct {
	File      string
	Line, Col int
	Msg       string // message without the position prefix
	Err       error  // optional underlying error
}

func (e *Error) Error() string {
	var b strings.Builder
	if e.File != "" {
		b.WriteString(e.File)
	} else {
		b.WriteString("<input>")
	}
	if e.Line > 0 {
		b.WriteByte(':')
		b.WriteString(strconv.Itoa(e.Line))
		if e.Col > 0 {
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(e.Col))
		}
	}
	b.WriteString(": ")
	b.WriteString(e.Msg)
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// Errorf creates a positioned error. Use 0 for an unknown line or column.
func Errorf(line, col int, format string, args ...any) *Error {
	return &Error{Line: line, Col: col, Msg: fmt.Sprintf(format, args...)}
}

// WrapPos anchors err at the given position. An already positioned error keeps
// its (innermost) position; only its unknown fields are filled in.
func WrapPos(err error, file string, line, col int) error {
	if err == nil {
		return nil
	}
	var de *Error
	if errors.As(err, &de) {
		if de.File == "" {
			de.File = file
		}
		if de.Line == 0 {
			de.Line, de.Col = line, col
		} else if de.Col == 0 && de.Line == line {
			de.Col = col
		}
		return err
	}
	return &Error{File: file, Line: line, Col: col, Msg: err.Error(), Err: err}
}

// SetFile fills in the file on a positioned error that does not have one yet.
// Other errors are returned unchanged.
func SetFile(err error, file string) error {
	var de *Error
	if err != nil && file != "" && errors.As(err, &de) && de.File == "" {
		de.File = file
	}
	return err
}
