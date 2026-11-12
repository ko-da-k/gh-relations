package render

import (
	"sort"
	"strings"
	"testing"

	"golang.org/x/text/width"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// emittedGlyphs is every rune this package can write that does not come from a
// title or a reference: the connectors, the shapes, the markers and every
// sigil. It is built from the constants themselves, so a glyph added to the
// renderer without a thought for its width shows up here.
func emittedGlyphs() []rune {
	var b strings.Builder
	b.WriteString(connectorBranch)
	b.WriteString(connectorLast)
	b.WriteString(indentBar)
	b.WriteString(shapeIssue)
	b.WriteString(shapePull)
	b.WriteString(markerBack)
	b.WriteString(markerUnxpd)
	for _, l := range linkSigils {
		b.WriteRune(l.sigil)
	}
	for _, t := range []graph.EdgeType{
		graph.EdgeCloses, graph.EdgeParent, graph.EdgeBlocks, graph.EdgeRelates,
		graph.EdgeDuplicate, graph.EdgeTracked, graph.EdgeMention,
	} {
		for _, fromEnd := range []bool{true, false} {
			if r, ok := sigilFor(t, fromEnd); ok {
				b.WriteRune(r)
			}
		}
	}

	seen := map[rune]bool{}
	var out []rune
	for _, r := range b.String() {
		if r == ' ' || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func TestAmbiguousGlyphsAgreesWithTheUnicodeData(t *testing.T) {
	// Given: pessimisticWidth budgets the runes in ambiguousGlyphs at two
	// columns and leaves the rest to uniseg, which fixes Ambiguous at one. The
	// list is hardcoded so that the renderer carries no Unicode table of its
	// own; this test is what keeps the hardcoded list honest.
	for _, r := range emittedGlyphs() {
		t.Run(string(r), func(t *testing.T) {
			// When: the list and the Unicode data are asked the same question
			listed := strings.ContainsRune(ambiguousGlyphs, r)
			actual := width.LookupRune(r).Kind() == width.EastAsianAmbiguous

			// Then: they agree. A glyph that is Ambiguous but unlisted would be
			// budgeted at one column and could overflow the terminal; a listed
			// glyph that is not would waste a column on every line.
			if listed != actual {
				t.Errorf("U+%04X %q: ambiguousGlyphs says %v, golang.org/x/text/width says %v",
					r, r, listed, actual)
			}
		})
	}
}

func TestAmbiguousGlyphsListsNothingTheRendererNeverWrites(t *testing.T) {
	// Given: the glyphs the renderer can emit
	emitted := map[rune]bool{}
	for _, r := range emittedGlyphs() {
		emitted[r] = true
	}

	// When / Then: the list holds no leftovers from a glyph that was replaced
	for _, r := range ambiguousGlyphs {
		if !emitted[r] {
			t.Errorf("ambiguousGlyphs lists U+%04X %q, which the renderer no longer writes", r, r)
		}
	}
}
