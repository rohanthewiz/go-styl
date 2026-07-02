package styl_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	styl "github.com/rohanthewiz/go-styl"
	"github.com/rohanthewiz/go-styl/internal/benchgen"
)

// BenchmarkCompileExamples times a compressed compile of every example sheet.
// SetBytes makes `go test -bench` report source MB/s alongside ns/op.
func BenchmarkCompileExamples(b *testing.B) {
	files, err := filepath.Glob("examples/*.styl")
	if err != nil || len(files) == 0 {
		b.Fatalf("no examples: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(filepath.Base(f), func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			for i := 0; i < b.N; i++ {
				if _, err := styl.Compile(string(src), styl.Options{Filename: f}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCompileSynthetic times synthetic sheets of increasing size
// (see internal/benchgen), showing how compile time scales.
func BenchmarkCompileSynthetic(b *testing.B) {
	for _, n := range []int{20, 100, 400} {
		src := benchgen.Sheet(n)
		b.Run(fmt.Sprintf("blocks-%d", n), func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			for i := 0; i < b.N; i++ {
				if _, err := styl.Compile(src, styl.Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCompileVariants times the non-default output modes on the same
// synthetic sheet: pretty printing, duplicate merging, and source maps.
func BenchmarkCompileVariants(b *testing.B) {
	src := benchgen.Sheet(100)
	variants := []struct {
		name string
		opts styl.Options
	}{
		{"compressed", styl.Options{}},
		{"pretty", styl.Options{Pretty: true}},
		{"merge", styl.Options{MergeDuplicates: true}},
		{"sourcemap", styl.Options{SourceMap: true}},
	}
	for _, v := range variants {
		b.Run(v.name, func(b *testing.B) {
			b.SetBytes(int64(len(src)))
			for i := 0; i < b.N; i++ {
				if _, err := styl.Build(src, v.opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
