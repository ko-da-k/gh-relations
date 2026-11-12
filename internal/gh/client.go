package gh

import (
	"context"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

var _ graph.Fetcher = (*Client)(nil)

// Client fetches the graph from GitHub. It satisfies graph.Fetcher.
type Client struct {
	gql *api.GraphQLClient

	// IDsPerRequest is the batch size for nodes(ids:). A level with more ids
	// than this is split.
	IDsPerRequest int
}

// NewClient builds a client from the host and token gh is already configured
// with.
func NewClient() (*Client, error) {
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("could not reach the GitHub API: %w", err)
	}
	return &Client{gql: gql, IDsPerRequest: defaultIDsPerRequest}, nil
}

type rootResponse struct {
	Repository struct {
		IssueOrPullRequest *item `json:"issueOrPullRequest"`
	} `json:"repository"`
}

type levelResponse struct {
	Nodes []*item `json:"nodes"`
}

// FetchRoot resolves the starting item.
func (c *Client) FetchRoot(ctx context.Context, root graph.Key) (graph.Batch, error) {
	var resp rootResponse
	variables := map[string]any{
		"owner":  root.Owner,
		"name":   root.Repo,
		"number": root.Number,
	}
	if err := c.gql.DoWithContext(ctx, rootQuery, variables, &resp); err != nil {
		return graph.Batch{}, fmt.Errorf("looking up %s: %w", root, err)
	}

	batch := graph.Batch{Requests: 1}
	if resp.Repository.IssueOrPullRequest == nil {
		return batch, nil
	}
	observations, err := toObservations([]*item{resp.Repository.IssueOrPullRequest})
	if err != nil {
		return graph.Batch{}, err
	}
	batch.Observations = observations
	return batch, nil
}

// FetchLevel fetches a whole level, in one request unless the ids have to be
// split over several.
func (c *Client) FetchLevel(ctx context.Context, nodes []graph.Node) (graph.Batch, error) {
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.ID != "" {
			ids = append(ids, n.ID)
		}
	}

	var batch graph.Batch
	for chunk := range chunks(ids, c.batchSize()) {
		var resp levelResponse
		if err := c.gql.DoWithContext(ctx, levelQuery, map[string]any{"ids": chunk}, &resp); err != nil {
			return graph.Batch{}, fmt.Errorf("fetching a level of %d items: %w", len(chunk), err)
		}
		batch.Requests++
		observations, err := toObservations(resp.Nodes)
		if err != nil {
			return graph.Batch{}, err
		}
		batch.Observations = append(batch.Observations, observations...)
	}
	return batch, nil
}

func (c *Client) batchSize() int {
	if c.IDsPerRequest > 0 {
		return c.IDsPerRequest
	}
	return defaultIDsPerRequest
}

// chunks splits a slice into runs of at most size.
func chunks(ids []string, size int) func(func([]string) bool) {
	return func(yield func([]string) bool) {
		for start := 0; start < len(ids); start += size {
			end := min(start+size, len(ids))
			if !yield(ids[start:end]) {
				return
			}
		}
	}
}
