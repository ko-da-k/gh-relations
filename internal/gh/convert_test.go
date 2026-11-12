package gh

import (
	"encoding/json"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// The JSON below is written by hand, in the shape the API returns, with
// invented repositories. No recorded response is committed: one would carry
// real repository names, real numbers and real accounts.

func parseItem(t *testing.T, body string) *item {
	t.Helper()
	var it item
	if err := json.Unmarshal([]byte(body), &it); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &it
}

func observe(t *testing.T, body string) graph.Observation {
	t.Helper()
	obs, err := toObservation(parseItem(t, body))
	if err != nil {
		t.Fatalf("toObservation: %v", err)
	}
	return obs
}

func home(number int) graph.Key {
	return graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: number}
}

func sibling(number int) graph.Key {
	return graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: number}
}

func findLink(obs graph.Observation, other graph.Key, kind graph.EdgeType) (graph.Link, bool) {
	for _, l := range obs.Links {
		if l.Other.Key == other && l.Type == kind {
			return l, true
		}
	}
	return graph.Link{}, false
}

func TestStateMapping(t *testing.T) {
	tests := []struct {
		name string
		// Given: an item as the API describes it
		body string
		// Then: the state the colour carries
		want graph.State
	}{
		{
			"an open issue",
			`{"__typename":"Issue","number":1,"state":"OPEN","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateOpen,
		},
		{
			"an issue closed as completed",
			`{"__typename":"Issue","number":2,"state":"CLOSED","stateReason":"COMPLETED","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateClosed,
		},
		{
			"an issue closed as not planned",
			`{"__typename":"Issue","number":3,"state":"CLOSED","stateReason":"NOT_PLANNED","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateNotPlanned,
		},
		{
			// Being a duplicate is an edge, not a state.
			"an issue closed as a duplicate",
			`{"__typename":"Issue","number":4,"state":"CLOSED","stateReason":"DUPLICATE","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateClosed,
		},
		{
			// stateReason was added in 2022, so it is null on anything closed
			// before then. That is a plain closed issue, not "not planned".
			"an issue closed before stateReason existed",
			`{"__typename":"Issue","number":5,"state":"CLOSED","stateReason":null,"repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateClosed,
		},
		{
			"a reopened issue",
			`{"__typename":"Issue","number":6,"state":"OPEN","stateReason":"REOPENED","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateOpen,
		},
		{
			"a merged pull request",
			`{"__typename":"PullRequest","number":7,"state":"MERGED","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateMerged,
		},
		{
			"a pull request closed without merging",
			`{"__typename":"PullRequest","number":8,"state":"CLOSED","repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateClosed,
		},
		{
			"a draft pull request",
			`{"__typename":"PullRequest","number":9,"state":"OPEN","isDraft":true,"repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateDraft,
		},
		{
			"an open pull request",
			`{"__typename":"PullRequest","number":10,"state":"OPEN","isDraft":false,"repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`,
			graph.StateOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			node, err := toNode(parseItem(t, tt.body))

			// Then
			if err != nil {
				t.Fatalf("toNode: %v", err)
			}
			if node.State != tt.want {
				t.Errorf("State = %v, want %v", node.State, tt.want)
			}
		})
	}
}

func TestKindComesFromTypename(t *testing.T) {
	tests := []struct {
		name string
		// Given
		body string
		// Then
		want graph.Kind
	}{
		{"Issue", `{"__typename":"Issue","number":1,"repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`, graph.KindIssue},
		{"PullRequest", `{"__typename":"PullRequest","number":1,"repository":{"nameWithOwner":"ko-da-k/gh-relations"}}`, graph.KindPullRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			node, err := toNode(parseItem(t, tt.body))

			// Then
			if err != nil {
				t.Fatalf("toNode: %v", err)
			}
			if node.Kind != tt.want {
				t.Errorf("Kind = %v, want %v", node.Kind, tt.want)
			}
		})
	}
}

func TestLinkDirections(t *testing.T) {
	// Given: an issue that reports every link field an issue has
	body := `{
      "__typename": "Issue", "id": "I_1", "number": 42, "title": "some issue title",
      "url": "https://example.test/42", "state": "OPEN",
      "repository": { "nameWithOwner": "ko-da-k/gh-relations" },
      "parent": { "__typename": "Issue", "number": 43, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } },
      "subIssues":  { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 44, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "blockedBy":  { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 46, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "blocking":   { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 52, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "relatesTo":  { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 51, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "duplicateOf": { "__typename": "Issue", "number": 50, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } },
      "trackedIssues":   { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 55, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "trackedInIssues": { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 49, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "closedByPullRequestsReferences": { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "PullRequest", "number": 41, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "timelineItems": { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "CrossReferencedEvent", "source": {
            "__typename": "Issue", "number": 66, "repository": { "nameWithOwner": "ko-da-k/gh-relations-fixtures" } } } ] }
    }`

	// When
	obs := observe(t, body)

	tests := []struct {
		name string
		// Then: which kind of edge, pointing which way relative to #42
		other graph.Key
		kind  graph.EdgeType
		dir   graph.Direction
	}{
		{"the pull request that closes it points at it", home(41), graph.EdgeCloses, graph.Inbound},
		{"its parent points at it", home(43), graph.EdgeParent, graph.Inbound},
		{"its sub-issue points away from it", home(44), graph.EdgeParent, graph.Outbound},
		{"a blocker points at it", home(46), graph.EdgeBlocks, graph.Inbound},
		{"something it blocks points away from it", home(52), graph.EdgeBlocks, graph.Outbound},
		{"relates has no direction", home(51), graph.EdgeRelates, graph.Undirected},
		{"duplicateOf points away from the duplicate", home(50), graph.EdgeDuplicate, graph.Outbound},
		{"a task list it holds points away from it", home(55), graph.EdgeTracked, graph.Outbound},
		{"a task list it is in points at it", home(49), graph.EdgeTracked, graph.Inbound},
		{"a cross reference points at it", sibling(66), graph.EdgeMention, graph.Inbound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link, ok := findLink(obs, tt.other, tt.kind)
			if !ok {
				t.Fatalf("no %v link to %v in %+v", tt.kind, tt.other, obs.Links)
			}
			if link.Dir != tt.dir {
				t.Errorf("Dir = %v, want %v", link.Dir, tt.dir)
			}
		})
	}

	t.Run("the subject keeps its own display fields", func(t *testing.T) {
		if obs.Subject.Key != home(42) || obs.Subject.ID != "I_1" || obs.Subject.Title != "some issue title" {
			t.Errorf("subject = %+v", obs.Subject)
		}
	})
}

func TestPullRequestReportsFewerFields(t *testing.T) {
	// Given: a pull request, which has closingIssuesReferences and a timeline
	// and none of the other link fields
	body := `{
      "__typename": "PullRequest", "id": "PR_1", "number": 41, "state": "OPEN",
      "repository": { "nameWithOwner": "ko-da-k/gh-relations" },
      "closingIssuesReferences": { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "Issue", "number": 42, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] },
      "timelineItems": { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "CrossReferencedEvent", "source": {
            "__typename": "Issue", "number": 1, "repository": { "nameWithOwner": "ko-da-k/gh-relations-fixtures" } } } ] }
    }`

	// When
	obs := observe(t, body)

	// Then: the closing reference points away from the pull request
	link, ok := findLink(obs, home(42), graph.EdgeCloses)
	if !ok {
		t.Fatal("the closing reference is missing")
	}
	if link.Dir != graph.Outbound {
		t.Errorf("Dir = %v, want outbound", link.Dir)
	}
	if len(obs.Links) != 2 {
		t.Errorf("links = %d, want 2", len(obs.Links))
	}
}

func TestTimelineIsSplitByTypename(t *testing.T) {
	// Given: one timeline carrying both event types, asked for in one request
	body := `{
      "__typename": "Issue", "number": 50, "state": "OPEN",
      "repository": { "nameWithOwner": "ko-da-k/gh-relations" },
      "timelineItems": { "pageInfo": { "hasNextPage": false }, "nodes": [
        { "__typename": "CrossReferencedEvent", "source": {
            "__typename": "PullRequest", "number": 47, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } },
        { "__typename": "MarkedAsDuplicateEvent",
          "canonical": { "__typename": "Issue", "number": 50, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } },
          "duplicate": { "__typename": "Issue", "number": 53, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } },
        { "__typename": "SomethingElse" }
      ] }
    }`

	// When
	obs := observe(t, body)

	// Then: a mention from the cross reference
	if _, ok := findLink(obs, home(47), graph.EdgeMention); !ok {
		t.Error("the cross reference did not become a mention")
	}

	// and both ends of the duplicate event, with no direction. The event names
	// the timeline's owner as well; dropping that is normalization's job, not
	// the converter's, so the converter reports what the API said.
	for _, number := range []int{50, 53} {
		link, ok := findLink(obs, home(number), graph.EdgeDuplicate)
		if !ok {
			t.Fatalf("the duplicate event did not yield #%d", number)
		}
		if link.Dir != graph.Undirected {
			t.Errorf("#%d: Dir = %v, want undirected", number, link.Dir)
		}
	}

	// and an event type we did not ask about is ignored rather than failing
	if len(obs.Links) != 3 {
		t.Errorf("links = %d, want 3", len(obs.Links))
	}
}

func TestTruncatedConnectionIsReported(t *testing.T) {
	// Given: an item with more sub-issues than one page holds
	body := `{
      "__typename": "Issue", "number": 43, "state": "OPEN",
      "repository": { "nameWithOwner": "ko-da-k/gh-relations" },
      "subIssues": { "pageInfo": { "hasNextPage": true }, "nodes": [
        { "__typename": "Issue", "number": 42, "repository": { "nameWithOwner": "ko-da-k/gh-relations" } } ] }
    }`

	// When
	obs := observe(t, body)

	// Then: the first page is kept and the item is marked as incomplete.
	// Truncation is read from hasNextPage, never from totalCount, because
	// PullRequest.timelineItems reports a totalCount that ignores itemTypes.
	if !obs.Subject.PartialLinks {
		t.Error("PartialLinks should be set when a connection had another page")
	}
	if len(obs.Links) != 1 {
		t.Errorf("links = %d, want the one on the first page", len(obs.Links))
	}
}

func TestNullNodesAreSkipped(t *testing.T) {
	// Given: a nodes array with a null in it, which is what an id the account
	// cannot see comes back as
	var resp levelResponse
	body := `{ "nodes": [
      { "__typename": "Issue", "number": 42, "state": "OPEN", "repository": { "nameWithOwner": "ko-da-k/gh-relations" } },
      null,
      { "__typename": "Issue", "number": 44, "state": "OPEN", "repository": { "nameWithOwner": "ko-da-k/gh-relations" } }
    ] }`
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// When
	observations, err := toObservations(resp.Nodes)

	// Then: the readable items come through and the null is not an error
	if err != nil {
		t.Fatalf("toObservations: %v", err)
	}
	if len(observations) != 2 {
		t.Fatalf("got %d observations, want 2", len(observations))
	}
}

func TestMalformedRepositoryIsAnError(t *testing.T) {
	// Given: an item whose nameWithOwner is not an OWNER/REPO pair
	body := `{"__typename":"Issue","number":1,"repository":{"nameWithOwner":"gh-relations"}}`

	// When
	_, err := toObservation(parseItem(t, body))

	// Then
	if err == nil {
		t.Fatal("want an error for a repository that is not OWNER/REPO")
	}
}
