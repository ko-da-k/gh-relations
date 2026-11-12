package render

import (
	"bufio"
	"strings"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// Every sigil below is East Asian Narrow. The shapes and the connectors are
// Ambiguous, so a terminal set to draw Ambiguous full width makes them two
// columns wide; indent survives that because every sibling shifts together,
// but a sigil of uneven width would not, since the shape follows it. Hence
// "▴ ▾ ⊚" rather than "△ ▽ ⊙".
const (
	shapeIssue  = "◯"
	shapePull   = "◇"
	markerBack  = "↱"
	markerUnxpd = "+"
)

// linkSigils is every sigil in the order children are drawn in: declared
// relations in EdgeType order, both directions of a kind together, mentions
// last. Each gloss completes linkFrame.
var linkSigils = []struct {
	sigil rune
	gloss string
}{
	{'▸', "closes/closed by"}, // one sigil both ways; the shapes say which end is which
	{'▴', "parent of"},
	{'▾', "sub of"},
	{'⊘', "blocks"},
	{'⊚', "blocked by"},
	{'~', "relates to"},
	{'=', "duplicate"}, // bare: the API does not always say which one is the copy
	{'▪', "tracks"},
	{'▫', "tracked in"},
	{'@', "mentions"},
}

// linkFrame is the line above the sigils, which says whose relation they
// describe.
const linkFrame = "what this line is to its parent above:"

// sigilFor returns the sigil for a line, which reads as what that line is to
// the line above it. fromEnd says whether the node on the line is the From end
// of the edge. A mention has a sigil in one direction only: GitHub reports a
// bare reference from the mentioned side, so the line under it is always the
// item that wrote it.
func sigilFor(t graph.EdgeType, fromEnd bool) (rune, bool) {
	switch t {
	case graph.EdgeCloses:
		return '▸', true
	case graph.EdgeParent:
		if fromEnd {
			return '▴', true
		}
		return '▾', true
	case graph.EdgeBlocks:
		if fromEnd {
			return '⊘', true
		}
		return '⊚', true
	case graph.EdgeRelates:
		return '~', true
	case graph.EdgeDuplicate:
		return '=', true
	case graph.EdgeTracked:
		if fromEnd {
			return '▪', true
		}
		return '▫', true
	case graph.EdgeMention:
		if fromEnd {
			return '@', true
		}
		return 0, false
	}
	return 0, false
}

// sigilRank is a sigil's place in linkSigils, and len(linkSigils) for anything
// not in it.
func sigilRank(s rune) int {
	for i, l := range linkSigils {
		if l.sigil == s {
			return i
		}
	}
	return len(linkSigils)
}

// markerGlosses is the legend's wording for the two end-of-line markers.
var markerGlosses = map[string]string{
	markerUnxpd: "not expanded",
	markerBack:  "drawn above",
}

// stateOrder lists states open first, with the two that share grey at the end.
var stateOrder = []graph.State{
	graph.StateOpen, graph.StateMerged, graph.StateClosed,
	graph.StateDraft, graph.StateNotPlanned,
}

// legendLabelWidth is the column the rows line up on. The labels are ASCII, so
// a byte count is a column count here.
const legendLabelWidth = 7

// writeLegend says what the glyphs mean, listing only what was drawn. Without a
// single sigil the tree is one line, and a key for it would be longer than what
// it explains, so nothing is written at all.
func (t *tree) writeLegend(w *bufio.Writer) {
	links := t.linkRow()
	if links == "" {
		return
	}

	rows := [][2]string{
		{"kind", t.kindRow()},
		{"state", t.stateRow()},
		{"link", linkFrame},
		{"", links},
		{"mark", t.markerRow()},
	}

	wrote := false
	for _, row := range rows {
		if row[1] == "" {
			continue
		}
		if !wrote {
			w.WriteByte('\n')
			wrote = true
		}
		w.WriteString(row[0])
		w.WriteString(strings.Repeat(" ", legendLabelWidth-len(row[0])))
		w.WriteString(row[1])
		w.WriteByte('\n')
	}
}

func (t *tree) kindRow() string {
	var parts []string
	if t.kinds[graph.KindIssue] {
		parts = append(parts, shapeIssue+" issue")
	}
	if t.kinds[graph.KindPullRequest] {
		parts = append(parts, shapePull+" pull request")
	}
	return strings.Join(parts, "  ")
}

// stateRow is written only when there are colours: without them every line
// already carries its state as a word.
func (t *tree) stateRow() string {
	if !t.opts.Color {
		return ""
	}
	var parts []string
	for _, s := range stateOrder {
		if t.states[s] {
			parts = append(parts, colorFor(s)+s.String()+ansiReset)
		}
	}
	return strings.Join(parts, "  ")
}

func (t *tree) linkRow() string {
	var parts []string
	for _, l := range linkSigils {
		if t.sigils[l.sigil] {
			parts = append(parts, string(l.sigil)+" "+l.gloss)
		}
	}
	return strings.Join(parts, "  ")
}

func (t *tree) markerRow() string {
	var parts []string
	for _, m := range []string{markerUnxpd, markerBack} {
		if t.markers[m] {
			parts = append(parts, m+" "+markerGlosses[m])
		}
	}
	return strings.Join(parts, "  ")
}
