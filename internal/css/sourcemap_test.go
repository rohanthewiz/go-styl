package css

import (
	"strings"
	"testing"
)

func TestWriteVLQ(t *testing.T) {
	cases := map[int]string{
		0:   "A",
		1:   "C",
		-1:  "D",
		2:   "E",
		16:  "gB",
		-16: "hB",
		123: "2H",
	}
	for v, want := range cases {
		var b strings.Builder
		writeVLQ(&b, v)
		if got := b.String(); got != want {
			t.Errorf("writeVLQ(%d) = %q, want %q", v, got, want)
		}
	}
}

// decodeVLQ decodes one base64-VLQ value starting at i, returning the value and
// the next index. Used to verify the encoded mappings round-trip.
func decodeVLQ(s string, i int) (int, int) {
	const chars = base64Chars
	shift, result := 0, 0
	for {
		d := strings.IndexByte(chars, s[i])
		i++
		result |= (d & 0x1f) << shift
		if d&0x20 == 0 {
			break
		}
		shift += 5
	}
	value := result >> 1
	if result&1 == 1 {
		value = -value
	}
	return value, i
}

func TestEncodeMappingsRoundTrip(t *testing.T) {
	m := &SourceMap{}
	// gen (line, col) -> src (line, col)
	m.add(0, 0, 0, 0)
	m.add(1, 1, 1, 2)
	m.add(3, 0, 5, 0)

	mappings := m.encodeMappings()
	// Three generated lines have segments; line 2 is empty -> mappings has 3 ';'.
	if strings.Count(mappings, ";") != 3 {
		t.Fatalf("expected 3 line separators, got %q", mappings)
	}

	// Decode the first generated line's single segment back to (0,0,0,0).
	genCol, i := decodeVLQ(mappings, 0)
	srcIdx, i := decodeVLQ(mappings, i)
	srcLine, i := decodeVLQ(mappings, i)
	srcCol, _ := decodeVLQ(mappings, i)
	if genCol != 0 || srcIdx != 0 || srcLine != 0 || srcCol != 0 {
		t.Errorf("first segment = (%d,%d,%d,%d), want (0,0,0,0)", genCol, srcIdx, srcLine, srcCol)
	}
}
