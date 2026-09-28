package eval

import (
	"github.com/rohanthewiz/go-styl/internal/ast"
	"github.com/rohanthewiz/go-styl/internal/css"
)

// EvaluatePruned evaluates a stylesheet like Evaluate, then drops every rule
// whose selectors cannot match a document limited to the given used names
// (see css.Prune for the exact matching rules).
func EvaluatePruned(sheet *ast.Stylesheet, opts Options, used css.UsedNames) (string, error) {
	ev, nodes, err := evalNodes(sheet, opts)
	if err != nil {
		return "", err
	}
	out := css.RenderSheet(css.Prune(nodes, used, opts.Pretty), opts.Pretty, nil)
	if err := ev.checkOutput(out); err != nil {
		return "", err
	}
	return out, nil
}
