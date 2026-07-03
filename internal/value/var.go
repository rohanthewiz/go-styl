package value

// Var is a variable exposed as a CSS custom property (via
// Options.CustomProperties). In value position it renders var(--name), so the
// browser resolves it at runtime; operations that need the concrete value —
// arithmetic, comparisons, built-ins, interpolation — unwrap it with Deref
// and compute from the compile-time value.
type Var struct {
	Name  string // custom property name, without the "--" prefix
	Inner Value  // the compile-time value
}

func (v *Var) TypeName() string       { return v.Inner.TypeName() }
func (v *Var) String() string         { return v.CSS(true) }
func (v *Var) CSS(pretty bool) string { return "var(--" + v.Name + ")" }

// Deref returns the concrete value behind any chain of Var wrappers.
func Deref(v Value) Value {
	for {
		w, ok := v.(*Var)
		if !ok {
			return v
		}
		v = w.Inner
	}
}
