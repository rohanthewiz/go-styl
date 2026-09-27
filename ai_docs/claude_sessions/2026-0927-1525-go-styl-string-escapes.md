# Session: backslash escapes in strings (N-026) (2026-09-27)

Session ID: `6e41c92e-5a0b-4cdf-b02a-b5b9e0524fcc`

Implemented next-list item N-026: backslash escapes in quoted strings were
dropped. `content "\2022"` came out as `"2022"`, so the page showed the text
instead of a bullet. The CSS was silently wrong. This was one of the blockers
for N-022 (compile cema with go-styl).

This is the same session as `2026-0927-1357-go-styl-multiline-selectors`
(N-024) and `2026-0927-1423-go-styl-trailing-parent-ref` (N-025).

## Stylus behavior (probed against stylus 0.64)

- **A string's value is its raw source text, backslashes included.** Stylus
  prints these unchanged:
  - `"\2022"`, `'\201C'`, `"\f101"`
  - `"\a"`, `"tab\9"`, `"back\\slash"`
  - multi-escape font names like `"\5FAE\8F6F\96C5\9ED1"`
- **Functions see the backslash too.** `unquote("\2022")` gives `\2022`, and
  `length("\2022")` is 5.
- **Escaped quotes don't parse.** Stylus's string rule doesn't honor escapes,
  so `'it\'s'` and `"say \"hi\""` are a ParseError.

## What changed

- **`scanString`** (`internal/lexer/lexer.go`):
  - When the lexer meets `\`, it now keeps both the backslash and the rune
    after it. Before, it dropped the backslash and kept only the rune.
  - Pairing the two means an escaped quote doesn't end the string. go-styl
    outputs `'it\'s'` unchanged, which is valid CSS; Stylus fails to parse it.
- **Nothing else had to change.** `value.Str` prints as quote + value +
  quote, with no re-escaping.
- **Token spacing is now accurate for escaped strings.** `setSpacing` works
  out where each token ends from `len(Text) + 2`. Before, a string with
  escapes came out shorter than its source, so that calculation was off.

## Tests

- **`string_escape_test.go`** has 11 cases:
  - unicode escapes, single-quoted strings, an icon-font codepoint
  - an escaped backslash, `\a`, a quote inside a string of the other kind
  - a multi-escape font name
  - `unquote`, and an escape carried through a variable
  - the two escaped-quote cases, which are go-styl-only
- **`TestLexStringEscapes`** (`internal/lexer/lexer_test.go`) checks the raw
  token text.
- `go test ./...` passes. Difftest is unchanged at 23/32.

## Notes

- **String operations that still differ** (raised as N-036):
  - `length("abc")` is 3 in Stylus because it counts characters. go-styl
    gives 1, treating the string as a one-item list.
  - `"x" + "y"` is `'xy'` in Stylus. go-styl gives the error
    `cannot apply "+" to string and string`.
- **Not part of this item:** the sprintf-style `%` on strings is N-028.
- The untracked `.cats-todo/` folder isn't from this work and is left out of
  commits.

## Next

Closed: N-026. Declined: None. Raised: N-036.
Deferred: None. Promoted: None.
Updated: N-022 (blockers now N-027…N-030). Full list: `ai_docs/todo/next-list.md`.
