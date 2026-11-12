package graph

import "fmt"

// EdgeType is one of the seven link kinds the API exposes as structured
// fields. The constants are in drawing order, with mentions last.
type EdgeType uint8

const (
	EdgeCloses EdgeType = iota
	EdgeParent
	EdgeBlocks
	EdgeRelates
	EdgeDuplicate
	EdgeTracked
	EdgeMention

	numEdgeTypes = int(EdgeMention) + 1
)

var edgeNames = [numEdgeTypes]string{
	EdgeCloses:    "closes",
	EdgeParent:    "parent",
	EdgeBlocks:    "blocks",
	EdgeRelates:   "relates",
	EdgeDuplicate: "duplicate",
	EdgeTracked:   "tracked",
	EdgeMention:   "mention",
}

func (t EdgeType) String() string {
	if int(t) < numEdgeTypes {
		return edgeNames[t]
	}
	return fmt.Sprintf("EdgeType(%d)", uint8(t))
}

// Structural reports whether the edge is a relation someone declared, rather
// than a bare reference in prose.
func (t EdgeType) Structural() bool { return t != EdgeMention }

// Edge is one link between two nodes, stored in a canonical direction:
//
//	Closes     From is the pull request,     To is the issue it closes
//	Parent     From is the parent,           To is the sub-issue
//	Blocks     From is the blocker,          To is the one held up
//	Tracked    From holds the task list,     To is the item listed
//	Mention    From wrote the reference,     To is the one referred to
//	Relates    no direction: the lower key is From
//	Duplicate  From is the duplicate, To is the one it duplicates, when the API
//	           says so; otherwise the lower key is From
type Edge struct {
	From Key
	To   Key
	Type EdgeType
}

// Other returns the end of the edge that is not k. The caller is expected to
// pass an end of the edge.
func (e Edge) Other(k Key) Key {
	if e.From == k {
		return e.To
	}
	return e.From
}

// Touches reports whether k is one of the edge's ends.
func (e Edge) Touches(k Key) bool { return e.From == k || e.To == k }

// pair is the unordered identity of an edge: the two ends plus the kind,
// whichever way round the edge happens to be stored.
type pair struct {
	Lo, Hi Key
	Type   EdgeType
}

func (e Edge) pair() pair {
	lo, hi := e.From, e.To
	if hi.Less(lo) {
		lo, hi = hi, lo
	}
	return pair{Lo: lo, Hi: hi, Type: e.Type}
}

// Direction says how a link found on one node relates to that node.
type Direction uint8

const (
	// Outbound points away from the node the link was found on.
	Outbound Direction = iota
	// Inbound points at the node the link was found on.
	Inbound
	// Undirected is a link the API reports without telling the ends apart,
	// such as relatesTo, or a duplicate known only from a timeline event.
	Undirected
)

// Link is one relation as a single fetched item reports it: a direction
// relative to that item, and whatever the response said about the far end.
type Link struct {
	Type  EdgeType
	Dir   Direction
	Other Node
}

// Observation is everything one fetched item told us: what it is, and the links
// it reports. It is the only shape internal/gh has to produce.
type Observation struct {
	Subject Node
	Links   []Link
}
