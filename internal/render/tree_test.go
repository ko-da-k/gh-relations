package render_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/graph"
	"github.com/ko-da-k/gh-relations/internal/graph/graphtest"
	"github.com/ko-da-k/gh-relations/internal/render"
)

// title is the sample fixture's own title for an item, which a line in the
// drawn tree ends with.
func title(k graph.Key) string { return graphtest.Sample().Node(k).Title }

func drawSample(t *testing.T, root graph.Key, depth int, opts render.Options) string {
	t.Helper()
	return drawFixture(t, graphtest.Sample(), root, depth, opts)
}

func drawFixture(t *testing.T, fixture *graphtest.Fixture, root graph.Key, depth int, opts render.Options) string {
	t.Helper()
	walkOpts := graph.DefaultOptions()
	walkOpts.Depth = depth
	g, err := graph.Walk(context.Background(), graphtest.NewFetcher(fixture), root, walkOpts)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return draw(t, g, opts)
}

func draw(t *testing.T, g *graph.Graph, opts render.Options) string {
	t.Helper()
	var b strings.Builder
	if err := render.Tree(&b, g, opts); err != nil {
		t.Fatalf("Tree: %v", err)
	}
	return b.String()
}

func TestTreeDrawsTheWholeSampleGraph(t *testing.T) {
	// Given: the sample graph walked from pull request A#2 at the default
	// depth. It carries every edge kind, both cross-repository directions and
	// a cycle back to the root. Every item in it exists: running the extension
	// on ko-da-k/gh-relations#2 draws exactly this, which is checked by hand
	// rather than by a test, since a test would have to reach the network.
	want := `◇ #2 open: 【for Test】closes #3, mentions #5 and #7
├▸ ◯ #3 closed: 【for Test】closed by #2, sub of #4, blocked by #7
│  ├▴ ◯ #4 open: 【for Test】has 3 sub-issues: #3, #5, fixtures#8
│  │  ├▾ ◯ #5 not planned: 【for Test】sub of #4, closed by #9
│  │  │  ├▸ ◇ #9 closed: 【for Test】closes #5  +
│  │  │  └@ ◇ #2 open: 【for Test】closes #3, mentions #5 and #7  ↱
│  │  ├▾ ◯ gh-relations-fixtures#8 open: 【for Test】sub of gh-relations#4
│  │  └@ ◇ #8 draft: 【for Test】duplicate of #11 by comment, mentions #4
│  │     └= ◯ #11 open: 【for Test】#8 is marked a duplicate of this by comment  +
│  ├⊘ ◯ #7 closed: 【for Test】blocks #3, blocked by fixtures#7, closed by fixtures#9
│  │  ├▸ ◇ gh-relations-fixtures#9 merged: 【for Test】closes gh-relations#7
│  │  ├⊘ ◯ gh-relations-fixtures#7 open: 【for Test】blocks gh-relations#7, mentions gh-relations#2  ↱
│  │  └@ ◇ #2 open: 【for Test】closes #3, mentions #5 and #7  ↱
│  └@ ◯ #6 open: 【for Test】tracked in #10, fixtures#10 and fixtures#11, mentions #3
│     ├▪ ◯ #10 open: 【for Test】tracks #6, relates to fixtures#11
│     │  └~ ◯ gh-relations-fixtures#11 open: 【for Test】tracks gh-relations#6, relates to gh-relations#10  ↱
│     ├▪ ◯ gh-relations-fixtures#10 open: 【for Test】tracks gh-relations#6
│     └▪ ◯ gh-relations-fixtures#11 open: 【for Test】tracks gh-relations#6, relates to gh-relations#10
└@ ◯ gh-relations-fixtures#7 open: 【for Test】blocks gh-relations#7, mentions gh-relations#2

kind   ◯ issue  ◇ pull request
link   what this line is to its parent above:
       ▸ closes/closed by  ▴ parent of  ▾ sub of  ⊘ blocks  ~ relates to  = duplicate  ▪ tracks  @ mentions
mark   + not expanded  ↱ drawn above
`

	// When
	got := drawSample(t, graphtest.A(2), 4, render.Options{Legend: true})

	// Then
	if got != want {
		t.Errorf("tree mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestTreeRootKindDoesNotChangeAnEdge(t *testing.T) {
	// Given: the closes edge between pull request A#2 and issue A#3, seen from
	// each end in turn
	fromPR := drawSample(t, graphtest.A(2), 1, render.Options{})
	fromIssue := drawSample(t, graphtest.A(3), 1, render.Options{})

	// When: each tree's first child line is read
	prChild := lineWith(fromPR, "◯ #3 ")
	issueChild := lineWith(fromIssue, "◇ #2 ")

	// Then: the same sigil both ways, with the shape saying which end is which
	if !strings.HasPrefix(prChild, "├▸ ◯ #3") {
		t.Errorf("under the pull request root: %q", prChild)
	}
	if !strings.HasPrefix(issueChild, "├▸ ◇ #2") {
		t.Errorf("under the issue root: %q", issueChild)
	}
}

func TestTreeRootComesFirst(t *testing.T) {
	// Given / When
	got := drawSample(t, graphtest.A(2), 2, render.Options{})

	// Then
	first, _, _ := strings.Cut(got, "\n")
	if first != "◇ #2 open: "+title(graphtest.A(2)) {
		t.Errorf("first line = %q, want the root with no connector", first)
	}
}

func TestTreeReferences(t *testing.T) {
	// Given: a tree rooted in ko-da-k/gh-relations
	got := drawSample(t, graphtest.A(2), 4, render.Options{})

	tests := []struct {
		name string
		// Then: how a node is written
		want string
	}{
		{"a node of the root's repository is shortened", "◯ #3"},
		{"a sibling repository of the same owner keeps the repository and drops the owner", "◯ gh-relations-fixtures#8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(got, tt.want) {
				t.Errorf("tree does not contain %q", tt.want)
			}
		})
	}

	t.Run("another owner is qualified", func(t *testing.T) {
		// Given: the one fixture with a node outside the owner. Sample holds
		// only items that exist, and no item outside this owner can.
		got := drawFixture(t, graphtest.SampleWithForeign(), graphtest.A(2), 4, render.Options{})

		// Then
		if !strings.Contains(got, "◯ another_owner/gh-relations#71") {
			t.Errorf("tree does not qualify a node outside the owner:\n%s", got)
		}
	})
}

func TestTreeMarkers(t *testing.T) {
	t.Run("a node a limit stopped from expanding ends in a plus", func(t *testing.T) {
		// Given: a depth of 1, so every neighbour is drawn and none is expanded
		// When
		got := drawSample(t, graphtest.A(3), 1, render.Options{})

		// Then
		for _, line := range strings.Split(strings.TrimSpace(got), "\n")[1:] {
			if !strings.HasSuffix(line, "  +") {
				t.Errorf("line should end in the unexpanded marker: %q", line)
			}
		}
	})

	t.Run("a node drawn elsewhere becomes one line ending in an arrow", func(t *testing.T) {
		// Given: the cycle, where A#5 names the root that reached it
		// When
		got := drawSample(t, graphtest.A(2), 4, render.Options{})

		// Then: the root appears again as a single line, not as a second subtree
		back := countLines(got, "@ ◇ #2 open: "+title(graphtest.A(2))+"  ↱")
		if back != 2 {
			t.Errorf("found %d reference lines back to the root, want 2", back)
		}
		if roots := countLines(got, "◇ #2 open: "+title(graphtest.A(2))); roots != 3 {
			t.Errorf("the root is written %d times, want once as the root and twice as a reference", roots)
		}
	})

	t.Run("the edge a node arrived on is not drawn again under it", func(t *testing.T) {
		// Given: A#3 arrived on the closes edge from the root
		// When
		got := drawSample(t, graphtest.A(2), 4, render.Options{})

		// Then: nothing under A#3 points back at the root on that same edge
		if strings.Contains(got, "▸ ◇ #2") {
			t.Error("the closes edge is drawn a second time underneath the node it introduced")
		}
	})
}

func TestTreeOrdersChildren(t *testing.T) {
	// Given: one node with a line of every kind under it, added in an order
	// that is not the drawing order
	root := graphtest.A(1)
	g := graph.NewGraph(root)
	g.Add(graph.Node{Key: root, Kind: graph.KindIssue, Title: "root"})

	type child struct {
		key  graph.Key
		edge graph.Edge
	}
	children := []child{
		{graphtest.B(8), graph.Edge{From: graphtest.B(8), To: root, Type: graph.EdgeMention}},
		{graphtest.A(9), graph.Edge{From: graphtest.A(9), To: root, Type: graph.EdgeTracked}},
		{graphtest.A(8), graph.Edge{From: root, To: graphtest.A(8), Type: graph.EdgeRelates}},
		{graphtest.A(7), graph.Edge{From: graphtest.A(7), To: root, Type: graph.EdgeBlocks}},
		{graphtest.A(6), graph.Edge{From: graphtest.A(6), To: root, Type: graph.EdgeParent}},
		{graphtest.A(5), graph.Edge{From: graphtest.A(5), To: root, Type: graph.EdgeCloses}},
		{graphtest.A(4), graph.Edge{From: root, To: graphtest.A(4), Type: graph.EdgeParent}},
		{graphtest.A(3), graph.Edge{From: graphtest.A(3), To: root, Type: graph.EdgeMention}},
	}
	for _, c := range children {
		g.Add(graph.Node{Key: c.key, Kind: graph.KindIssue, Title: "child", Hop: 1, Stop: graph.StopDepth})
		g.AddEdge(c.edge, true)
		g.Via[c.key] = c.edge
	}

	// When
	got := draw(t, g, render.Options{})

	// Then: declared relations in the order of the edge table, both directions
	// of a kind together, mentions last; inside that the root's own repository
	// by number, then other repositories
	want := []string{
		"├▸ ◯ #5", // closes
		"├▴ ◯ #6", // parent of the root
		"├▾ ◯ #4", // sub-issue of the root
		"├⊘ ◯ #7", // blocker of the root
		"├~ ◯ #8", // relates
		"├▪ ◯ #9", // tracks the root
		"├@ ◯ #3", // mention, same repository first
		"└@ ◯ gh-relations-fixtures#8",
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")[1:]
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), got)
	}
	for i, prefix := range want {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Errorf("line %d = %q, want it to start with %q", i, lines[i], prefix)
		}
	}
}

func TestTreeIndent(t *testing.T) {
	// Given: a tree several levels deep
	got := drawSample(t, graphtest.A(2), 4, render.Options{})

	t.Run("one level is three columns, whatever the kind is called", func(t *testing.T) {
		// When: the indent of each level is measured
		// Then: every continuation is one of exactly two three-column units
		for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
			rest := line
			for strings.HasPrefix(rest, indentBar) || strings.HasPrefix(rest, indentBlank) {
				rest = rest[len(indentBar):]
			}
			if strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "│") {
				t.Errorf("line has an indent that is not a whole number of units: %q", line)
			}
		}
	})

	t.Run("all siblings share one indent string", func(t *testing.T) {
		// This is the condition under which a terminal that draws Ambiguous
		// glyphs full width still lines the bars up: every sibling shifts by
		// the same amount.
		if strings.Contains(got, "│ ├") || strings.Contains(got, "│   ├") {
			t.Error("found an indent unit of a different width")
		}
	})
}

const (
	indentBar   = "│  "
	indentBlank = "   "
)

func lineWith(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func countLines(s, needle string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			n++
		}
	}
	return n
}

func TestOneNodeGetsOneLineHoweverManyRelationsReachIt(t *testing.T) {
	t.Run("a parent is not written out again under its own child", func(t *testing.T) {
		// Given: a root with one child, which is also related to the root and
		// tracks it. All three links join the same pair, and the walk already
		// drew that pair as parent and child.
		root, child := graphtest.A(1), graphtest.B(2)
		g := graph.NewGraph(root)
		g.Add(graph.Node{Key: root, Kind: graph.KindIssue, Title: "root"})
		g.Add(graph.Node{Key: child, Kind: graph.KindIssue, Title: "child", Hop: 1})

		blocked := graph.Edge{From: root, To: child, Type: graph.EdgeBlocks}
		g.AddEdge(blocked, true)
		g.Via[child] = blocked
		g.AddEdge(graph.Edge{From: root, To: child, Type: graph.EdgeRelates}, true)
		g.AddEdge(graph.Edge{From: child, To: root, Type: graph.EdgeTracked}, true)

		// When
		got := draw(t, g, render.Options{})

		// Then: the extra links land on the line that already exists, with the
		// sigils in the order of the edge table, and the root is not met a
		// second time under its own child
		want := "◯ #1 open: root\n" +
			"└⊚~▪ ◯ gh-relations-fixtures#2 open: child\n"
		if got != want {
			t.Errorf("tree =\n%s\nwant\n%s", got, want)
		}
		if n := strings.Count(got, "#1"); n != 1 {
			t.Errorf("the root is written %d times, want once:\n%s", n, got)
		}
	})

	t.Run("a folded line that the walk came through stays expanded", func(t *testing.T) {
		// Given: a child the walk reached through a closes edge, which also
		// mentions its parent. A mention is only visible from the mentioned
		// side, so the only end this second line can sit under is the parent --
		// the same end the walk's own line for the child sits under.
		root, child, grandchild := graphtest.A(1), graphtest.A(2), graphtest.A(3)
		g := graph.NewGraph(root)
		g.Add(graph.Node{Key: root, Kind: graph.KindIssue, Title: "root"})
		g.Add(graph.Node{Key: child, Kind: graph.KindPullRequest, Title: "child", Hop: 1})
		g.Add(graph.Node{Key: grandchild, Kind: graph.KindIssue, Title: "grandchild", Hop: 2})

		closes := graph.Edge{From: child, To: root, Type: graph.EdgeCloses}
		g.AddEdge(closes, true)
		g.Via[child] = closes
		g.AddEdge(graph.Edge{From: child, To: root, Type: graph.EdgeMention}, true)

		sub := graph.Edge{From: grandchild, To: child, Type: graph.EdgeParent}
		g.AddEdge(sub, true)
		g.Via[grandchild] = sub

		// When
		got := draw(t, g, render.Options{})

		// Then: the subtree is still under the folded line, and the line is not
		// marked as drawn elsewhere
		want := "◯ #1 open: root\n" +
			"└▸@ ◇ #2 open: child\n" +
			"   └▴ ◯ #3 open: grandchild\n"
		if got != want {
			t.Errorf("tree =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("references are folded by what they point at, not by where they sort", func(t *testing.T) {
		// Given: three children of the root, where the first is related to the
		// second, tracks the second, and is duplicated by the third. None of
		// those three links joins a pair the walk drew as parent and child, so
		// all three are references -- and the two that point at the second
		// child do not end up next to each other, because lines sort by link
		// kind and the duplicate falls between them.
		root := graphtest.A(1)
		a, b, c := graphtest.A(2), graphtest.A(3), graphtest.A(4)
		g := graph.NewGraph(root)
		g.Add(graph.Node{Key: root, Kind: graph.KindIssue, Title: "root"})
		for key, title := range map[graph.Key]string{a: "a", b: "b", c: "c"} {
			g.Add(graph.Node{Key: key, Kind: graph.KindIssue, Title: title, Hop: 1})
			e := graph.Edge{From: key, To: root, Type: graph.EdgeCloses}
			g.AddEdge(e, true)
			g.Via[key] = e
		}
		g.AddEdge(graph.Edge{From: a, To: b, Type: graph.EdgeRelates}, true)
		g.AddEdge(graph.Edge{From: a, To: b, Type: graph.EdgeTracked}, true)
		g.AddEdge(graph.Edge{From: a, To: c, Type: graph.EdgeDuplicate}, true)

		// When
		got := draw(t, g, render.Options{})

		// Then: one folded reference for #3, the duplicate still its own line,
		// and both #3 and #4 still expanded in their own right further down
		want := "◯ #1 open: root\n" +
			"├▸ ◯ #2 open: a\n" +
			"│  ├~▫ ◯ #3 open: b  ↱\n" +
			"│  └= ◯ #4 open: c  ↱\n" +
			"├▸ ◯ #3 open: b\n" +
			"└▸ ◯ #4 open: c\n"
		if got != want {
			t.Errorf("tree =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("a mention the parent itself wrote is the one link that cannot be folded", func(t *testing.T) {
		// Given: a child the walk reached through a closes edge, where the
		// parent is the one that wrote a bare reference to the child. The
		// parent end of a mention carries no sigil -- there is none for "is
		// mentioned by" -- so this line cannot go beside the walk's own line,
		// and the parent is written out again underneath its child after all.
		//
		// PreferStructural drops every mention that runs beside a declared
		// relation, so a walk never builds this shape; the graph is handed to
		// the renderer directly, because the rule being checked is its own.
		root, child := graphtest.A(1), graphtest.A(2)
		g := graph.NewGraph(root)
		g.Add(graph.Node{Key: root, Kind: graph.KindIssue, Title: "root"})
		g.Add(graph.Node{Key: child, Kind: graph.KindIssue, Title: "child", Hop: 1})

		closes := graph.Edge{From: child, To: root, Type: graph.EdgeCloses}
		g.AddEdge(closes, true)
		g.Via[child] = closes
		g.AddEdge(graph.Edge{From: root, To: child, Type: graph.EdgeMention}, true)

		// When
		got := draw(t, g, render.Options{})

		// Then
		want := "◯ #1 open: root\n" +
			"└▸ ◯ #2 open: child\n" +
			"   └@ ◯ #1 open: root  ↱\n"
		if got != want {
			t.Errorf("tree =\n%s\nwant\n%s", got, want)
		}
	})

	t.Run("every sigil of a folded line reaches the legend", func(t *testing.T) {
		// Given: the folded line is the only place "~" and "▪" appear, so a
		// legend built from the glyphs that were drawn has to see past the
		// first sigil of a line
		root, child := graphtest.A(1), graphtest.A(2)
		g := graph.NewGraph(root)
		g.Add(graph.Node{Key: root, Kind: graph.KindIssue, Title: "root"})
		g.Add(graph.Node{Key: child, Kind: graph.KindIssue, Title: "child", Hop: 1})

		blocked := graph.Edge{From: root, To: child, Type: graph.EdgeBlocks}
		g.AddEdge(blocked, true)
		g.Via[child] = blocked
		g.AddEdge(graph.Edge{From: root, To: child, Type: graph.EdgeRelates}, true)
		g.AddEdge(graph.Edge{From: child, To: root, Type: graph.EdgeTracked}, true)

		// When
		got := draw(t, g, render.Options{Legend: true})

		// Then
		if want := "⊚ blocked by  ~ relates to  ▪ tracks"; !strings.Contains(got, want) {
			t.Errorf("tree =\n%s\nwant a legend row %q", got, want)
		}
	})
}
