// Package render draws a link graph as a tree for a terminal.
//
// It works from a graph.Graph alone and never touches the API, so a test can
// build exactly the graph it wants to see drawn.
package render

import (
	"bufio"
	"fmt"
	"io"
	"sort"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

const (
	connectorBranch = "├"
	connectorLast   = "└"
	indentBar       = "│  "
	indentBlank     = "   "
)

// Options is how the tree should look.
type Options struct {
	// Color turns on the colours that carry the state. With it off the state is
	// written into the line instead.
	Color bool

	// Hyperlinks makes every reference an OSC 8 terminal hyperlink. It is a
	// separate switch from Color.
	Hyperlinks bool

	// Width is the terminal width to fit lines into. Zero means do not truncate.
	Width int

	// Legend writes what the glyphs that appeared mean.
	Legend bool
}

// Tree writes the graph as a tree rooted at g.Root.
func Tree(w io.Writer, g *graph.Graph, opts Options) error {
	root, ok := g.Get(g.Root)
	if !ok {
		return fmt.Errorf("the graph has no node for its root %s", g.Root)
	}
	out := bufio.NewWriter(w)
	t := &tree{
		g:        g,
		opts:     opts,
		children: map[graph.Key][]line{},
		sigils:   map[rune]bool{},
		kinds:    map[graph.Kind]bool{},
		states:   map[graph.State]bool{},
		markers:  map[string]bool{},
		drawn:    map[graph.Key]bool{},
	}
	t.plan()

	t.drawn[g.Root] = true
	out.WriteString(t.label("", root, false))
	out.WriteByte('\n')
	t.renderChildren(out, g.Root, "")

	if opts.Legend {
		t.writeLegend(out)
	}
	return out.Flush()
}

// line is one line drawn under a node.
type line struct {
	target graph.Key

	// sigils is what the connector draws: one glyph per relation this line and
	// the line above it stand in, in the order of linkSigils. There is more
	// than one only where foldSameTarget merged several lines into this one.
	sigils []rune

	// backRef marks a line that only points at a node drawn elsewhere. It ends
	// in "↱" and is not expanded, which is also what keeps a cycle from
	// recursing.
	backRef bool
}

// rank is where the line sorts among its siblings, and last when it has no
// sigil.
func (l line) rank() int {
	if len(l.sigils) == 0 {
		return len(linkSigils)
	}
	return sigilRank(l.sigils[0])
}

type tree struct {
	g        *graph.Graph
	opts     Options
	children map[graph.Key][]line

	// What the legend has to explain, filled while drawing so that it lists
	// what the reader actually saw.
	sigils  map[rune]bool
	kinds   map[graph.Kind]bool
	states  map[graph.State]bool
	markers map[string]bool

	drawn map[graph.Key]bool
}

// plan works out what goes under each node: the walk's own tree from graph.Via,
// then a line for every other edge the tree would otherwise not show.
func (t *tree) plan() {
	viaEdges := make(map[graph.Edge]bool, len(t.g.Via))
	for child, e := range t.g.Via {
		parent := e.Other(child)
		if _, ok := t.g.Get(parent); !ok {
			continue
		}
		viaEdges[e] = true
		t.children[parent] = append(t.children[parent], newLine(child, e, false))
	}

	for _, e := range t.g.Edges {
		if viaEdges[e] || !t.g.Drawable(e) {
			continue
		}
		host, ok := t.hostFor(e)
		if !ok {
			continue
		}
		t.children[host] = append(t.children[host], newLine(e.Other(host), e, true))
	}

	for key, lines := range t.children {
		sort.Slice(lines, func(i, j int) bool { return t.less(lines[i], lines[j]) })
		t.children[key] = foldSameTarget(lines)
	}
}

// newLine makes the line that draws target under the other end of e.
func newLine(target graph.Key, e graph.Edge, backRef bool) line {
	l := line{target: target, backRef: backRef}
	if sigil, ok := sigilFor(e.Type, e.From == target); ok {
		l.sigils = []rune{sigil}
	}
	return l
}

// foldSameTarget merges the lines under one node that point at the same node
// into one row carrying every sigil. They are not necessarily next to each
// other, since they sort by sigil rather than by target.
func foldSameTarget(lines []line) []line {
	out := make([]line, 0, len(lines))
	at := make(map[graph.Key]int, len(lines))

	for _, l := range lines {
		i, seen := at[l.target]
		if !seen {
			at[l.target] = len(out)
			out = append(out, l)
			continue
		}
		out[i].sigils = append(out[i].sigils, l.sigils...)
		// The walk's own line claims the node, so a fold that holds one keeps
		// the subtree here instead of becoming a reference.
		out[i].backRef = out[i].backRef && l.backRef
	}
	return out
}

// hostFor picks which of an edge's two ends the extra line goes under: an end
// the walk already drew as the other's parent wins outright, and otherwise the
// deeper end does. Only an end that can carry a sigil is eligible, which is
// what puts a mention under the item it referred to.
func (t *tree) hostFor(e graph.Edge) (graph.Key, bool) {
	var best graph.Key
	var bestHop int
	found := false

	for _, host := range [2]graph.Key{e.From, e.To} {
		far := e.Other(host)
		if _, ok := sigilFor(e.Type, e.From == far); !ok {
			continue
		}
		node, ok := t.g.Get(host)
		if !ok {
			continue
		}
		if t.isViaParent(host, far) {
			return host, true
		}
		if !found || node.Hop > bestHop {
			best, bestHop, found = host, node.Hop, true
		}
	}
	return best, found
}

// isViaParent reports whether the walk reached child through parent.
func (t *tree) isViaParent(parent, child graph.Key) bool {
	via, ok := t.g.Via[child]
	return ok && via.Other(child) == parent
}

// less orders children by sigil, then the root's own repository by number,
// then other repositories by owner, repository and number.
func (t *tree) less(a, b line) bool {
	if a.rank() != b.rank() {
		return a.rank() < b.rank()
	}
	homeA := a.target.SameRepo(t.g.Root)
	homeB := b.target.SameRepo(t.g.Root)
	if homeA != homeB {
		return homeA
	}
	return a.target.Less(b.target)
}

// renderChildren draws everything under key. indent is what precedes the
// connector of each child.
func (t *tree) renderChildren(w *bufio.Writer, key graph.Key, indent string) {
	lines := t.children[key]
	for i, child := range lines {
		connector, continuation := connectorBranch, indentBar
		if i == len(lines)-1 {
			connector, continuation = connectorLast, indentBlank
		}
		t.renderLine(w, child, indent+connector, indent+continuation)
	}
}

// renderLine draws one line and, unless it is only a reference, what is under
// it. A node already drawn becomes a reference here, which is what stops a
// graph whose Via map is not a tree from recursing forever.
func (t *tree) renderLine(w *bufio.Writer, l line, prefix, childIndent string) {
	node, ok := t.g.Get(l.target)
	if !ok {
		return
	}

	backRef := l.backRef || t.drawn[l.target]
	if !backRef {
		// A reference does not claim the node: its own place in the tree may
		// still be below, and must not be demoted by a line drawn first.
		t.drawn[l.target] = true
	}

	w.WriteString(t.label(prefix+t.sigilColumn(l)+" ", node, backRef))
	w.WriteByte('\n')
	if backRef {
		return
	}
	t.renderChildren(w, l.target, childIndent)
}

// sigilColumn is the connector's last column, and notes for the legend what it
// drew. A line with no sigil holds the column open with a space, so that its
// shape lines up with its siblings'.
func (t *tree) sigilColumn(l line) string {
	if len(l.sigils) == 0 {
		return " "
	}
	for _, s := range l.sigils {
		t.sigils[s] = true
	}
	return string(l.sigils)
}
