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
	return m, nil
}
