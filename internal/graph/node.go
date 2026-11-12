package graph

// Kind separates the two things a node can be.
type Kind uint8

const (
	KindIssue Kind = iota
	KindPullRequest
)

func (k Kind) String() string {
	if k == KindPullRequest {
		return "pull request"
	}
	return "issue"
}

// State is the lifecycle an item is in. A closed issue is StateNotPlanned only
// where the API gave that reason explicitly; a duplicate is a closed issue,
// since the duplicate relation is an edge rather than a state.
type State uint8

const (
	StateOpen State = iota
	StateClosed
	StateMerged
	StateDraft
	StateNotPlanned
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateMerged:
		return "merged"
	case StateDraft:
		return "draft"
	case StateNotPlanned:
		return "not planned"
	default:
		return "open"
	}
}

// StopReason records why the walk did not expand a node.
type StopReason uint8

const (
	StopNone StopReason = iota
	StopDepth
	StopScope
	StopMaxNodes
)

func (r StopReason) String() string {
	switch r {
	case StopDepth:
		return "depth"
	case StopScope:
		return "scope"
	case StopMaxNodes:
		return "max-nodes"
	default:
		return ""
	}
}

// Node is one Issue or pull request in the graph. Its display fields come from
// whichever response first named it, which is not always its own: a node the
// walk never expands still has a number, title, state and URL.
type Node struct {
	Key   Key
	ID    string // GraphQL global id; only the fetcher uses it
	Kind  Kind
	State State
	Title string
	URL   string

	// Hop is the number of edges from the root along the path the node was
	// discovered on. Every edge kind counts as one.
	Hop int

	Stop StopReason

	// PartialLinks is set when one of the node's link connections was cut off
	// at its first page, so its links are known to be incomplete.
	PartialLinks bool
}
