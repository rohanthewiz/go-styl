package eval

import (
	"sort"

	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/css"
	"github.com/rohanthewiz/go-styl/internal/value"
)

// ManifestVar is one root-scope variable and its final compile-time CSS value.
type ManifestVar struct {
	Name  string
	Value string
}

// Manifest is what a stylesheet exposes to Go code: the class names, IDs, and
// keyframes names present in the rendered CSS, plus the root-scope variables
// (Globals included) with their final values. Lists are sorted and
// duplicate-free so generated code is stable across compiles.
type Manifest struct {
	Classes   []string
	IDs       []string
	Keyframes []string
	Vars      []ManifestVar
}

// ExtractManifest evaluates a stylesheet and collects its Manifest instead of
// rendering CSS.
func ExtractManifest(sheet *ast.Stylesheet, opts Options) (*Manifest, error) {
	ev, nodes, err := evalNodes(sheet, opts)
	if err != nil {
		return nil, err
	}
	return manifestOf(ev, nodes), nil
}

// manifestOf collects the Manifest of an evaluated tree: the names its
// rendered CSS would contain and the evaluator's final root-scope variables.
func manifestOf(ev *evaluator, nodes []css.Node) *Manifest {
	names := css.CollectNames(nodes)
	m := &Manifest{
		Classes:   names.Classes,
		IDs:       names.IDs,
		Keyframes: names.Keyframes,
	}
	for name, v := range ev.rootScope.vars {
		m.Vars = append(m.Vars, ManifestVar{Name: name, Value: value.Deref(v).CSS(true)})
	}
	sort.Slice(m.Vars, func(i, j int) bool { return m.Vars[i].Name < m.Vars[j].Name })
	return m
}

// EvaluateScoped evaluates a stylesheet as a scoped component: class and
// @keyframes names are renamed via rename (see css.Scope) before rendering.
// It returns the rendered CSS, the Manifest of the sheet in local names (so
// generated constants are named after what the author wrote), and the
// local -> scoped mapping of every renamed name.
//
// The Manifest is collected before renaming: it lists local class names plus
// any class that only appears inside ":global(...)" (those simply have no
// entry in the mapping).
func EvaluateScoped(sheet *ast.Stylesheet, opts Options, rename func(string) string) (string, *Manifest, map[string]string, error) {
	ev, nodes, err := evalNodes(sheet, opts)
	if err != nil {
		return "", nil, nil, err
	}
	m := manifestOf(ev, nodes)
	renamed := css.Scope(nodes, rename)
	out := css.RenderSheet(nodes, opts.Pretty, nil)
	if err := ev.checkOutput(out); err != nil {
		return "", nil, nil, err
	}
	return out, m, renamed, nil
}
