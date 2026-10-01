package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriteOutputCreatesDirs: -o into a directory that doesn't exist yet
// creates it (and any parents), and a second write into it still works.
func TestWriteOutputCreatesDirs(t *testing.T) {
	p := filepath.Join(t.TempDir(), "build", "css", "app.css")
	if err := writeOutput(p, []byte("a{}")); err != nil {
		t.Fatal(err)
	}
	if err := writeOutput(p+".map", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(p); err != nil || string(got) != "a{}" {
		t.Errorf("read back %q, %v", got, err)
	}
}
