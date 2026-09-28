package styl

import (
	"github.com/rohanthewiz/go-styl/internal/parser"
)

// ErrFormatUnsafe is returned by Format when no reformatting of the source
// kept its meaning; the source should be left as it is.
var ErrFormatUnsafe = parser.ErrFormatUnsafe

// Format rewrites Stylus source into canonical layout (the `styl fmt`
// subcommand): two-space indentation by nesting depth (brace depth in brace
// syntax), single spaces inside lines, no trailing whitespace, at most one
// blank line in a row, and a single final newline. Comments are kept.
//
// Only whitespace changes, and the result is checked: it must parse to the
// same stylesheet as the input (source positions aside), or Format falls
// back to fixing just trailing whitespace and blank lines, and failing that
// returns ErrFormatUnsafe. Source that doesn't parse returns its parse error,
// positioned like a Compile error.
func Format(src string) (string, error) {
	out, err := parser.Format(src)
	if err != nil && err != parser.ErrFormatUnsafe {
		return "", compileErr(err, "")
	}
	return out, err
}
