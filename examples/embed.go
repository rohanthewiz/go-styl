// Package examples embeds the example .styl files so tools (like the WASM
// playground) can bundle and list them.
package examples

import "embed"

// FS holds every example stylesheet, including the imports/ partials, so it
// can also serve as an @import filesystem (styl.Options.FS).
//
//go:embed *.styl all:imports
var FS embed.FS
