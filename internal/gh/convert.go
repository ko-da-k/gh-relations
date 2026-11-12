package gh

import (
	"fmt"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// item is one Issue or pull request as the response carries it. Nested items
// only ever carry the Ref fields, so their own link fields stay empty.
type item struct {
	Typename    string  `json:"__typename"`
	ID          string  `json:"id"`
	Number      int     `json:"number"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	State       string  `json:"state"`
	StateReason *string `json:"stateReason"`
	IsDraft     bool    `json:"isDraft"`
	Repository  struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`

	Parent                         *item      `json:"parent"`
	SubIssues                      connection `json:"subIssues"`
	BlockedBy                      connection `json:"blockedBy"`
	Blocking                       connection `json:"blocking"`
	RelatesTo                      connection `json:"relatesTo"`
	DuplicateOf                    *item      `json:"duplicateOf"`
	TrackedIssues                  connection `json:"trackedIssues"`
	TrackedInIssues                connection `json:"trackedInIssues"`
	ClosingIssuesReferences        connection `json:"closingIssuesReferences"`
	ClosedByPullRequestsReferences connection `json:"closedByPullRequestsReferences"`
	TimelineItems                  timeline   `json:"timelineItems"`
}

type pageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
}

type connection struct {
	PageInfo pageInfo `json:"pageInfo"`
	Nodes    []*item  `json:"nodes"`
}

type timeline struct {
	PageInfo pageInfo       `json:"pageInfo"`
	Nodes    []timelineItem `json:"nodes"`
}

type timelineItem struct {
	Typename string `json:"__typename"`

	// Source is the item that wrote the reference, on a CrossReferencedEvent.
	Source *item `json:"source"`

	// Canonical and Duplicate come from a MarkedAsDuplicateEvent. The names
	// carry no direction: in every measured event the canonical was the
	// timeline's owner, whichever one was really the duplicate.
	Canonical *item `json:"canonical"`
	Duplicate *item `json:"duplicate"`
}

// toNode turns one item into a graph node. It is also used for nested items,
// which carry everything needed to draw a node that is never fetched.
func toNode(it *item) (graph.Node, error) {
	key, err := graph.ParseNameWithOwner(it.Repository.NameWithOwner, it.Number)
	if err != nil {
		return graph.Node{}, fmt.Errorf("item %d: %w", it.Number, err)
	}
	kind := graph.KindIssue
	if it.Typename == "PullRequest" {
		kind = graph.KindPullRequest
	}
	return graph.Node{
		Key:   key,
		ID:    it.ID,
		Kind:  kind,
		State: stateOf(it, kind),
		Title: it.Title,
		URL:   it.URL,
	}, nil
}

// stateOf maps the API's state to the one the colour carries. stateReason was
// added in 2022 and is null on issues closed before then, so only an explicit
// NOT_PLANNED is treated as one; DUPLICATE is a plain closed issue, since being
// a duplicate is an edge rather than a state.
func stateOf(it *item, kind graph.Kind) graph.State {
	if kind == graph.KindPullRequest {
		switch it.State {
		case "MERGED":
			return graph.StateMerged
		case "CLOSED":
			return graph.StateClosed
		default:
			if it.IsDraft {
				return graph.StateDraft
			}
			return graph.StateOpen
		}
	}

	if it.State != "CLOSED" {
		return graph.StateOpen
	}
	if it.StateReason != nil && *it.StateReason == "NOT_PLANNED" {
		return graph.StateNotPlanned
	}
	return graph.StateClosed
}

// toObservation turns one fetched item into what the walk consumes: the item,
// and every link it reports with that link's direction relative to it.
func toObservation(it *item) (graph.Observation, error) {
	subject, err := toNode(it)
	if err != nil {
		return graph.Observation{}, err
	}

	obs := graph.Observation{Subject: subject}
	var convErr error

	add := func(other *item, kind graph.EdgeType, dir graph.Direction) {
		if other == nil || convErr != nil {
			return
		}
		node, err := toNode(other)
		if err != nil {
			convErr = err
			return
		}
		obs.Links = append(obs.Links, graph.Link{Type: kind, Dir: dir, Other: node})
	}

	addAll := func(c connection, kind graph.EdgeType, dir graph.Direction) {
		if c.PageInfo.HasNextPage {
			obs.Subject.PartialLinks = true
		}
		for _, other := range c.Nodes {
			add(other, kind, dir)
		}
	}

	// A pull request's own closing references point away from it; an issue's
	// closedBy references point at it. One relation, two fields.
	addAll(it.ClosingIssuesReferences, graph.EdgeCloses, graph.Outbound)
	addAll(it.ClosedByPullRequestsReferences, graph.EdgeCloses, graph.Inbound)

	add(it.Parent, graph.EdgeParent, graph.Inbound)
	addAll(it.SubIssues, graph.EdgeParent, graph.Outbound)

	addAll(it.Blocking, graph.EdgeBlocks, graph.Outbound)
	addAll(it.BlockedBy, graph.EdgeBlocks, graph.Inbound)

	addAll(it.RelatesTo, graph.EdgeRelates, graph.Undirected)

	// duplicateOf is the only thing that says which of two items is the
	// duplicate, and it is set only by closing as a duplicate.
	add(it.DuplicateOf, graph.EdgeDuplicate, graph.Outbound)

	addAll(it.TrackedIssues, graph.EdgeTracked, graph.Outbound)
	addAll(it.TrackedInIssues, graph.EdgeTracked, graph.Inbound)

	if it.TimelineItems.PageInfo.HasNextPage {
		obs.Subject.PartialLinks = true
	}
	for _, event := range it.TimelineItems.Nodes {
		switch event.Typename {
		case "CrossReferencedEvent":
			// Held only on the side that was referred to, so the source is
			// always the item that wrote the reference.
			add(event.Source, graph.EdgeMention, graph.Inbound)
		case "MarkedAsDuplicateEvent":
			add(event.Canonical, graph.EdgeDuplicate, graph.Undirected)
			add(event.Duplicate, graph.EdgeDuplicate, graph.Undirected)
		}
	}

	if convErr != nil {
		return graph.Observation{}, convErr
	}
	return obs, nil
}

// toObservations converts a whole response, skipping the nulls that nodes(ids:)
// carries where an id could not be resolved.
func toObservations(items []*item) ([]graph.Observation, error) {
	out := make([]graph.Observation, 0, len(items))
	for _, it := range items {
		if it == nil {
			continue
		}
		obs, err := toObservation(it)
		if err != nil {
			return nil, err
		}
		out = append(out, obs)
	}
	return out, nil
}
