// Package graph holds the Issue / pull request link graph: the nodes, the edges
// between them, the normalization that turns one fetched item into edges, and
// the breadth-first walk that grows the graph outwards from a root.
//
// Nothing in this package talks to the GitHub API. A caller hands it
// Observation values and gets a Graph back.
package graph

import (
	"fmt"
	"strings"
)

// Key identifies one Issue or pull request. Issues and pull requests share one
// number space within a repository, so Kind is not part of the key.
type Key struct {
	Owner  string
	Repo   string
	Number int
}

// String is the fully qualified form, "ko-da-k/gh-relations#14".
func (k Key) String() string {
	return fmt.Sprintf("%s/%s#%d", k.Owner, k.Repo, k.Number)
}

// Display writes the key as a reader of a tree rooted in home would write it,
// at one of three lengths:
//
//	#14                     the root's own repository
//	gh-relations#14         another repository of the same owner
//	another/gh-relations#14 another owner
func (k Key) Display(home Key) string {
	switch {
	case k.SameRepo(home):
		return fmt.Sprintf("#%d", k.Number)
	case k.SameOwner(home):
		return fmt.Sprintf("%s#%d", k.Repo, k.Number)
	default:
		return k.String()
	}
}

// SameRepo reports whether both keys live in one repository.
func (k Key) SameRepo(o Key) bool {
	return k.Owner == o.Owner && k.Repo == o.Repo
}

// SameOwner reports whether both keys live under one account.
func (k Key) SameOwner(o Key) bool {
	return k.Owner == o.Owner
}

// Less orders keys by owner, then repository, then number.
func (k Key) Less(o Key) bool {
	if k.Owner != o.Owner {
		return k.Owner < o.Owner
	}
	if k.Repo != o.Repo {
		return k.Repo < o.Repo
	}
	return k.Number < o.Number
}

// ParseNameWithOwner splits the API's nameWithOwner ("ko-da-k/gh-relations")
// into a key for the given number.
func ParseNameWithOwner(nameWithOwner string, number int) (Key, error) {
	owner, repo, ok := strings.Cut(nameWithOwner, "/")
	if !ok || owner == "" || repo == "" {
		return Key{}, fmt.Errorf("not an OWNER/REPO pair: %q", nameWithOwner)
	}
	return Key{Owner: owner, Repo: repo, Number: number}, nil
}
