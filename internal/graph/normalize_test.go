package graph_test

import (
	"fmt"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// The repositories these fixtures live in. They are spelled out here rather
// than taken from graphtest, because graphtest is introduced by a later change
// in this stack and this one has to build on its own.
//
// home and sibling share an owner; foreign does not, which is the distinction
// --scope org rests on. Two of the three are this project's own repositories,
// and the third has an underscore in its owner, which GitHub does not allow in
// an account name, so none of them can turn out to belong to a stranger.

func home(number int) graph.Key {
	return graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: number}
}

func sibling(number int) graph.Key {
	return graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: number}
}

func issue(k graph.Key) graph.Node {
	return graph.Node{
		Key:   k,
		ID:    fmt.Sprintf("I_%s%d", k.Repo, k.Number),
		Kind:  graph.KindIssue,
		State: graph.StateOpen,
		Title: fmt.Sprintf("issue %d", k.Number),
		URL:   fmt.Sprintf("https://example.test/%s/%d", k.Repo, k.Number),
	}
}

func pull(k graph.Key) graph.Node {
	n := issue(k)
	n.ID = fmt.Sprintf("PR_%s%d", k.Repo, k.Number)
	n.Kind = graph.KindPullRequest
	n.Title = fmt.Sprintf("pull request %d", n.Key.Number)
	return n
}

// edgesFrom runs every observation through Normalize and into one graph, which
// is what the walk does. The assertions are then about the graph the user sees.
func edgesFrom(root graph.Key, obs ...graph.Observation) *graph.Graph {
	g := graph.NewGraph(root)
	for _, o := range obs {
		g.Add(o.Subject)
		for _, link := range graph.Normalize(o) {
			g.Add(link.Other)
			g.AddEdge(link.Edge, link.Directed)
		}
	}
	return g
}

func wantEdges(t *testing.T, g *graph.Graph, want ...graph.Edge) {
	t.Helper()
	if len(g.Edges) != len(want) {
		t.Fatalf("got %d edges %v, want %d %v", len(g.Edges), g.Edges, len(want), want)
	}
	for i := range want {
		if g.Edges[i] != want[i] {
			t.Errorf("edge %d = %v, want %v", i, g.Edges[i], want[i])
		}
	}
}

func TestNormalizeCollapsesOneRelationSeenFromBothEnds(t *testing.T) {
	tests := []struct {
		name string
		// Given: the same relation as each of its two ends reports it
		fromOneEnd   graph.Observation
		fromTheOther graph.Observation
		// Then: the single canonical edge
		want graph.Edge
	}{
		{
			// closingIssuesReferences on the PR, closedByPullRequestsReferences on the issue
			name: "closes",
			fromOneEnd: graph.Observation{
				Subject: pull(home(41)),
				Links:   []graph.Link{{Type: graph.EdgeCloses, Dir: graph.Outbound, Other: issue(home(42))}},
			},
			fromTheOther: graph.Observation{
				Subject: issue(home(42)),
				Links:   []graph.Link{{Type: graph.EdgeCloses, Dir: graph.Inbound, Other: pull(home(41))}},
			},
			want: graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses},
		},
		{
			// parent on the sub-issue, subIssues on the parent
			name: "parent",
			fromOneEnd: graph.Observation{
				Subject: issue(home(42)),
				Links:   []graph.Link{{Type: graph.EdgeParent, Dir: graph.Inbound, Other: issue(home(43))}},
			},
			fromTheOther: graph.Observation{
				Subject: issue(home(43)),
				Links:   []graph.Link{{Type: graph.EdgeParent, Dir: graph.Outbound, Other: issue(home(42))}},
			},
			want: graph.Edge{From: alpha43, To: alpha42, Type: graph.EdgeParent},
		},
		{
			// blockedBy on the one held up, blocking on the blocker
			name: "blocks",
			fromOneEnd: graph.Observation{
				Subject: issue(home(42)),
				Links:   []graph.Link{{Type: graph.EdgeBlocks, Dir: graph.Inbound, Other: issue(home(46))}},
			},
			fromTheOther: graph.Observation{
				Subject: issue(home(46)),
				Links:   []graph.Link{{Type: graph.EdgeBlocks, Dir: graph.Outbound, Other: issue(home(42))}},
			},
			want: graph.Edge{From: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 46}, To: alpha42, Type: graph.EdgeBlocks},
		},
		{
			// trackedInIssues on the listed item, trackedIssues on the one holding the list
			name: "tracked",
			fromOneEnd: graph.Observation{
				Subject: issue(home(45)),
				Links:   []graph.Link{{Type: graph.EdgeTracked, Dir: graph.Inbound, Other: issue(home(49))}},
			},
			fromTheOther: graph.Observation{
				Subject: issue(home(49)),
				Links:   []graph.Link{{Type: graph.EdgeTracked, Dir: graph.Outbound, Other: issue(home(45))}},
			},
			want: graph.Edge{
				From: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 49},
				To:   graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 45},
				Type: graph.EdgeTracked,
			},
		},
		{
			// relatesTo answers from both sides with the same pair
			name: "relates",
			fromOneEnd: graph.Observation{
				Subject: issue(home(51)),
				Links:   []graph.Link{{Type: graph.EdgeRelates, Dir: graph.Undirected, Other: issue(home(50))}},
			},
			fromTheOther: graph.Observation{
				Subject: issue(home(50)),
				Links:   []graph.Link{{Type: graph.EdgeRelates, Dir: graph.Undirected, Other: issue(home(51))}},
			},
			want: graph.Edge{
				From: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 50},
				To:   graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 51},
				Type: graph.EdgeRelates,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: both ends are normalized into one graph
			g := edgesFrom(tt.want.From, tt.fromOneEnd, tt.fromTheOther)

			// Then
			wantEdges(t, g, tt.want)
		})
	}
}

func TestNormalizeDuplicate(t *testing.T) {
	alpha50 := graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 50}
	alpha53 := graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 53}
	alpha47 := graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 47}

	t.Run("duplicateOf gives the edge its direction", func(t *testing.T) {
		// Given: #53 was closed as a duplicate of #50, so duplicateOf is set on #53
		obs := graph.Observation{
			Subject: issue(home(53)),
			Links:   []graph.Link{{Type: graph.EdgeDuplicate, Dir: graph.Outbound, Other: issue(home(50))}},
		}

		// When
		g := edgesFrom(alpha53, obs)

		// Then: the duplicate is the From end
		wantEdges(t, g, graph.Edge{From: alpha53, To: alpha50, Type: graph.EdgeDuplicate})
	})

	t.Run("a timeline event alone is stored in key order", func(t *testing.T) {
		// Given: a "Duplicate of #50" comment, which leaves duplicateOf null and
		// writes only a MarkedAsDuplicateEvent. Its canonical and duplicate labels
		// were the timeline's owner and the other item in every measured case, so
		// they are not read as a direction.
		obs := graph.Observation{
			Subject: issue(home(54)),
			Links:   []graph.Link{{Type: graph.EdgeDuplicate, Dir: graph.Undirected, Other: issue(home(50))}},
		}

		// When
		g := edgesFrom(graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 54}, obs)

		// Then: lower key first, so the pair collapses whichever end it arrives from
		wantEdges(t, g, graph.Edge{
			From: alpha50,
			To:   graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 54},
			Type: graph.EdgeDuplicate,
		})
	})

	t.Run("the two routes to one duplicate collapse, and the direction wins", func(t *testing.T) {
		// Given: #53 reports duplicateOf #50, and #50's timeline reports the event
		byField := graph.Observation{
			Subject: issue(home(53)),
			Links:   []graph.Link{{Type: graph.EdgeDuplicate, Dir: graph.Outbound, Other: issue(home(50))}},
		}
		byEvent := graph.Observation{
			Subject: issue(home(50)),
			Links:   []graph.Link{{Type: graph.EdgeDuplicate, Dir: graph.Undirected, Other: issue(home(53))}},
		}

		// When: the undirected one arrives first
		g := edgesFrom(alpha50, byEvent, byField)

		// Then: one edge, pointing the way duplicateOf said
		wantEdges(t, g, graph.Edge{From: alpha53, To: alpha50, Type: graph.EdgeDuplicate})
	})

	t.Run("a pull request and an issue are one duplicate edge", func(t *testing.T) {
		// Given: a "Duplicate of #50" comment on pull request #47
		obs := graph.Observation{
			Subject: pull(home(47)),
			Links:   []graph.Link{{Type: graph.EdgeDuplicate, Dir: graph.Undirected, Other: issue(home(50))}},
		}

		// When
		g := edgesFrom(alpha47, obs)

		// Then: the kinds do not matter, only the keys
		wantEdges(t, g, graph.Edge{From: alpha47, To: alpha50, Type: graph.EdgeDuplicate})
	})

	t.Run("an event naming the subject itself is not an edge", func(t *testing.T) {
		// Given: a MarkedAsDuplicateEvent names both ends, one being the timeline's owner
		obs := graph.Observation{
			Subject: issue(home(50)),
			Links: []graph.Link{
				{Type: graph.EdgeDuplicate, Dir: graph.Undirected, Other: issue(home(50))},
				{Type: graph.EdgeDuplicate, Dir: graph.Undirected, Other: issue(home(53))},
			},
		}

		// When
		g := edgesFrom(alpha50, obs)

		// Then: only the other end becomes an edge
		wantEdges(t, g, graph.Edge{From: alpha50, To: alpha53, Type: graph.EdgeDuplicate})
	})
}

func TestNormalizeMention(t *testing.T) {
	t.Run("a cross reference points from the item that wrote it", func(t *testing.T) {
		// Given: #42's timeline reports that #45 referred to it
		obs := graph.Observation{
			Subject: issue(home(42)),
			Links:   []graph.Link{{Type: graph.EdgeMention, Dir: graph.Inbound, Other: issue(home(45))}},
		}

		// When
		g := edgesFrom(alpha42, obs)

		// Then: the writer is the From end
		wantEdges(t, g, graph.Edge{
			From: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 45},
			To:   alpha42,
			Type: graph.EdgeMention,
		})
	})
}

func TestPreferStructural(t *testing.T) {
	t.Run("a declared relation beats the cross reference it also produced", func(t *testing.T) {
		// Given: a PR that closes an issue. The closing keyword also writes a
		// cross-reference, so the issue reports both.
		obs := graph.Observation{
			Subject: issue(home(42)),
			Links: []graph.Link{
				{Type: graph.EdgeCloses, Dir: graph.Inbound, Other: pull(home(41))},
				{Type: graph.EdgeMention, Dir: graph.Inbound, Other: pull(home(41))},
			},
		}
		g := edgesFrom(alpha42, obs)

		// When
		g.PreferStructural()

		// Then: only the kind that says what the link means survives
		wantEdges(t, g, graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses})
	})

	t.Run("a task list entry beats its cross reference", func(t *testing.T) {
		// Given: "- [ ] #45" in #49's body writes both a tracked relation and a mention
		obs := graph.Observation{
			Subject: issue(home(45)),
			Links: []graph.Link{
				{Type: graph.EdgeTracked, Dir: graph.Inbound, Other: issue(home(49))},
				{Type: graph.EdgeMention, Dir: graph.Inbound, Other: issue(home(49))},
			},
		}
		g := edgesFrom(graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 45}, obs)

		// When
		g.PreferStructural()

		// Then
		wantEdges(t, g, graph.Edge{
			From: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 49},
			To:   graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 45},
			Type: graph.EdgeTracked,
		})
	})

	t.Run("a cross reference with no relation behind it stays", func(t *testing.T) {
		// Given: a bare #42 written in #45's body, with no keyword and no sidebar link
		obs := graph.Observation{
			Subject: issue(home(42)),
			Links:   []graph.Link{{Type: graph.EdgeMention, Dir: graph.Inbound, Other: issue(home(45))}},
		}
		g := edgesFrom(alpha42, obs)

		// When
		g.PreferStructural()

		// Then
		wantEdges(t, g, graph.Edge{
			From: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 45},
			To:   alpha42,
			Type: graph.EdgeMention,
		})
	})

	t.Run("a node that arrived on a dropped mention is re-pointed at the survivor", func(t *testing.T) {
		// Given: #41 was discovered through a mention, and only later did a closes
		// edge for the same pair turn up
		g := graph.NewGraph(alpha42)
		g.Add(issue(home(42)))
		g.Add(pull(home(41)))
		mention := graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeMention}
		closes := graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses}
		g.AddEdge(mention, true)
		g.AddEdge(closes, true)
		g.Via[alpha41] = mention

		// When
		g.PreferStructural()

		// Then: the tree refers to an edge that is still in the graph
		if got := g.Via[alpha41]; got != closes {
			t.Errorf("Via = %v, want the surviving %v", got, closes)
		}
		wantEdges(t, g, closes)
	})

	t.Run("a mention between another pair is untouched", func(t *testing.T) {
		// Given: a closes edge between #41 and #42, and a mention between #41 and #44
		g := graph.NewGraph(alpha41)
		closes := graph.Edge{From: alpha41, To: alpha42, Type: graph.EdgeCloses}
		alpha44 := graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 44}
		mention := graph.Edge{From: alpha41, To: alpha44, Type: graph.EdgeMention}
		g.AddEdge(closes, true)
		g.AddEdge(mention, true)

		// When
		g.PreferStructural()

		// Then
		wantEdges(t, g, closes, mention)
	})
}
