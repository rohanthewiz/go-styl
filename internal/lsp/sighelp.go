package lsp

import (
	"sort"
	"strings"

	"github.com/rohanthewiz/go-styl/internal/eval"
)

// --- signature help ---

// signatureHelp shows a user function or mixin's parameters while its
// arguments are typed: `button(|` or `+button(bg, |`.
//
// The call is found textually on the cursor's line (the text being typed
// rarely parses): scanning left from the cursor over code (comments and
// strings masked), the first `(` not closed before the cursor opens the
// call, and the word glued to its left names it. Commas at that paren depth
// count the active parameter. A name with no user definition in reach falls
// back to the built-in of that name (a user function shadows a built-in, as
// in a compile), whose parameter lists come from the builtin registry.
func (s *Server) signatureHelp(d *document, pos Position) (any, *rpcError) {
	if pos.Line < 0 || pos.Line >= len(d.lines) {
		return nil, nil
	}
	line := []rune(strings.Split(codeMask(d.text), "\n")[pos.Line])
	c := min(runeCol(d.lines[pos.Line], pos.Character), len(line))

	depth, commas, open := 0, 0, -1
	for i := c - 1; i >= 0 && open < 0; i-- {
		switch line[i] {
		case ')', ']':
			depth++
		case '(', '[':
			if depth == 0 {
				if line[i] == '(' {
					open = i
				} else {
					return nil, nil // inside a list index, not a call
				}
			} else {
				depth--
			}
		case ',':
			if depth == 0 {
				commas++
			}
		}
	}
	if open <= 0 {
		return nil, nil
	}
	end := open
	start := end
	for start > 0 && isWordRune(line[start-1]) {
		start--
	}
	for start < end && line[start] == '-' {
		start++
	}
	if start == end {
		return nil, nil
	}
	name := string(line[start:end])
	var sigs []string
	if an := d.index(); an != nil {
		if df, ok := an.lookup(name, d.path, pos.Line+1, defFunc); ok {
			sigs = []string{df.Sig}
		}
	}
	if sigs == nil {
		sigs = eval.BuiltinSignatures(name)
	}
	if sigs == nil {
		return nil, nil
	}

	// Each calling form is one SignatureInformation. The active one is the
	// first that can take the arguments typed so far (commas+1 of them):
	// with rgba's `rgba(red, green, blue, alpha) | rgba(color, alpha)`, both
	// fit at the first comma and the first form stays active, and the
	// four-argument form is the only candidate from the third argument on.
	// When none fits (too many arguments), the last form is shown.
	help := SignatureHelp{Signatures: []SignatureInformation{}, ActiveSignature: -1}
	for i, sig := range sigs {
		params := sigParams(sig)
		rest := len(params) > 0 && strings.HasSuffix(params[len(params)-1], "...")
		info := SignatureInformation{Label: sig, Parameters: []ParameterInformation{}}
		for _, p := range params {
			info.Parameters = append(info.Parameters, ParameterInformation{Label: p})
		}
		help.Signatures = append(help.Signatures, info)
		if help.ActiveSignature < 0 && (commas < len(params) || rest) {
			help.ActiveSignature = i
		}
	}
	if help.ActiveSignature < 0 {
		help.ActiveSignature = len(sigs) - 1
	}
	params := help.Signatures[help.ActiveSignature].Parameters
	help.ActiveParameter = commas
	if n := len(params); n > 0 && commas >= n && strings.HasSuffix(params[n-1].Label, "...") {
		help.ActiveParameter = n - 1 // a rest parameter takes every remaining argument
	}
	return help, nil
}

// sigParams splits a definition line's parameter list (`name(a, b = f(1, 2),
// rest...)`) at its top-level commas. Each piece is a substring of the
// line, as ParameterInformation string labels must be.
func sigParams(sig string) []string {
	open := strings.IndexByte(sig, '(')
	if open < 0 {
		return nil
	}
	var out []string
	depth, from := 0, open+1
	for i := open + 1; i < len(sig); i++ {
		switch sig[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				if p := strings.TrimSpace(sig[from:i]); p != "" {
					out = append(out, p)
				}
				return out
			}
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(sig[from:i]))
				from = i + 1
			}
		}
	}
	return out
}

// --- CSS property names ---

// atPropertyPosition reports whether the cursor is where a declaration's
// property name goes: inside a block (an indented line, or after `{`/`;` in
// brace syntax), with only name characters typed so far.
func atPropertyPosition(d *document, pos Position) bool {
	if pos.Line < 0 || pos.Line >= len(d.lines) {
		return false
	}
	lineText := d.lines[pos.Line]
	before := string([]rune(lineText)[:min(runeCol(lineText, pos.Character), len([]rune(lineText)))])
	seg := before
	if i := strings.LastIndexAny(before, "{;"); i >= 0 {
		seg = before[i+1:]
	} else if !strings.HasPrefix(before, " ") && !strings.HasPrefix(before, "\t") {
		return false // a root-level line: a selector or an assignment
	}
	seg = strings.TrimLeft(seg, " \t")
	for _, r := range seg {
		if !(r == '-' || r >= 'a' && r <= 'z') {
			return false
		}
	}
	return true
}

// cssProperties is the standard CSS property set offered at property
// position. It is not exhaustive (vendor-prefixed and obsolete properties
// are left out) and doesn't need to be: Stylus passes any property through,
// so this is a typing aid, never a check.
var cssProperties = func() []string {
	list := strings.Fields(`
accent-color align-content align-items align-self all animation
animation-composition animation-delay animation-direction animation-duration
animation-fill-mode animation-iteration-count animation-name
animation-play-state animation-timing-function appearance aspect-ratio
backdrop-filter backface-visibility background background-attachment
background-blend-mode background-clip background-color background-image
background-origin background-position background-position-x
background-position-y background-repeat background-size block-size border
border-block border-block-color border-block-end border-block-start
border-block-style border-block-width border-bottom border-bottom-color
border-bottom-left-radius border-bottom-right-radius border-bottom-style
border-bottom-width border-collapse border-color border-image
border-image-outset border-image-repeat border-image-slice
border-image-source border-image-width border-inline border-inline-color
border-inline-end border-inline-start border-inline-style
border-inline-width border-left border-left-color border-left-style
border-left-width border-radius border-right border-right-color
border-right-style border-right-width border-spacing border-style
border-top border-top-color border-top-left-radius border-top-right-radius
border-top-style border-top-width border-width bottom box-decoration-break
box-shadow box-sizing break-after break-before break-inside caption-side
caret-color clear clip-path color color-scheme column-count column-fill
column-gap column-rule column-rule-color column-rule-style
column-rule-width column-span column-width columns contain container
container-name container-type content content-visibility counter-increment
counter-reset counter-set cursor direction display empty-cells filter flex
flex-basis flex-direction flex-flow flex-grow flex-shrink flex-wrap float
font font-display font-family font-feature-settings font-kerning
font-optical-sizing font-size font-size-adjust font-stretch font-style
font-synthesis font-variant font-variant-caps font-variant-east-asian
font-variant-ligatures font-variant-numeric font-variation-settings
font-weight forced-color-adjust gap grid grid-area grid-auto-columns
grid-auto-flow grid-auto-rows grid-column grid-column-end grid-column-start
grid-row grid-row-end grid-row-start grid-template grid-template-areas
grid-template-columns grid-template-rows hanging-punctuation height hyphens
image-rendering inline-size inset inset-block inset-block-end
inset-block-start inset-inline inset-inline-end inset-inline-start
isolation justify-content justify-items justify-self left letter-spacing
line-break line-clamp line-height list-style list-style-image
list-style-position list-style-type margin margin-block margin-block-end
margin-block-start margin-bottom margin-inline margin-inline-end
margin-inline-start margin-left margin-right margin-top mask mask-image
mask-position mask-repeat mask-size max-block-size max-height
max-inline-size max-width min-block-size min-height min-inline-size
min-width mix-blend-mode object-fit object-position offset opacity order
orphans outline outline-color outline-offset outline-style outline-width
overflow overflow-anchor overflow-wrap overflow-x overflow-y
overscroll-behavior overscroll-behavior-x overscroll-behavior-y padding
padding-block padding-block-end padding-block-start padding-bottom
padding-inline padding-inline-end padding-inline-start padding-left
padding-right padding-top page-break-after page-break-before
page-break-inside perspective perspective-origin place-content place-items
place-self pointer-events position print-color-adjust quotes resize right
rotate row-gap scale scroll-behavior scroll-margin scroll-margin-block
scroll-margin-bottom scroll-margin-inline scroll-margin-left
scroll-margin-right scroll-margin-top scroll-padding scroll-padding-block
scroll-padding-bottom scroll-padding-inline scroll-padding-left
scroll-padding-right scroll-padding-top scroll-snap-align scroll-snap-stop
scroll-snap-type scrollbar-color scrollbar-gutter scrollbar-width
shape-outside tab-size table-layout text-align text-align-last
text-combine-upright text-decoration text-decoration-color
text-decoration-line text-decoration-style text-decoration-thickness
text-emphasis text-indent text-justify text-orientation text-overflow
text-rendering text-shadow text-transform text-underline-offset
text-underline-position text-wrap top touch-action transform
transform-origin transform-style transition transition-behavior
transition-delay transition-duration transition-property
transition-timing-function translate unicode-bidi user-select
vertical-align view-transition-name visibility white-space widows width
will-change word-break word-spacing writing-mode z-index zoom`)
	sort.Strings(list)
	return list
}()
