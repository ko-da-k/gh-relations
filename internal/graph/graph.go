package graph

// Graph is the result of a walk: the nodes that were admitted, the edges
// between them, and the shape the walk discovered them in.
//
// Via is that shape, and it cannot be re-derived from Edges: a mention is only
// visible from the mentioned side, so an edge can be recorded long after both
// of its ends were placed, and a tree re-derived from the edge set would move
// nodes to parents the walk never went through.
type Graph struct {
	Root  Key
	Nodes map[Key]*Node
	Order []Key // nodes in discovery order
	Edges []Edge
	Via   map[Key]Edge // the edge a node arrived on; the root has none

	// Requests is how many API requests the walk spent.
	Requests int

	// Truncated is set when a limit kept something out, so what is drawn is
	// known to be a part of the real graph rather than all of it.
	Truncated bool
}

// NewGraph returns an empty graph rooted at the given key.
func NewGraph(root Key) *Graph {
	return &Graph{
		Root:  root,
		Nodes: make(map[Key]*Node),
		Via:   make(map[Key]Edge),
	}
}

// Get returns the node under the key.
func (g *Graph) Get(k Key) (*Node, bool) {
	n, ok := g.Nodes[k]
	return n, ok
}

// Len is the number of nodes.
func (g *Graph) Len() int { return len(g.Nodes) }

// Add puts a node in the graph and returns it. A node already present keeps its
// place in Order and its hop count, and only fills in display fields it was
// missing.
func (g *Graph) Add(n Node) *Node {
	if existing, ok := g.Nodes[n.Key]; ok {
		existing.merge(n)
		return existing
	}
	stored := n
	g.Nodes[n.Key] = &stored
	g.Order = append(g.Order, n.Key)
	return &stored
}

// merge fills in fields that are missing from n but present in other.
func (n *Node) merge(other Node) {
	if n.ID == "" {
		n.ID = other.ID
	}
	if n.Title == "" {
		n.Title = other.Title
	}
	if n.URL == "" {
		n.URL = other.URL
	}
	if other.PartialLinks {
		n.PartialLinks = true
	}
}

// AddEdge records an edge, keeping one edge per node pair per kind. An edge
// already there is dropped, except that a known direction replaces an unknown
// one: a duplicate seen first as a timeline event and later through duplicateOf
// ends up pointing the right way.
func (g *Graph) AddEdge(e Edge, directed bool) {
	p := e.pair()
	for i := range g.Edges {
		if g.Edges[i].pair() != p {
			continue
		}
		if directed && g.Edges[i] != e {
			g.repointVia(g.Edges[i], e)
			g.Edges[i] = e
		}
		return
	}
	g.Edges = append(g.Edges, e)
}

// repointVia moves every node that arrived on old onto e, so that the walk's
// tree keeps naming edges the graph still holds.
func (g *Graph) repointVia(old, e Edge) {
	for k, via := range g.Via {
		if via == old {
			g.Via[k] = e
		}
	}
}

// Drawable reports whether both ends of the edge are nodes in the graph. Only
// --max-nodes can leave an edge with an end that is not.
func (g *Graph) Drawable(e Edge) bool {
	_, from := g.Nodes[e.From]
	_, to := g.Nodes[e.To]
	return from && to
}
