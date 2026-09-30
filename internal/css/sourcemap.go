package css

import (
	"encoding/json"
	"sort"
	"strings"
)

// SourceMap collects generated→original position segments during rendering and
// encodes them as a Source Map v3 document. names are unused.
//
// Sources. The entry file is always sources[0]. Every other file a position can
// name (an @import, or a mixin defined in one) is registered up front with
// AddSource, and it joins "sources" the first time a segment points into it:
// a partial that only defines variables contributes no output, so listing it
// would only add noise to the DevTools source tree. Positions are keyed by the
// evaluator's file key (Pos.File); the name written to "sources" is chosen by
// the caller, since only it knows how paths should read next to the map.
//
//	Pos.File ──sourceIndex──▶ index ──▶ sources[index] / sourcesContent[index]
//	  ""  or entry key   ─▶ 0
//	  registered key     ─▶ next free index on first use
//	  anything else      ─▶ -1 (segment dropped: no mapping beats a wrong one)
type SourceMap struct {
	segs     []segment
	file     string
	hasFile  bool
	entryKey string   // Pos.File value that also means the entry (see SetEntry)
	sources  []string // "sources"; [0] is the entry
	contents []string // "sourcesContent", parallel to sources ("" = none)
	index    map[string]int
	known    map[string]sourceFile // registered with AddSource
}

type segment struct {
	genLine, genCol, src, srcLine, srcCol int
}

// sourceFile is a registered non-entry source: its "sources" name and text.
type sourceFile struct{ name, content string }

// NewSourceMap creates a collector. file is the generated filename (for the "file"
// field, optional), source is the .styl path, and content is the original source
// (embedded as sourcesContent so the map is self-contained; pass "" to omit).
func NewSourceMap(file, source, content string) *SourceMap {
	if source == "" {
		source = "input.styl"
	}
	return &SourceMap{
		file:     file,
		hasFile:  file != "",
		sources:  []string{source},
		contents: []string{content},
	}
}

// SetEntry records the file key the evaluator uses for the entry source (its
// Filename). Positions carrying it, like those carrying "", map to sources[0].
func (m *SourceMap) SetEntry(key string) { m.entryKey = key }

// AddSource registers a non-entry file under its evaluator key: name is what
// "sources" will show for it and content its text for "sourcesContent" ("" to
// omit). It is listed only once a segment points into it.
func (m *SourceMap) AddSource(key, name, content string) {
	if m.known == nil {
		m.known = map[string]sourceFile{}
	}
	m.known[key] = sourceFile{name: name, content: content}
}

// sourceIndex returns the "sources" index for a position's file key, listing a
// registered file on first use, or -1 when the key is unknown.
func (m *SourceMap) sourceIndex(key string) int {
	if key == "" || key == m.entryKey {
		return 0
	}
	if i, ok := m.index[key]; ok {
		return i
	}
	sf, ok := m.known[key]
	if !ok {
		return -1
	}
	if m.index == nil {
		m.index = map[string]int{}
	}
	// A zero SourceMap (tests) has no entry slot yet; keep index 0 for it.
	if len(m.sources) == 0 {
		m.sources, m.contents = []string{"input.styl"}, []string{""}
	}
	i := len(m.sources)
	m.sources = append(m.sources, sf.name)
	m.contents = append(m.contents, sf.content)
	m.index[key] = i
	return i
}

func (m *SourceMap) add(genLine, genCol int, pos Pos) {
	src := m.sourceIndex(pos.File)
	if src < 0 {
		return
	}
	m.segs = append(m.segs, segment{genLine, genCol, src, pos.Line - 1, pos.Col - 1})
}

// JSON renders the Source Map v3 document.
func (m *SourceMap) JSON() string {
	doc := struct {
		Version        int       `json:"version"`
		File           string    `json:"file,omitempty"`
		Sources        []string  `json:"sources"`
		SourcesContent []*string `json:"sourcesContent,omitempty"`
		Names          []string  `json:"names"`
		Mappings       string    `json:"mappings"`
	}{
		Version:  3,
		Sources:  m.sources,
		Names:    []string{},
		Mappings: m.encodeMappings(),
	}
	if doc.Sources == nil {
		doc.Sources = []string{"input.styl"}
	}
	if m.hasFile {
		doc.File = m.file
	}
	// sourcesContent runs parallel to sources, a null standing for a source
	// with no embedded text. It is left out entirely when no source has any
	// (the spec's "absent" rather than an array of nulls).
	for _, c := range m.contents {
		if c != "" {
			doc.SourcesContent = make([]*string, len(m.contents))
			for i := range m.contents {
				if m.contents[i] != "" {
					doc.SourcesContent[i] = &m.contents[i]
				}
			}
			break
		}
	}
	out, _ := json.Marshal(doc)
	return string(out)
}

// encodeMappings produces the VLQ-encoded "mappings" string. Generated columns
// reset per output line; source index/line/column deltas persist across the whole
// stream (even when the source changes between segments), per the Source Map v3
// spec.
func (m *SourceMap) encodeMappings() string {
	if len(m.segs) == 0 {
		return ""
	}
	segs := append([]segment(nil), m.segs...)
	sort.SliceStable(segs, func(i, j int) bool {
		if segs[i].genLine != segs[j].genLine {
			return segs[i].genLine < segs[j].genLine
		}
		return segs[i].genCol < segs[j].genCol
	})

	maxLine := segs[len(segs)-1].genLine
	var b strings.Builder

	prevGenCol := 0
	prevSrcIdx := 0
	prevSrcLine := 0
	prevSrcCol := 0

	si := 0
	for line := 0; line <= maxLine; line++ {
		if line > 0 {
			b.WriteByte(';')
		}
		prevGenCol = 0
		firstOnLine := true
		for si < len(segs) && segs[si].genLine == line {
			s := segs[si]
			if !firstOnLine {
				b.WriteByte(',')
			}
			firstOnLine = false
			writeVLQ(&b, s.genCol-prevGenCol)
			writeVLQ(&b, s.src-prevSrcIdx)
			writeVLQ(&b, s.srcLine-prevSrcLine)
			writeVLQ(&b, s.srcCol-prevSrcCol)
			prevGenCol = s.genCol
			prevSrcIdx = s.src
			prevSrcLine = s.srcLine
			prevSrcCol = s.srcCol
			si++
		}
	}
	return b.String()
}

const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// writeVLQ appends the Base64 VLQ encoding of v (signed) to b.
func writeVLQ(b *strings.Builder, v int) {
	var u uint
	if v < 0 {
		u = (uint(-v) << 1) | 1
	} else {
		u = uint(v) << 1
	}
	for {
		digit := u & 0x1f
		u >>= 5
		if u > 0 {
			digit |= 0x20 // continuation bit
		}
		b.WriteByte(base64Chars[digit])
		if u == 0 {
			break
		}
	}
}
