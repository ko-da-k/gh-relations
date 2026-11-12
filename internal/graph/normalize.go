package graph

// NormalizedLink is one reported link turned into an edge in the canonical
// direction, together with whatever the response said about the node at the
// far end.
type NormalizedLink struct {
	Edge  Edge
	Other Node

	// Directed is false when the API did not tell the two ends apart, so the
	// edge was stored in key order rather than in a meaningful direction.
	Directed bool
}

// Normalize turns everything one fetched item reported into canonical edges.
// One relation reaches us from both of its ends, so orienting every link the
// same way is what lets the graph collapse them to one edge.
func Normalize(obs Observation) []NormalizedLink {
	subject := obs.Subject.Key
	out := make([]NormalizedLink, 0, len(obs.Links))
	for _, link := range obs.Links {
		if link.Other.Key == subject {
			// A MarkedAsDuplicateEvent names both ends, one of which is the
			// timeline's owner.
			continue
		}
		edge, directed := orient(subject, link)
		out = append(out, NormalizedLink{Edge: edge, Other: link.Other, Directed: directed})
	}
	return out
}

// orient puts one reported link into the canonical direction of its kind.
func orient(subject Key, link Link) (Edge, bool) {
	other := link.Other.Key

	// relatesTo has no direction, and a duplicate known only from a timeline
	// event has none the field names can be trusted for, so both go in key
	// order and collapse whichever end they arrive from.
	if link.Type == EdgeRelates || link.Dir == Undirected {
		lo, hi := subject, other
		if hi.Less(lo) {
			lo, hi = hi, lo
		}
		return Edge{From: lo, To: hi, Type: link.Type}, false
	}

	if link.Dir == Inbound {
		return Edge{From: other, To: subject, Type: link.Type}, true
	}
	return Edge{From: subject, To: other, Type: link.Type}, true
}

// unorderedEnds is the two ends of an edge, lower key first, ignoring the kind.
func unorderedEnds(e Edge) [2]Key {
	if e.To.Less(e.From) {
		return [2]Key{e.To, e.From}
	}
	return [2]Key{e.From, e.To}
}

// PreferStructural drops a mention between two nodes that a declared relation
// already joins. Every structural link also shows up as a cross-reference, and
// the structural kind is the one that says what the link means. A caller who
// named the kinds explicitly keeps both, because then the mention was asked for.
func (g *Graph) PreferStructural() {
	structural := make(map[[2]Key]Edge)
	for _, e := range g.Edges {
		if e.Type.Structural() {
			ends := unorderedEnds(e)
			if _, seen := structural[ends]; !seen {
				structural[ends] = e
			}
		}
	}
	if len(structural) == 0 {
		return
	}

	kept := make([]Edge, 0, len(g.Edges))
	for _, e := range g.Edges {
		replacement, redundant := structural[unorderedEnds(e)]
		if e.Type == EdgeMention && redundant {
			g.repointVia(e, replacement)
			continue
		}
		kept = append(kept, e)
	}
	g.Edges = kept
}
