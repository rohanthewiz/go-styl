package value

import "strings"

// Hash is a Stylus object: an insertion-ordered map from string keys to values,
// written `{key: value, 'other-key': 1px 2px}`.
//
// Unlike every other value, a Hash is mutable and shared by reference, as in
// Stylus: after `b = a`, both names hold the same *Hash, so `b.x = 1` (and
// merge(b, …)) is visible through a as well. clone() makes an independent
// copy. Keys keep their first-insertion order, which is the order keys(),
// values() and `for k, v in obj` report — Stylus iterates its JS object the
// same way.
type Hash struct {
	keys []string
	vals map[string]Value
}

// NewHash returns an empty object.
func NewHash() *Hash { return &Hash{vals: map[string]Value{}} }

// Get returns the value stored under key.
func (h *Hash) Get(key string) (Value, bool) {
	v, ok := h.vals[key]
	return v, ok
}

// Has reports whether key is present.
func (h *Hash) Has(key string) bool {
	_, ok := h.vals[key]
	return ok
}

// Set stores v under key. A new key is appended to the iteration order; an
// existing key keeps its position.
func (h *Hash) Set(key string, v Value) {
	if _, ok := h.vals[key]; !ok {
		h.keys = append(h.keys, key)
	}
	h.vals[key] = v
}

// Keys returns the keys in insertion order. The slice is shared; callers must
// not modify it.
func (h *Hash) Keys() []string { return h.keys }

// Len is the number of keys.
func (h *Hash) Len() int { return len(h.keys) }

// Clone copies the object. Nested objects are cloned too, so the copy shares
// no mutable state with the original (Stylus's clone() is deep as well).
func (h *Hash) Clone() *Hash {
	out := &Hash{keys: append([]string(nil), h.keys...), vals: make(map[string]Value, len(h.vals))}
	for k, v := range h.vals {
		if nested, ok := v.(*Hash); ok {
			v = nested.Clone()
		}
		out.vals[k] = v
	}
	return out
}

func (h *Hash) TypeName() string { return "object" }
func (h *Hash) String() string   { return h.CSS(true) }

// CSS renders the object as `{key: value, …}`. An object is not a CSS value,
// so this form only reaches output through string concatenation or
// interpolation; the evaluator rejects an object as a declaration value. It
// also serves as the identity used by `==` (which compares CSS forms).
func (h *Hash) CSS(pretty bool) string {
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range h.keys {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(h.vals[k].CSS(pretty))
	}
	b.WriteByte('}')
	return b.String()
}

// KeyString converts a value used as an object key (`obj[k]`, `'k' in obj`,
// define-style names) to the key text: a string's contents, an ident's name,
// or any other value's CSS form (so obj[1] and obj['1'] are the same key, as
// with JS object keys in Stylus).
func KeyString(v Value) string {
	switch x := Deref(v).(type) {
	case *Str:
		return x.Val
	case *Ident:
		return x.Name
	default:
		return x.CSS(true)
	}
}
