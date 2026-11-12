package graph_test

import (
	"context"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/graph"
	"github.com/ko-da-k/gh-relations/internal/graph/graphtest"
)

func walk(t *testing.T, root graph.Key, opts graph.Options) (*graph.Graph, *graphtest.Fetcher) {
	t.Helper()
	return walkFixture(t, graphtest.Sample(), root, opts)
}

// walkForeign uses the fixture that has a node under another owner, which is
// the only thing --scope all can be told apart from --scope org by.
func walkForeign(t *testing.T, root graph.Key, opts graph.Options) (*graph.Graph, *graphtest.Fetcher) {
	t.Helper()
	return walkFixture(t, graphtest.SampleWithForeign(), root, opts)
}

func walkFixture(t *testing.T, fixture *graphtest.Fixture, root graph.Key, opts graph.Options) (*graph.Graph, *graphtest.Fetcher) {
	t.Helper()
	f := graphtest.NewFetcher(fixture)
	g, err := graph.Walk(context.Background(), f, root, opts)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return g, f
}

func TestWalkDepth(t *testing.T) {
	tests := []struct {
		name string
		// Given: a depth, walking the sample graph from pull request A#2
		depth int
		// Then: what the walk spends and what it brings back
		wantRequests int
		wantNodes    int
		wantEdges    int
	}{
		// The root still has to be fetched to be drawn, but its links are
		// already past the depth, so none of them is read.
		{"zero is the root alone", 0, 1, 1, 0},
		// One request: the root's neighbours are admitted from the root's own
		// response and never fetched themselves.
		{"one expands the root only", 1, 1, 3, 2},
		{"two reaches the second rank", 2, 2, 6, 6},
		{"three reaches the third", 3, 3, 13, 14},
		// The default. The sample graph fills out here.
		{"four covers the sample graph", 4, 4, 15, 18},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			opts := graph.DefaultOptions()
			opts.Depth = tt.depth

			// When
			g, _ := walk(t, graphtest.A(2), opts)

			// Then
			if g.Requests != tt.wantRequests {
				t.Errorf("Requests = %d, want %d", g.Requests, tt.wantRequests)
			}
			if g.Len() != tt.wantNodes {
				t.Errorf("nodes = %d, want %d", g.Len(), tt.wantNodes)
			}
			if len(g.Edges) != tt.wantEdges {
				t.Errorf("edges = %d, want %d", len(g.Edges), tt.wantEdges)
			}
		})
	}
}

func TestWalkSpendsOneRequestPerLevel(t *testing.T) {
	t.Run("requests never exceed max(1, depth)", func(t *testing.T) {
		// Given: a depth far past what the sample graph can fill
		opts := graph.DefaultOptions()
		opts.Depth = 9

		// When
		g, f := walk(t, graphtest.A(2), opts)

		// Then: the walk stops when it runs out of unexpanded nodes, not at the depth
		if g.Requests > 9 {
			t.Errorf("Requests = %d, want at most 9", g.Requests)
		}
		if f.Levels+1 != g.Requests {
			t.Errorf("Requests = %d, want one per level plus the root (%d)", g.Requests, f.Levels+1)
		}
	})

	t.Run("the deepest rank is admitted but never fetched", func(t *testing.T) {
		// Given: a depth of 2, so nodes at hop 2 are drawn and not expanded
		opts := graph.DefaultOptions()
		opts.Depth = 2

		// When
		g, f := walk(t, graphtest.A(2), opts)

		// Then: two requests, not three, and the hop-2 nodes carry the reason
		if g.Requests != 2 {
			t.Errorf("Requests = %d, want 2", g.Requests)
		}
		if f.WasFetched(graphtest.A(4)) {
			t.Error("a node at the depth should not be fetched")
		}
		n, ok := g.Get(graphtest.A(4))
		if !ok {
			t.Fatal("the node at the depth should still be in the graph")
		}
		if n.Stop != graph.StopDepth {
			t.Errorf("Stop = %v, want depth", n.Stop)
		}
		if n.Title == "" {
			t.Error("a node that is never fetched is still drawn, so it needs its title")
		}
	})

	t.Run("each node is fetched at most once", func(t *testing.T) {
		// Given: the default walk, where several paths lead to the same nodes
		// When
		_, f := walk(t, graphtest.A(2), graph.DefaultOptions())

		// Then
		for _, k := range []graph.Key{graphtest.A(4), graphtest.A(3), graphtest.A(7), graphtest.A(2)} {
			if got := f.FetchCount(k); got > 1 {
				t.Errorf("%v was fetched %d times, want at most 1", k, got)
			}
		}
	})

	t.Run("a level with more ids than one request takes is split", func(t *testing.T) {
		// Given: a fetcher that can only carry two ids per request
		f := graphtest.NewFetcher(graphtest.Sample())
		f.IDsPerRequest = 2

		// When
		g, err := graph.Walk(context.Background(), f, graphtest.A(2), graph.DefaultOptions())

		// Then: the same levels, more requests, and the graph is unchanged
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}
		if g.Requests <= f.Levels+1 {
			t.Errorf("Requests = %d, want more than one per level (%d levels)", g.Requests, f.Levels)
		}
		if g.Len() != 15 {
			t.Errorf("nodes = %d, want the same 15 as an unsplit walk", g.Len())
		}
	})
}

func TestWalkHandlesCycles(t *testing.T) {
	// A#2 reaches A#5 through A#3 and A#4, and A#5 reports that A#2
	// referred to it, which closes the loop.
	t.Run("a cycle adds an edge and no requests", func(t *testing.T) {
		// Given / When
		g, f := walk(t, graphtest.A(2), graph.DefaultOptions())

		// Then: the edge back to the root is there
		var found bool
		for _, e := range g.Edges {
			if e.From == graphtest.A(2) && e.To == graphtest.A(5) && e.Type == graph.EdgeMention {
				found = true
			}
		}
		if !found {
			t.Error("the edge closing the cycle is missing")
		}

		// and the root was fetched once, not again from the far end
		if got := f.FetchCount(graphtest.A(2)); got != 1 {
			t.Errorf("the root was fetched %d times, want 1", got)
		}
	})

	t.Run("the node keeps the edge it was discovered on", func(t *testing.T) {
		// Given / When
		g, _ := walk(t, graphtest.A(2), graph.DefaultOptions())

		// Then: A#5 arrived as a sub-issue of A#4, not on the mention that
		// was recorded later
		via, ok := g.Via[graphtest.A(5)]
		if !ok {
			t.Fatal("A#5 has no discovery edge")
		}
		if via.Type != graph.EdgeParent || via.From != graphtest.A(4) {
			t.Errorf("Via = %v, want the parent edge from A#4", via)
		}
	})

	t.Run("a node reached by several paths is one node", func(t *testing.T) {
		// Given: A#7 is reported by A#3, by B#7 and by B#9
		// When
		g, _ := walk(t, graphtest.A(2), graph.DefaultOptions())

		// Then
		n, ok := g.Get(graphtest.A(7))
		if !ok {
			t.Fatal("A#7 is missing")
		}
		if n.Hop != 2 {
			t.Errorf("Hop = %d, want the shortest path 2", n.Hop)
		}
		seen := 0
		for _, k := range g.Order {
			if k == graphtest.A(7) {
				seen++
			}
		}
		if seen != 1 {
			t.Errorf("A#7 appears %d times in Order, want 1", seen)
		}
	})

	t.Run("the same number in two repositories stays two nodes", func(t *testing.T) {
		// Given: the two repositories were filled independently, so both hold
		// a #7, and the two are different items
		// When
		g, _ := walk(t, graphtest.A(2), graph.DefaultOptions())

		// Then
		home, ok := g.Get(graphtest.A(7))
		if !ok {
			t.Fatal("ko-da-k/gh-relations#7 is missing")
		}
		sibling, ok := g.Get(graphtest.B(7))
		if !ok {
			t.Fatal("ko-da-k/gh-relations-fixtures#7 is missing")
		}
		if home.Title == sibling.Title {
			t.Errorf("both #7s came back as %q, so the number alone was taken for the identity", home.Title)
		}
	})
}

func TestWalkLearnsADirectionLater(t *testing.T) {
	// Given: #24's timeline reports a duplicate event, which carries no
	// direction, and #26 reports duplicateOf, which does. The walk sees the
	// undirected one first, because that is the end it starts from.
	fixture := &graphtest.Fixture{
		Nodes: map[graph.Key]graph.Node{
			graphtest.A(11): {Key: graphtest.A(11), Kind: graph.KindIssue, Title: "the one that stays"},
			graphtest.A(53): {Key: graphtest.A(53), Kind: graph.KindIssue, Title: "filed twice by mistake"},
		},
		Relations: []graphtest.Relation{graphtest.DuplicateOf(graphtest.A(53), graphtest.A(11))},
	}

	// When
	g, err := graph.Walk(context.Background(), graphtest.NewFetcher(fixture), graphtest.A(11), graph.DefaultOptions())
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Then: one edge, pointing the way duplicateOf said
	want := graph.Edge{From: graphtest.A(53), To: graphtest.A(11), Type: graph.EdgeDuplicate}
	if len(g.Edges) != 1 || g.Edges[0] != want {
		t.Errorf("edges = %v, want just %v", g.Edges, want)
	}

	// and the node that arrived on the earlier form of that edge follows it,
	// or the tree would point at an edge the graph no longer holds
	if got := g.Via[graphtest.A(53)]; got != want {
		t.Errorf("Via = %v, want %v", got, want)
	}
}

func TestWalkScope(t *testing.T) {
	// G#71 sits at hop 4 under the default depth, where nothing is expanded, so
	// the scope only shows at a depth that would otherwise reach past it.
	const depth = 5

	tests := []struct {
		name string
		// Given: a scope
		scope graph.Scope
		// Then: whether each of the three repositories is ever fetched
		wantSiblingFetched bool
		wantForeignFetched bool
	}{
		{"repo stays in the root's repository", graph.ScopeRepo, false, false},
		{"org covers another repository of the same owner", graph.ScopeOrg, true, false},
		{"all ignores the owner", graph.ScopeAll, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			opts := graph.DefaultOptions()
			opts.Depth = depth
			opts.Scope = tt.scope

			// When
			g, f := walkForeign(t, graphtest.A(2), opts)

			// Then: out-of-scope nodes are drawn either way
			for _, k := range []graph.Key{graphtest.B(7), graphtest.G(71)} {
				if _, ok := g.Get(k); !ok {
					t.Errorf("%v should be admitted whatever the scope", k)
				}
			}
			if got := f.WasFetched(graphtest.B(7)); got != tt.wantSiblingFetched {
				t.Errorf("ko-da-k/gh-relations-fixtures fetched = %v, want %v", got, tt.wantSiblingFetched)
			}
			if got := f.WasFetched(graphtest.G(71)); got != tt.wantForeignFetched {
				t.Errorf("another_owner/gh-relations fetched = %v, want %v", got, tt.wantForeignFetched)
			}
		})
	}

	t.Run("a node stopped by the scope says so", func(t *testing.T) {
		// Given: the root's own repository only
		opts := graph.DefaultOptions()
		opts.Scope = graph.ScopeRepo

		// When
		g, _ := walk(t, graphtest.A(2), opts)

		// Then
		n, ok := g.Get(graphtest.B(7))
		if !ok {
			t.Fatal("ko-da-k/gh-relations-fixtures#1 is missing")
		}
		if n.Stop != graph.StopScope {
			t.Errorf("Stop = %v, want scope", n.Stop)
		}
	})

	t.Run("the scope compares owners, not account types", func(t *testing.T) {
		// Given: ko-da-k/gh-relations and ko-da-k/gh-relations-fixtures share an owner; another_owner/gh-relations does not
		root := graphtest.A(2)

		// When / Then
		if !graphtest.B(7).SameOwner(root) {
			t.Error("ko-da-k/gh-relations-fixtures should count as the same owner as ko-da-k/gh-relations")
		}
		if graphtest.G(71).SameOwner(root) {
			t.Error("another_owner/gh-relations should not count as the same owner")
		}
	})
}

func TestWalkPrefersStructuralLinks(t *testing.T) {
	t.Run("by default the relation hides the cross reference it caused", func(t *testing.T) {
		// Given / When
		g, _ := walk(t, graphtest.A(2), graph.DefaultOptions())

		// Then
		for _, e := range g.Edges {
			if e.Type == graph.EdgeMention && e.From == graphtest.A(2) && e.To == graphtest.A(3) {
				t.Error("the mention behind the closes edge should have been dropped")
			}
		}
	})
}

func TestWalkMaxNodes(t *testing.T) {
	t.Run("the graph stops at the cap and says it was cut", func(t *testing.T) {
		// Given: room for eight nodes
		opts := graph.DefaultOptions()
		opts.MaxNodes = 8

		// When
		g, _ := walk(t, graphtest.A(2), opts)

		// Then
		if g.Len() != 8 {
			t.Errorf("nodes = %d, want 8", g.Len())
		}
		if !g.Truncated {
			t.Error("Truncated should be set when the cap kept nodes out")
		}
	})

	t.Run("an edge whose far end was refused stays in the graph but is not drawable", func(t *testing.T) {
		// Given / When
		opts := graph.DefaultOptions()
		opts.MaxNodes = 8
		g, _ := walk(t, graphtest.A(2), opts)

		// Then: --max-nodes is the one bound that can record an edge and then
		// refuse the node at its far end
		dangling := 0
		for _, e := range g.Edges {
			if !g.Drawable(e) {
				dangling++
			}
		}
		if dangling != 5 {
			t.Errorf("edges with an end that was never admitted = %d, want 5", dangling)
		}
	})

	t.Run("depth and scope alone leave every edge drawable", func(t *testing.T) {
		// Given: the default walk, bounded by depth and scope only
		for _, scope := range []graph.Scope{graph.ScopeRepo, graph.ScopeOrg, graph.ScopeAll} {
			opts := graph.DefaultOptions()
			opts.Scope = scope

			// When
			g, _ := walk(t, graphtest.A(2), opts)

			// Then: a node one hop past the depth is never observed, and an
			// out-of-scope node is admitted before it is left unexpanded
			for _, e := range g.Edges {
				if !g.Drawable(e) {
					t.Errorf("scope %v left %v with an end that is not a node", scope, e)
				}
			}
			if g.Truncated {
				t.Errorf("scope %v should not mark the graph as cut", scope)
			}
		}
	})
}

func TestWalkRoot(t *testing.T) {
	t.Run("the same link appears whichever end is the root", func(t *testing.T) {
		// Given: the closes edge between pull request A#2 and issue A#3
		want := graph.Edge{From: graphtest.A(2), To: graphtest.A(3), Type: graph.EdgeCloses}

		// When: the walk starts from each end in turn
		fromPR, _ := walk(t, graphtest.A(2), graph.DefaultOptions())
		fromIssue, _ := walk(t, graphtest.A(3), graph.DefaultOptions())

		// Then: one edge, same direction, same kind, in both graphs
		for _, g := range []*graph.Graph{fromPR, fromIssue} {
			found := 0
			for _, e := range g.Edges {
				if e.Touches(graphtest.A(2)) && e.Touches(graphtest.A(3)) {
					if e != want {
						t.Errorf("edge = %v, want %v", e, want)
					}
					found++
				}
			}
			if found != 1 {
				t.Errorf("found %d edges between A#2 and A#3, want 1", found)
			}
		}
	})

	t.Run("a pull request root reports fewer kinds than an issue root", func(t *testing.T) {
		// Given: a pull request has no parent, subIssues or blockedBy of its own
		opts := graph.DefaultOptions()
		opts.Depth = 1

		// When
		fromPR, _ := walk(t, graphtest.A(2), opts)
		fromIssue, _ := walk(t, graphtest.A(3), opts)

		// Then: the issue's first rank is wider
		if len(fromIssue.Edges) <= len(fromPR.Edges) {
			t.Errorf("issue root has %d edges, pull request root %d; want the issue to see more",
				len(fromIssue.Edges), len(fromPR.Edges))
		}
	})

	t.Run("a root nobody referred to comes back alone", func(t *testing.T) {
		// Given: an item whose only reference is one it wrote itself, in prose.
		// GitHub reports that only from the mentioned side, so the root's own
		// response has nothing in it.
		fixture := &graphtest.Fixture{
			Nodes: map[graph.Key]graph.Node{
				graphtest.A(1): {Key: graphtest.A(1), Kind: graph.KindIssue, Title: "writes a bare #2"},
				graphtest.A(2): {Key: graphtest.A(2), Kind: graph.KindIssue, Title: "never answers"},
			},
			Relations: []graphtest.Relation{graphtest.Mentions(graphtest.A(1), graphtest.A(2))},
		}

		// When
		g, err := graph.Walk(context.Background(), graphtest.NewFetcher(fixture), graphtest.A(1), graph.DefaultOptions())

		// Then: the root stays alone, which is the blind spot and not a bug
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}
		if g.Len() != 1 {
			t.Errorf("nodes = %d, want the root alone", g.Len())
		}
	})

	t.Run("a root that does not exist is an error, not an empty graph", func(t *testing.T) {
		// Given: a number the fixture has never heard of
		f := graphtest.NewFetcher(graphtest.Sample())

		// When
		_, err := graph.Walk(context.Background(), f, graphtest.A(999), graph.DefaultOptions())

		// Then
		if err == nil {
			t.Fatal("want an error for a root that was not found")
		}
	})
}

func TestWalkAdoptsTheNameTheAPIAnswersWith(t *testing.T) {
	// Given: a root written the way the user typed it, which is not the
	// repository's canonical name -- another case, or the old name of a
	// repository that has since been renamed
	asTyped := graph.Key{Owner: "KO-DA-K", Repo: "GH-Relations", Number: 2}
	f := graphtest.NewFetcher(graphtest.SampleWithForeign())
	f.RootAlias = map[graph.Key]graph.Key{asTyped: graphtest.A(2)}

	// When
	g, err := graph.Walk(context.Background(), f, asTyped, graph.DefaultOptions())
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}

	// Then: the graph takes the canonical name, so the root is a node in it.
	// Everything downstream compares against Root -- the scope, the short
	// "#14" form, the renderer's starting point -- so a mismatch would leave
	// the whole tree unreachable.
	if g.Root != graphtest.A(2) {
		t.Errorf("Root = %v, want the name the API answered with %v", g.Root, graphtest.A(2))
	}
	if _, ok := g.Get(g.Root); !ok {
		t.Fatal("the root is not a node in its own graph")
	}

	// and the scope still works, which it would not if Root kept the owner the
	// user typed
	if _, ok := g.Get(graphtest.B(7)); !ok {
		t.Error("a sibling repository of the canonical owner should have been admitted")
	}
	if f.WasFetched(graphtest.G(71)) {
		t.Error("--scope org should still exclude another owner")
	}
}

func TestOptionsValidate(t *testing.T) {
	tests := []struct {
		name string
		// Given: options
		mutate func(*graph.Options)
		// Then: whether they are usable
		wantErr bool
	}{
		{"the defaults are valid", func(*graph.Options) {}, false},
		{"depth zero is valid and means the root alone", func(o *graph.Options) { o.Depth = 0 }, false},
		{"a negative depth is rejected", func(o *graph.Options) { o.Depth = -1 }, true},
		{"a cap below one would admit nothing", func(o *graph.Options) { o.MaxNodes = 0 }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			opts := graph.DefaultOptions()
			tt.mutate(&opts)

			// When
			err := opts.Validate()

			// Then
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
