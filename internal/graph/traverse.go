package graph

import (
	"context"
	"fmt"
)

// Scope says where the walk may expand. A node outside the scope is still
// admitted and drawn, it is simply never fetched.
type Scope uint8

const (
	// ScopeRepo expands only inside the root's repository.
	ScopeRepo Scope = iota
	// ScopeOrg expands inside any repository of the root's owner.
	ScopeOrg
	// ScopeAll expands anywhere.
	ScopeAll
)

func (s Scope) String() string {
	switch s {
	case ScopeRepo:
		return "repo"
	case ScopeAll:
		return "all"
	default:
		return "org"
	}
}

// ParseScope maps a flag value to a scope.
func ParseScope(s string) (Scope, error) {
	switch s {
	case "repo":
		return ScopeRepo, nil
	case "org":
		return ScopeOrg, nil
	case "all":
		return ScopeAll, nil
	}
	return 0, fmt.Errorf("unknown scope %q (want repo, org or all)", s)
}

// Options bounds the walk on two independent axes: how far (Depth) and where
// (Scope), with MaxNodes as a backstop.
type Options struct {
	// Depth is how many hops out to expand. Every edge kind counts as one hop,
	// so the walk spends at most max(1, Depth) requests.
	Depth int

	Scope Scope

	// MaxNodes stops the walk once the graph holds this many nodes. It bounds
	// the links that sit inside one level, which Depth cannot see.
	MaxNodes int
}

// DefaultOptions is what the command uses when no flag says otherwise.
func DefaultOptions() Options {
	return Options{Depth: 4, Scope: ScopeOrg, MaxNodes: 50}
}

// Validate rejects option values that would make the walk meaningless.
func (o Options) Validate() error {
	if o.Depth < 0 {
		return fmt.Errorf("--depth must not be negative, got %d", o.Depth)
	}
	if o.MaxNodes < 1 {
		return fmt.Errorf("--max-nodes must be at least 1, got %d", o.MaxNodes)
	}
	return nil
}

// Batch is the result of one call into the fetcher: what it saw, and how many
// API requests it spent. A level is normally one request, but a level too big
// for one nodes(ids:) call is split, so the count is the fetcher's to report.
type Batch struct {
	Observations []Observation
	Requests     int
}

// Fetcher is the walk's only contact with the outside world.
type Fetcher interface {
	// FetchRoot resolves the root by repository and number, because at that
	// point there is no global id to go on yet.
	FetchRoot(ctx context.Context, root Key) (Batch, error)

	// FetchLevel fetches a whole level, in as few requests as it can.
	FetchLevel(ctx context.Context, nodes []Node) (Batch, error)
}

// Walk grows the graph outwards from the root, one request per level. A node is
// marked as seen when it is discovered rather than when it is expanded.
func Walk(ctx context.Context, fetcher Fetcher, root Key, opts Options) (*Graph, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	g := NewGraph(root)

	batch, err := fetcher.FetchRoot(ctx, root)
	if err != nil {
		return nil, err
	}
	g.Requests += batch.Requests
	if len(batch.Observations) == 0 {
		return nil, fmt.Errorf("%s was not found, or is not visible to this account", root)
	}

	// The API answers with the repository's canonical name, which is not always
	// how the user wrote it. Everything downstream compares against g.Root, so
	// the graph adopts the name the API gave.
	rootNode := batch.Observations[0].Subject
	rootNode.Hop = 0
	g.Root = rootNode.Key
	g.Add(rootNode)

	frontier := g.absorb(batch.Observations, opts)
	for len(frontier) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batch, err := fetcher.FetchLevel(ctx, frontier)
		if err != nil {
			return nil, err
		}
		g.Requests += batch.Requests
		frontier = g.absorb(batch.Observations, opts)
	}

	g.PreferStructural()
	return g, nil
}

// absorb folds one level's observations into the graph and returns the nodes
// that make up the next level.
func (g *Graph) absorb(observations []Observation, opts Options) []Node {
	var next []Node

	for _, obs := range observations {
		subject, ok := g.Get(obs.Subject.Key)
		if !ok {
			continue
		}
		subject.merge(obs.Subject)
		subject.Stop = StopNone
		if subject.PartialLinks {
			g.Truncated = true
		}

		// Only --depth 0 reaches this: the root is fetched to be drawn at all,
		// but its links are already past the depth.
		if subject.Hop >= opts.Depth {
			subject.Stop = StopDepth
			continue
		}

		for _, link := range Normalize(obs) {
			other := link.Other.Key
			if _, known := g.Get(other); !known {
				if g.Len() >= opts.MaxNodes {
					// The edge is recorded although its far end is not admitted,
					// which is why --max-nodes is the one bound that can leave an
					// edge the tree cannot draw.
					g.Truncated = true
					subject.Stop = StopMaxNodes
					g.AddEdge(link.Edge, link.Directed)
					continue
				}
				admitted := link.Other
				admitted.Hop = subject.Hop + 1
				admitted.Stop = stopReasonFor(g.Root, admitted, opts)
				g.Add(admitted)
				g.Via[other] = link.Edge
				if admitted.Stop == StopNone {
					next = append(next, admitted)
				}
			}
			g.AddEdge(link.Edge, link.Directed)
		}
	}

	// A full graph makes another level pointless: it could only add edges
	// between nodes already in it, at the price of another request.
	if g.Len() >= opts.MaxNodes && len(next) > 0 {
		for _, n := range next {
			if node, ok := g.Get(n.Key); ok {
				node.Stop = StopMaxNodes
			}
		}
		g.Truncated = true
		return nil
	}
	return next
}

// stopReasonFor decides whether a freshly admitted node will be expanded.
func stopReasonFor(root Key, n Node, opts Options) StopReason {
	if n.Hop >= opts.Depth {
		return StopDepth
	}
	if !inScope(root, n.Key, opts.Scope) {
		return StopScope
	}
	return StopNone
}

func inScope(root, k Key, scope Scope) bool {
	switch scope {
	case ScopeRepo:
		return k.SameRepo(root)
	case ScopeAll:
		return true
	default:
		return k.SameOwner(root)
	}
}
