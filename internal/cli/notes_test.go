package cli_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/cli"
	"github.com/ko-da-k/gh-relations/internal/graph"
	"github.com/ko-da-k/gh-relations/internal/graph/graphtest"
)

func walkSample(t *testing.T, root graph.Key, opts graph.Options) *graph.Graph {
	t.Helper()
	g, err := graph.Walk(context.Background(), graphtest.NewFetcher(graphtest.Sample()), root, opts)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return g
}

func TestNotes(t *testing.T) {
	t.Run("a graph of one node explains why", func(t *testing.T) {
		// Given: an item whose only reference is one it wrote itself, in prose
		fixture := &graphtest.Fixture{
			Nodes: map[graph.Key]graph.Node{
				graphtest.A(1): {Key: graphtest.A(1), Kind: graph.KindIssue, Title: "writes a bare #2"},
				graphtest.A(2): {Key: graphtest.A(2), Kind: graph.KindIssue, Title: "never answers"},
			},
			Relations: []graphtest.Relation{graphtest.Mentions(graphtest.A(1), graphtest.A(2))},
		}
		opts := graph.DefaultOptions()
		g, err := graph.Walk(context.Background(), graphtest.NewFetcher(fixture), graphtest.A(1), opts)
		if err != nil {
			t.Fatalf("Walk: %v", err)
		}

		// When
		var out strings.Builder
		cli.Notes(&out, g, opts)

		// Then: the note says this is the blind spot, not an empty repository
		if !strings.Contains(out.String(), "No links found") {
			t.Errorf("note = %q", out.String())
		}
	})

	t.Run("it says which limit left the plus marks", func(t *testing.T) {
		// Given: a walk stopped by the depth
		opts := graph.DefaultOptions()
		opts.Depth = 1
		g := walkSample(t, graphtest.A(2), opts)

		// When
		var out strings.Builder
		cli.Notes(&out, g, opts)

		// Then
		if !strings.Contains(out.String(), "--depth") {
			t.Errorf("note = %q, want it to name --depth", out.String())
		}
	})

	t.Run("it names the node cap when that is what stopped the walk", func(t *testing.T) {
		// Given
		opts := graph.DefaultOptions()
		opts.MaxNodes = 8
		g := walkSample(t, graphtest.A(2), opts)

		// When
		var out strings.Builder
		cli.Notes(&out, g, opts)

		// Then
		if !strings.Contains(out.String(), "--max-nodes") {
			t.Errorf("note = %q, want it to name --max-nodes", out.String())
		}
	})

	t.Run("a walk that ran out of graph says nothing", func(t *testing.T) {
		// Given: a depth past everything the sample graph holds, and a scope
		// wide enough that the node outside the owner is expanded too
		opts := graph.DefaultOptions()
		opts.Depth = 9
		opts.Scope = graph.ScopeAll
		g := walkSample(t, graphtest.A(2), opts)

		// When
		var out strings.Builder
		cli.Notes(&out, g, opts)

		// Then: no limit was hit, so there is nothing to say
		if out.Len() != 0 {
			t.Errorf("note = %q, want nothing", out.String())
		}
	})
}
