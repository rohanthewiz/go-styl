package eval

import (
	"strings"
	"testing"
)

// TestBuiltinSignatures checks that every built-in, of both kinds, has a
// signature naming it, and that the context built-ins' signature table has
// no entry for a function that no longer exists.
func TestBuiltinSignatures(t *testing.T) {
	for _, name := range BuiltinNames() {
		sigs := BuiltinSignatures(name)
		if len(sigs) == 0 {
			t.Errorf("%s: no signature", name)
		}
		for _, sig := range sigs {
			if !strings.HasPrefix(sig, name+"(") || !strings.HasSuffix(sig, ")") {
				t.Errorf("%s: malformed signature %q", name, sig)
			}
		}
	}
	for name := range ctxBuiltinSigs {
		if _, ok := ctxBuiltins[name]; !ok {
			t.Errorf("ctxBuiltinSigs has %q, which isn't a context built-in", name)
		}
	}
}
