package styl

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatPublic(t *testing.T) {
	got, err := Format("// theme\n.a\n\tcolor   red\n\n\n\t&:hover\n\t\tcolor blue\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "// theme\n.a\n  color red\n\n  &:hover\n    color blue\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// A parse error is positioned like a compile error.
	_, err = Format(".a\n  color (red\n")
	if err == nil || errors.Is(err, ErrFormatUnsafe) || !strings.Contains(err.Error(), ":2:") {
		t.Errorf("parse error: %v", err)
	}
}
