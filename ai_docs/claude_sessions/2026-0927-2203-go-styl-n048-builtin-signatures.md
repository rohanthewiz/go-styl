# Session: built-in signatures for the LSP (N-048) (2026-09-27)

Session ID: `0790ffb0-9311-4f19-b2bf-920965694169`

Closed N-048. Before this, `eval.BuiltinNames` had names only: signature
help returned nothing for `darken(` and hover said just `darken()` /
"Built-in function".

## The signature is the registration key (`internal/builtin/builtin.go`)

```go
register("darken(color, amount)", darken)
register("rgba(red, green, blue, alpha) | rgba(color, alpha)", rgba)
```

`register(sig, f)` cuts the name at the first `(` and records both
`Registry[name]` and `signatures[name]`, and it panics on a signature with
no parameter list. Making the signature the key means no built-in can be
added without one, and the parameter list sits on the same line as the
registration instead of in a separate table that could drift out of date.
All 94 registrations were rewritten with a script, with each parameter list
checked against the function's `wantArgs`/`len(args)` handling and doc comment.

Notation:
- a simple default is shown (`range(start, stop, step = 1)`,
  `mix(color1, color2, weight = 50%)`, `blend(top, bottom = white)`)
- other optional parameters end in `?` (`alpha(color, value?)`,
  `substr(str, start, length?)`)
- a rest parameter ends in `...` (`push(list, values...)`,
  `merge(dest, src..., deep = false)`)
- more than one calling form is joined with ` | ` (only rgba needs it)

`builtin.Signatures(name) []string` splits the forms.

## Context built-ins (`internal/eval/context.go`)

`ctxBuiltinSigs` sits beside `ctxBuiltins` (selector, selectors,
selector-exists, current-media, define, lookup, add-property, warn, use,
json, prefix-classes). I left the `ctxBuiltins` values unchanged instead of
turning them into a struct, because every caller (`evalCall`, the mixin
fallback) would then have to unwrap it. `eval.BuiltinSignatures(name)` in
`tooling.go` checks the context table first, then the registry.
`internal/eval/tooling_test.go` (the first test file in `internal/eval`)
checks that every name from `BuiltinNames()` has a well-formed signature
starting with `name(`, and that `ctxBuiltinSigs` has no stale entries.

## LSP (`internal/lsp/sighelp.go`, `server.go`)

- **Signature help**: the user definition lookup runs first. A user
  function still takes priority over a built-in, as in a compile. With no
  user definition, it falls back to `eval.BuiltinSignatures`. Each form is
  one `SignatureInformation`. The active form is the first one that can
  take `commas+1` arguments (or ends in a rest parameter), otherwise the
  last. For `rgba(1, 2, |`, the four-channel form is active with parameter 2.
- **Hover** on a built-in shows its forms in a `stylus` code block.
- **Completion** detail is `built-in darken(color, amount)` instead of
  `built-in`.

Tests: `TestSignatureHelp` now checks `darken(red, |` (label and active
parameter 1) and rgba's two forms, replacing the old "built-ins have no
signature" assertion. There is a new `TestBuiltinHover`. `go vet` and
`go test ./...` pass.

## Next

Closed: N-048. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
