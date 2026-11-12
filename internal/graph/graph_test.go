package graph_test

import (
	"testing"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

var (
	alpha43 = graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 43}
	alpha42 = graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 42}
	alpha41 = graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 41}
	beta62  = graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: 62}
)

func TestGraphAdd(t *testing.T) {
	t.Run("a node is added once and keeps its place", func(t *testing.T) {
		// Given: an empty graph
		g := graph.NewGraph(alpha41)

		// When: the same node arrives twice
		g.Add(graph.Node{Key: alpha42, Title: "some issue title"})
		g.Add(graph.Node{Key: alpha42, Title: "some issue title"})

		// Then: it is one node, listed once
		if got := g.Len(); got != 1 {
			t.Errorf("Len() = %d, want 1", got)
		}
		if got := len(g.Order); got != 1 {
			t.Errorf("Order has %d entries, want 1", got)
		}
	})

	t.Run("a later response fills in display fields the first one lacked", func(t *testing.T) {
		// Given: a node first named by a response that carried no title
		g := graph.NewGraph(alpha41)
		g.Add(graph.Node{Key: alpha42, Hop: 1})

		// When: a fuller description of it arrives
		g.Add(graph.Node{Key: alpha42, Title: "some issue title", URL: "https://example.test/42", ID: "I_42", Hop: 3})

		// Then: the gaps are filled but the hop count of the shortest path stands
		n, ok := g.Get(alpha42)
		if !ok {
			t.Fatal("node is missing")
		}
		if n.Title != "some issue title" || n.URL != "https://example.test/42" || n.ID != "I_42" {
			t.Errorf("display fields were not filled in: %+v", n)
		}
		if n.Hop != 1 {
			t.Errorf("Hop = %d, want the first (shortest) hop count 1", n.Hop)
		}
	})

	t.Run("the same number in another repository is another node", func(t *testing.T) {
		// Given: an empty graph
		g := graph.NewGraph(alpha41)

		// When: two items that share a number but not a repository arrive
		g.Add(graph.Node{Key: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 2}})
		g.Add(graph.Node{Key: beta62})

		// Then
		if got := g.Len(); got != 2 {
			t.Errorf("Len() = %d, want 2", got)
		}
	})
}

func TestGraphAddEdge(t *testing.T) {
	t.Run("the same pair and kind is recorded once whichever end it arrives from", func(t *testing.T) {
		// Given: a graph
		g := graph.NewGraph(alpha41)

		// When: a closes edge arrives from the PR side and then from the issue side
		g.AddEdge(graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses}, true)
		g.AddEdge(graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses}, true)

		// Then
		if got := len(g.Edges); got != 1 {
			t.Errorf("recorded %d edges, want 1", got)
		}
	})

	t.Run("a known direction replaces an unknown one for the same pair", func(t *testing.T) {
		// Given: a duplicate seen first as a timeline event, which carries no direction,
		// so it was stored in key order
		g := graph.NewGraph(alpha41)
		g.AddEdge(graph.Edge{From: alpha43, To: alpha42, Type: graph.EdgeDuplicate}, false)

		// When: duplicateOf later says which one is the duplicate
		g.AddEdge(graph.Edge{From: alpha42, To: alpha43, Type: graph.EdgeDuplicate}, true)

		// Then: still one edge, now pointing the way the API said
		if got := len(g.Edges); got != 1 {
			t.Fatalf("recorded %d edges, want 1", got)
		}
		if g.Edges[0].From != alpha42 || g.Edges[0].To != alpha43 {
			t.Errorf("edge = %v, want ko-da-k/gh-relations#42 -> ko-da-k/gh-relations#43", g.Edges[0])
		}
	})

	t.Run("an unknown direction does not overwrite a known one", func(t *testing.T) {
		// Given: a duplicate whose direction is already known
		g := graph.NewGraph(alpha41)
		g.AddEdge(graph.Edge{From: alpha42, To: alpha43, Type: graph.EdgeDuplicate}, true)

		// When: the same pair arrives again from a timeline event, in key order
		g.AddEdge(graph.Edge{From: alpha43, To: alpha42, Type: graph.EdgeDuplicate}, false)

		// Then
		if g.Edges[0].From != alpha42 || g.Edges[0].To != alpha43 {
			t.Errorf("edge = %v, want the directed form to stand", g.Edges[0])
		}
	})

	t.Run("two kinds between one pair are two edges", func(t *testing.T) {
		// Given: a graph
		g := graph.NewGraph(alpha41)

		// When: the same pair is linked both structurally and by a mention
		g.AddEdge(graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses}, true)
		g.AddEdge(graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeMention}, true)

		// Then: AddEdge keeps both; choosing between them is normalization's job
		if got := len(g.Edges); got != 2 {
			t.Errorf("recorded %d edges, want 2", got)
		}
	})
}

func TestGraphDrawable(t *testing.T) {
	// Given: a graph holding one of an edge's two ends
	g := graph.NewGraph(alpha41)
	g.Add(graph.Node{Key: alpha41})
	g.Add(graph.Node{Key: alpha42})

	tests := []struct {
		name string
		// Given: an edge
		edge graph.Edge
		// Then: whether the tree can draw it
		want bool
	}{
		{"both ends admitted", graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses}, true},
		{"far end was never admitted", graph.Edge{From: alpha41, To: beta62, Type: graph.EdgeMention}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := g.Drawable(tt.edge)

			// Then
			if got != tt.want {
				t.Errorf("Drawable(%v) = %v, want %v", tt.edge, got, tt.want)
			}
		})
	}
}
