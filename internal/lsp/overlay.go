package lsp

import (
	"io"
	"io/fs"
	"path"
	"sync"
	"time"
)

// overlayFS is the file system an analysis compile reads through: the disk
// (an os.DirFS at the volume root), with the editor text of every open
// document laid over it.
//
//	fs.ReadFile("home/me/site/_vars.styl")
//	        │
//	        ├─ open in the editor? ──yes──▶ the buffer's text (saved or not)
//	        └─ no ───────────────────────▶ base (the disk)
//
// Without it an importer's diagnostics and hover values would describe the
// imported file as last saved, while definitions and completion (which read
// the editor's text directly) already describe it as typed.
//
// It also records every file the compile reads (reads), which is how the
// server knows which open documents to re-analyze when another one changes:
// a document depends on exactly the files its last compile opened, however
// deeply they were imported.
//
// Limit: directory listings (fs.Glob, for `@import "dir/*"`) come from the
// base only, so a buffer that has never been saved isn't matched by a glob
// import until it is. A named import of it resolves, because Stat and Open
// both consult the overlay first.
type overlayFS struct {
	base  fs.FS
	files map[string]string // fs path (slash-separated, relative to the base) → text

	mu    sync.Mutex // reads is written from the compile; guard it anyway
	reads map[string]bool
}

func newOverlayFS(base fs.FS, files map[string]string) *overlayFS {
	return &overlayFS{base: base, files: files, reads: map[string]bool{}}
}

func (o *overlayFS) note(name string) {
	o.mu.Lock()
	o.reads[name] = true
	o.mu.Unlock()
}

// Open serves an overlaid file from memory and anything else from the base.
// Only regular-file opens are recorded as reads: a Stat while resolving an
// import candidate isn't a dependency.
func (o *overlayFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if text, ok := o.files[name]; ok {
		o.note(name)
		return &memFile{name: name, data: []byte(text)}, nil
	}
	f, err := o.base.Open(name)
	if err == nil {
		if info, serr := f.Stat(); serr == nil && !info.IsDir() {
			o.note(name)
		}
	}
	return f, err
}

// Stat answers from the overlay first, so an import of a buffer that exists
// only in the editor resolves. It's implemented (fs.StatFS) rather than left
// to fs.Stat's Open+Stat fallback so that probing import candidates doesn't
// count as reading them.
func (o *overlayFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	if text, ok := o.files[name]; ok {
		return memInfo{name: path.Base(name), size: int64(len(text))}, nil
	}
	return fs.Stat(o.base, name)
}

// ReadDir delegates to the base (see the glob limit above). Implementing it
// keeps fs.Glob from opening directories through Open.
func (o *overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(o.base, name)
}

// deps returns the fs paths the compile read.
func (o *overlayFS) deps() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]string, 0, len(o.reads))
	for k := range o.reads {
		out = append(out, k)
	}
	return out
}

// memFile is an open overlaid file.
type memFile struct {
	name string
	data []byte
	off  int
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return memInfo{name: path.Base(f.name), size: int64(len(f.data))}, nil
}

func (f *memFile) Read(p []byte) (int, error) {
	if f.off >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.off:])
	f.off += n
	return n, nil
}

func (f *memFile) Close() error { return nil }

type memInfo struct {
	name string
	size int64
}

func (i memInfo) Name() string       { return i.name }
func (i memInfo) Size() int64        { return i.size }
func (i memInfo) Mode() fs.FileMode  { return 0o444 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }
