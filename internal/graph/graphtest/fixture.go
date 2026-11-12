// Package graphtest builds the sample link graph that the tests of
// internal/graph and internal/render both work from.
//
// Sample() is not invented. Every item in it exists, in ko-da-k/gh-relations
// and ko-da-k/gh-relations-fixtures, with the number and the title written
// here, and every edge was made with the mechanic the comment on its
// constructor names. Running the extension on the root draws this graph, which
// is what makes the example in the README something a reader can check rather
// than take on trust.
//
// Both repositories belong to this project, and every Issue and pull request
// in them is titled "【for Test】...": these 15, and a handful left over from
// working out which links GitHub would let us make, which no edge here reaches.
// So nothing recorded here is anybody's work, and no account appears but the
// owner's.
//
// SampleWithForeign() adds the one node that cannot be real: --scope all is
// only distinguishable from --scope org by an item under another owner, and
// creating one would mean writing into a repository that is not ours. Its
// owner holds an underscore, which GitHub does not allow in an account name,
// so the key can never resolve to anybody.
//
// The fixture reproduces the one property that makes this problem awkward: a
// link is not always visible from both of its ends. A declared relation is —
// closingIssuesReferences on the pull request and closedByPullRequestsReferences
// on the issue describe one link — but a bare reference in prose is reported
// only by the item that was referred to, and a duplicate created by a
// "Duplicate of #N" comment is reported only by the item that was commented on.
// Each edge below therefore says what each of its two ends can see.
package graphtest

import (
	"context"
	"fmt"
	"slices"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// The repositories the fixture uses, and the title prefix that keeps a reader
// of either repository from mistaking one of the items for work.
const (
	Owner        = "ko-da-k"
	RepoHome     = "gh-relations"
	RepoSibling  = "gh-relations-fixtures"
	ForeignOwner = "another_owner"
	RepoForeign  = "gh-relations"
	TitlePrefix  = "【for Test】"
)

// A returns a key in ko-da-k/gh-relations, the root's repository.
func A(number int) graph.Key {
	return graph.Key{Owner: Owner, Repo: RepoHome, Number: number}
}

// B returns a key in ko-da-k/gh-relations-fixtures: another repository under
// the same owner, which --scope repo stops at and --scope org does not.
//
// The two repositories were filled independently, so their numbers overlap:
// A(7) and B(7) are different items. Anything that keyed on the number alone
// would pass a fixture whose ranges were disjoint and fail this one.
func B(number int) graph.Key {
	return graph.Key{Owner: Owner, Repo: RepoSibling, Number: number}
}

// G returns a key under another owner, which --scope org stops at and
// --scope all does not.
func G(number int) graph.Key {
	return graph.Key{Owner: ForeignOwner, Repo: RepoForeign, Number: number}
}

// sight is what one end of an edge can see of it.
type sight struct {
	visible bool
	dir     graph.Direction
}

var blind = sight{}

func sees(d graph.Direction) sight { return sight{visible: true, dir: d} }

// Relation is one link in the fixture, plus what each end reports about it.
type Relation struct {
	Edge graph.Edge
	From sight // what Edge.From's own response says
	To   sight // what Edge.To's own response says
}

// Closes is a closing keyword or a Development-sidebar link: both ends report it.
func Closes(pr, issue graph.Key) Relation {
	return both(graph.Edge{From: pr, To: issue, Type: graph.EdgeCloses})
}

// Parent is a sub-issue relation: both ends report it.
func Parent(parent, sub graph.Key) Relation {
	return both(graph.Edge{From: parent, To: sub, Type: graph.EdgeParent})
}

// Blocks is an entry in the Relationships sidebar: both ends report it.
func Blocks(blocker, blocked graph.Key) Relation {
	return both(graph.Edge{From: blocker, To: blocked, Type: graph.EdgeBlocks})
}

// Tracks is a legacy task-list checkbox: both ends report it.
func Tracks(list, item graph.Key) Relation {
	return both(graph.Edge{From: list, To: item, Type: graph.EdgeTracked})
}

// Relates is the Relationships sidebar's loose link. Both ends report it, and
// neither says which is which.
func Relates(a, b graph.Key) Relation {
	lo, hi := a, b
	if hi.Less(lo) {
		lo, hi = hi, lo
	}
	e := graph.Edge{From: lo, To: hi, Type: graph.EdgeRelates}
	return Relation{Edge: e, From: sees(graph.Undirected), To: sees(graph.Undirected)}
}

// DuplicateOf is "close as duplicate": the duplicate's own response names the
// item it duplicates, and the other end sees only an undirected timeline event.
func DuplicateOf(duplicate, canonical graph.Key) Relation {
	e := graph.Edge{From: duplicate, To: canonical, Type: graph.EdgeDuplicate}
	return Relation{Edge: e, From: sees(graph.Outbound), To: sees(graph.Undirected)}
}

// DuplicateByComment is a "Duplicate of #N" comment. duplicateOf stays null, so
// only the item that was commented on reports the event, and it reports no
// direction. The other end sees a plain reference instead, which the caller
// adds with Mentions.
func DuplicateByComment(commented, other graph.Key) Relation {
	lo, hi := commented, other
	if hi.Less(lo) {
		lo, hi = hi, lo
	}
	e := graph.Edge{From: lo, To: hi, Type: graph.EdgeDuplicate}
	r := Relation{Edge: e}
	if e.From == commented {
		r.From = sees(graph.Undirected)
	} else {
		r.To = sees(graph.Undirected)
	}
	return r
}

// Mentions is a bare reference in prose, which only the item referred to
// reports.
func Mentions(writer, mentioned graph.Key) Relation {
	return Relation{
		Edge: graph.Edge{From: writer, To: mentioned, Type: graph.EdgeMention},
		From: blind,
		To:   sees(graph.Inbound),
	}
}

func both(e graph.Edge) Relation {
	return Relation{Edge: e, From: sees(graph.Outbound), To: sees(graph.Inbound)}
}

// Fixture is a graph the fetcher can answer questions about.
type Fixture struct {
	Nodes     map[graph.Key]graph.Node
	Relations []Relation
}

// Node returns the fixture's description of one item.
func (f *Fixture) Node(k graph.Key) graph.Node {
	if n, ok := f.Nodes[k]; ok {
		return n
	}
	return graph.Node{Key: k, Title: "unknown"}
}

// LinksFor is what the item's own response reports, and nothing else.
func (f *Fixture) LinksFor(k graph.Key) []graph.Link {
	var out []graph.Link
	for _, r := range f.Relations {
		switch {
		case r.Edge.From == k && r.From.visible:
			out = append(out, graph.Link{Type: r.Edge.Type, Dir: r.From.dir, Other: f.Node(r.Edge.To)})
		case r.Edge.To == k && r.To.visible:
			out = append(out, graph.Link{Type: r.Edge.Type, Dir: r.To.dir, Other: f.Node(r.Edge.From)})
		}
	}
	return out
}

// Fetcher answers a walk from a fixture, counting what it was asked for so a
// test can assert on request counts and on which items were ever fetched.
type Fetcher struct {
	Fixture *Fixture

	// IDsPerRequest is the nodes(ids:) batch size. Zero means no splitting.
	IDsPerRequest int

	// Fetched is every item the walk asked for, in order.
	Fetched []graph.Key

	// Levels is how many times FetchLevel was called.
	Levels int

	// RootAlias maps a root as the user wrote it to the key the API answers
	// with, which is how a renamed repository or another case is reproduced.
	RootAlias map[graph.Key]graph.Key
}

// NewFetcher wraps a fixture.
func NewFetcher(f *Fixture) *Fetcher { return &Fetcher{Fixture: f} }

// FetchRoot resolves the root the way repository.issueOrPullRequest does.
func (f *Fetcher) FetchRoot(_ context.Context, root graph.Key) (graph.Batch, error) {
	if canonical, ok := f.RootAlias[root]; ok {
		root = canonical
	}
	if _, ok := f.Fixture.Nodes[root]; !ok {
		return graph.Batch{Requests: 1}, nil
	}
	f.Fetched = append(f.Fetched, root)
	return graph.Batch{Observations: []graph.Observation{f.observe(root)}, Requests: 1}, nil
}

// FetchLevel answers a whole level, splitting it when IDsPerRequest is set.
func (f *Fetcher) FetchLevel(_ context.Context, nodes []graph.Node) (graph.Batch, error) {
	f.Levels++
	batch := graph.Batch{Requests: 1}
	if f.IDsPerRequest > 0 {
		batch.Requests = (len(nodes) + f.IDsPerRequest - 1) / f.IDsPerRequest
	}
	for _, n := range nodes {
		f.Fetched = append(f.Fetched, n.Key)
		batch.Observations = append(batch.Observations, f.observe(n.Key))
	}
	return batch, nil
}

// FetchCount is how many times the item was fetched.
func (f *Fetcher) FetchCount(k graph.Key) int {
	n := 0
	for _, got := range f.Fetched {
		if got == k {
			n++
		}
	}
	return n
}

// WasFetched reports whether the walk ever asked for the item's own links.
func (f *Fetcher) WasFetched(k graph.Key) bool { return slices.Contains(f.Fetched, k) }

func (f *Fetcher) observe(k graph.Key) graph.Observation {
	return graph.Observation{Subject: f.Fixture.Node(k), Links: f.Fixture.LinksFor(k)}
}

// issue and pull describe one item. The title is given without its prefix,
// which every item in both repositories carries, and the ID stands in for the
// opaque one the API answers with: nothing reads an ID but the batching, which
// only needs them to be distinct.
func issue(k graph.Key, state graph.State, title string) graph.Node {
	return graph.Node{
		Key:   k,
		ID:    fmt.Sprintf("I_%s_%s_%d", k.Owner, k.Repo, k.Number),
		Kind:  graph.KindIssue,
		State: state,
		Title: TitlePrefix + title,
		URL:   fmt.Sprintf("https://github.com/%s/%s/issues/%d", k.Owner, k.Repo, k.Number),
	}
}

func pull(k graph.Key, state graph.State, title string) graph.Node {
	return graph.Node{
		Key:   k,
		ID:    fmt.Sprintf("PR_%s_%s_%d", k.Owner, k.Repo, k.Number),
		Kind:  graph.KindPullRequest,
		State: state,
		Title: TitlePrefix + title,
		URL:   fmt.Sprintf("https://github.com/%s/%s/pull/%d", k.Owner, k.Repo, k.Number),
	}
}

// Sample is the fixture graph: the 15 items that exist in ko-da-k/gh-relations
// and ko-da-k/gh-relations-fixtures, and every link between them. It carries
// every edge kind, both cross-repository directions, and a cycle.
//
//	A#2 (PR)  closes     A#3               A#10        tracks     A#6
//	A#4       parent of  A#3, A#5, B#8     B#10, B#11  tracks     A#6
//	A#7       blocks     A#3               A#8 (PR)    duplicate  A#11  (comment)
//	B#7       blocks     A#7               A#10        relates    B#11
//	B#9 (PR)  closes     A#7
//	A#9 (PR)  closes     A#5
//
//	mentions (reported only by the item referred to)
//	  A#3 <- A#6, A#2     A#4 <- A#8, B#8     A#7 <- A#2, B#9
//	  A#5 <- A#2, A#9     A#2 <- B#7          A#11 <- A#8
//	  A#6 <- A#10, B#10, B#11
//
// The mention A#2 -> A#5 closes a cycle: A#2 reaches A#5 through A#3 and A#4,
// and A#5 names A#2 again.
//
// Several of the mentions are not separate acts: a closing keyword, a task-list
// checkbox and a "Duplicate of #N" comment each leave a cross-reference as
// well, and those are recorded here because GitHub reports them. They do not
// reach the tree, because PreferStructural drops a mention that runs beside a
// declared relation.
func Sample() *Fixture {
	nodes := []graph.Node{
		pull(A(2), graph.StateOpen, "closes #3, mentions #5 and #7"),
		issue(A(3), graph.StateClosed, "closed by #2, sub of #4, blocked by #7"),
		issue(A(4), graph.StateOpen, "has 3 sub-issues: #3, #5, fixtures#8"),
		issue(A(5), graph.StateNotPlanned, "sub of #4, closed by #9"),
		issue(A(6), graph.StateOpen, "tracked in #10, fixtures#10 and fixtures#11, mentions #3"),
		// Closed by the merge of B#9, which is in the other repository: a
		// closing keyword was measured to reach across repositories, and
		// merging the pull request closed this issue from there.
		issue(A(7), graph.StateClosed, "blocks #3, blocked by fixtures#7, closed by fixtures#9"),
		pull(A(8), graph.StateDraft, "duplicate of #11 by comment, mentions #4"),
		pull(A(9), graph.StateClosed, "closes #5"),
		issue(A(10), graph.StateOpen, "tracks #6, relates to fixtures#11"),
		issue(A(11), graph.StateOpen, "#8 is marked a duplicate of this by comment"),
		issue(B(7), graph.StateOpen, "blocks gh-relations#7, mentions gh-relations#2"),
		issue(B(8), graph.StateOpen, "sub of gh-relations#4"),
		pull(B(9), graph.StateMerged, "closes gh-relations#7"),
		issue(B(10), graph.StateOpen, "tracks gh-relations#6"),
		issue(B(11), graph.StateOpen, "tracks gh-relations#6, relates to gh-relations#10"),
	}

	f := &Fixture{Nodes: make(map[graph.Key]graph.Node, len(nodes))}
	for _, n := range nodes {
		f.Nodes[n.Key] = n
	}

	f.Relations = []Relation{
		Closes(A(2), A(3)),
		Parent(A(4), A(3)),
		Parent(A(4), A(5)),
		Parent(A(4), B(8)),
		Blocks(A(7), A(3)),
		Blocks(B(7), A(7)),
		Closes(B(9), A(7)),
		Closes(A(9), A(5)),
		Tracks(A(10), A(6)),
		Tracks(B(10), A(6)),
		Tracks(B(11), A(6)),
		Relates(A(10), B(11)),
		DuplicateByComment(A(8), A(11)),

		Mentions(A(6), A(3)),
		Mentions(A(2), A(3)),
		Mentions(A(8), A(4)),
		Mentions(B(8), A(4)),
		Mentions(A(2), A(7)),
		Mentions(B(9), A(7)),
		Mentions(A(2), A(5)),
		Mentions(A(9), A(5)),
		Mentions(A(10), A(6)),
		Mentions(B(10), A(6)),
		Mentions(B(11), A(6)),
		Mentions(B(7), A(2)),
		Mentions(A(8), A(11)),
	}
	return f
}

// SampleWithForeign is Sample plus one item under another owner, related to
// A#5.
//
// --scope all differs from --scope org only outside the owner, so a test for
// that difference needs a node there, and no real one can be made: it would
// mean writing into somebody else's repository. This is the one node in the
// package that does not exist, and it is kept out of Sample so that the
// example in the README stays reproducible in full.
func SampleWithForeign() *Fixture {
	f := Sample()
	foreign := issue(G(71), graph.StateOpen, "relates to gh-relations#5")
	f.Nodes[foreign.Key] = foreign
	// A#5 is the end it hangs off because A#5 sits in the middle of the graph,
	// so the node beyond it is reached at a hop the default depth does not
	// expand, which is where a scope can be told apart.
	f.Relations = append(f.Relations, Relates(A(5), G(71)))
	return f
}
