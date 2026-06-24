package css

import (
	"encoding/json"
	"sort"
	"strings"
)

// SourceMap collects generated→original position segments during rendering and
// encodes them as a Source Map v3 document. Only a single source file is tracked
// (index 0); names are unused.
type SourceMap struct {
	segs    []segment
	file    string
	source  string // source filename recorded in "sources"
	content string // original source text for "sourcesContent" ("" to omit)
	hasFile bool
}

type segment struct {
	genLine, genCol, srcLine, srcCol int
}

// NewSourceMap creates a collector. file is the generated filename (for the "file"
// field, optional), source is the .styl path, and content is the original source
// (embedded as sourcesContent so the map is self-contained; pass "" to omit).
func NewSourceMap(file, source, content string) *SourceMap {
	if source == "" {
		source = "input.styl"
	}
	return &SourceMap{file: file, source: source, content: content, hasFile: file != ""}
}

func (m *SourceMap) add(genLine, genCol, srcLine, srcCol int) {
	m.segs = append(m.segs, segment{genLine, genCol, srcLine, srcCol})
}

// JSON renders the Source Map v3 document.
func (m *SourceMap) JSON() string {
	doc := struct {
		Version        int      `json:"version"`
		File           string   `json:"file,omitempty"`
		Sources        []string `json:"sources"`
		SourcesContent []string `json:"sourcesContent,omitempty"`
		Names          []string `json:"names"`
		Mappings       string   `json:"mappings"`
	}{
		Version:  3,
		Sources:  []string{m.source},
		Names:    []string{},
		Mappings: m.encodeMappings(),
	}
	if m.hasFile {
		doc.File = m.file
	}
	if m.content != "" {
		doc.SourcesContent = []string{m.content}
	}
	out, _ := json.Marshal(doc)
	return string(out)
}

// encodeMappings produces the VLQ-encoded "mappings" string. Generated columns
// reset per output line; source index/line/column deltas persist across the whole
// stream, per the Source Map v3 spec.
func (m *SourceMap) encodeMappings() string {
	if len(m.segs) == 0 {
		return ""
	}
	segs := append([]segment(nil), m.segs...)
	sort.Slice(segs, func(i, j int) bool {
		if segs[i].genLine != segs[j].genLine {
			return segs[i].genLine < segs[j].genLine
		}
		return segs[i].genCol < segs[j].genCol
	})

	maxLine := segs[len(segs)-1].genLine
	var b strings.Builder

	prevGenCol := 0
	prevSrcLine := 0
	prevSrcCol := 0
	// prevSrcIdx is always 0 (single source), so its delta is 0 on the first
	// segment and 0 thereafter.
	prevSrcIdx := 0

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
			writeVLQ(&b, 0-prevSrcIdx)
			writeVLQ(&b, s.srcLine-prevSrcLine)
			writeVLQ(&b, s.srcCol-prevSrcCol)
			prevGenCol = s.genCol
			prevSrcIdx = 0
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
