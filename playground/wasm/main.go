//go:build js && wasm

// Command wasm is the WebAssembly build of go-styl backing the browser
// playground. It installs a global `goStyl` object with:
//
//	goStyl.compile(src, {pretty, mergeDuplicates, sourcemap,
//	                     globals, customProperties, pruneHTML}) ->
//	    {css, map, pruned, used, ms} | {error, file, line, col, ms}
//	goStyl.examples() -> [{name, source}]
//	goStyl.version -> module version string
//
// With pruneHTML (a rendered page), the compile result also carries
// `pruned`: the stylesheet reduced to the rules that page uses (styl.Prune,
// the engine behind critical-CSS middleware), and `used`: the names found
// in the HTML as {classes, ids, tags}.
//
// @import in playground source resolves against the embedded examples
// filesystem, so the bundled examples (and user snippets that import them)
// work unchanged.
package main

import (
	"errors"
	"io/fs"
	"runtime/debug"
	"sort"
	"syscall/js"
	"time"

	styl "github.com/rohanthewiz/go-styl"
	"github.com/rohanthewiz/go-styl/examples"
	"github.com/rohanthewiz/go-styl/internal/diag"
)

func main() {
	g := js.Global()
	api := g.Get("Object").New()
	api.Set("compile", js.FuncOf(compile))
	api.Set("examples", js.FuncOf(listExamples))
	api.Set("version", version())
	g.Set("goStyl", api)
	// Tell the page the API is ready (wasm instantiation is async).
	if cb := g.Get("goStylReady"); cb.Type() == js.TypeFunction {
		cb.Invoke()
	}
	select {} // keep the Go runtime alive for future JS calls
}

// compile implements goStyl.compile(src, opts?).
func compile(_ js.Value, args []js.Value) any {
	if len(args) < 1 || args[0].Type() != js.TypeString {
		return map[string]any{"error": "compile(src, opts?): src must be a string"}
	}
	src := args[0].String()
	opts := styl.Options{
		Pretty:   true,
		FS:       examples.FS,
		Filename: "playground.styl",
	}
	sourceMap := false
	pruneHTML := ""
	if len(args) > 1 && args[1].Type() == js.TypeObject {
		o := args[1]
		if v := o.Get("pruneHTML"); v.Type() == js.TypeString {
			pruneHTML = v.String()
		}
		if v := o.Get("pretty"); v.Type() == js.TypeBoolean {
			opts.Pretty = v.Bool()
		}
		if v := o.Get("mergeDuplicates"); v.Type() == js.TypeBoolean {
			opts.MergeDuplicates = v.Bool()
		}
		if v := o.Get("sourcemap"); v.Type() == js.TypeBoolean {
			sourceMap = v.Bool()
		}
		if v := o.Get("globals"); v.Type() == js.TypeObject {
			opts.Globals = toGlobals(v)
		}
		if v := o.Get("customProperties"); js.Global().Get("Array").Call("isArray", v).Bool() {
			opts.CustomProperties = toStrings(v)
		}
	}
	opts.SourceMap = sourceMap

	start := time.Now()
	res, err := styl.Build(src, opts)
	ms := float64(time.Since(start).Microseconds()) / 1000
	if err != nil {
		out := map[string]any{"error": err.Error(), "ms": ms}
		var de *diag.Error
		if errors.As(err, &de) {
			out["file"] = de.File
			out["line"] = de.Line
			out["col"] = de.Col
			out["msg"] = de.Msg
		}
		return out
	}
	out := map[string]any{"css": res.CSS, "ms": ms}
	if sourceMap {
		out["map"] = res.Map
	}
	if pruneHTML != "" {
		// The full build above already succeeded, so Prune (same source and
		// options) only differs in which rules it keeps.
		used := styl.UsedFromHTML(pruneHTML)
		pruned, err := styl.Prune(src, used, opts)
		if err != nil {
			out["pruneError"] = err.Error()
		} else {
			out["pruned"] = pruned
		}
		out["used"] = map[string]any{
			"classes": toAny(used.Classes),
			"ids":     toAny(used.IDs),
			"tags":    toAny(used.Tags),
		}
		out["ms"] = float64(time.Since(start).Microseconds()) / 1000
	}
	return out
}

// toGlobals converts a JS object into Options.Globals. String values are
// parsed as Stylus value expressions on the Go side; numbers, booleans, and
// null map to their Stylus counterparts. Other value types are dropped.
func toGlobals(o js.Value) map[string]any {
	keys := js.Global().Get("Object").Call("keys", o)
	g := make(map[string]any, keys.Length())
	for i := range keys.Length() {
		name := keys.Index(i).String()
		switch v := o.Get(name); v.Type() {
		case js.TypeString:
			g[name] = v.String()
		case js.TypeNumber:
			g[name] = v.Float()
		case js.TypeBoolean:
			g[name] = v.Bool()
		case js.TypeNull:
			g[name] = nil
		}
	}
	if len(g) == 0 {
		return nil
	}
	return g
}

// toAny converts a []string into a []any, the element type syscall/js can
// turn into a JS array.
func toAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// toStrings converts a JS array into a []string, skipping non-string entries.
func toStrings(arr js.Value) []string {
	n := arr.Length()
	out := make([]string, 0, n)
	for i := range n {
		if v := arr.Index(i); v.Type() == js.TypeString {
			out = append(out, v.String())
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// listExamples implements goStyl.examples().
func listExamples(_ js.Value, _ []js.Value) any {
	names, err := fs.Glob(examples.FS, "*.styl")
	if err != nil {
		return []any{}
	}
	sort.Strings(names)
	list := make([]any, 0, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(examples.FS, name)
		if err != nil {
			continue
		}
		list = append(list, map[string]any{"name": name, "source": string(data)})
	}
	return list
}

// version reports the go-styl module version baked into the build, falling
// back to "dev" for local (replace-directive / uncommitted) builds.
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				return s.Value[:7]
			}
		}
	}
	return "dev"
}
