// Package cli turns command-line input into a walk, and the walk's result into
// what the user sees.
package cli

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// urlPath matches the path of an Issue or pull request URL on any host, so that
// a link pasted from the browser works from any directory.
var urlPath = regexp.MustCompile(`^/([^/]+)/([^/]+)/(?:issues|pull)/(\d+)(?:/.*)?$`)

// qualified matches "OWNER/REPO#14".
var qualified = regexp.MustCompile(`^([^/#\s]+)/([^/#\s]+)#(\d+)$`)

// ResolveTarget works out which item to start from. A URL and an OWNER/REPO#N
// say which repository they mean, so they win over --repo; a bare number takes
// --repo, and failing that the repository the command was run in.
func ResolveTarget(arg, repoFlag string, current graph.Key) (graph.Key, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return graph.Key{}, fmt.Errorf("no Issue or pull request given")
	}

	if key, ok, err := parseURL(arg); ok || err != nil {
		return key, err
	}
	if m := qualified.FindStringSubmatch(arg); m != nil {
		number, _ := strconv.Atoi(m[3])
		return graph.Key{Owner: m[1], Repo: m[2], Number: number}, nil
	}

	number, err := strconv.Atoi(strings.TrimPrefix(arg, "#"))
	if err != nil || number <= 0 {
		return graph.Key{}, fmt.Errorf("%q is not a number, an OWNER/REPO#NUMBER or an Issue or pull request URL", arg)
	}

	home := current
	if repoFlag != "" {
		owner, repo, ok := strings.Cut(repoFlag, "/")
		if !ok || owner == "" || repo == "" {
			return graph.Key{}, fmt.Errorf("--repo wants OWNER/REPO, got %q", repoFlag)
		}
		home = graph.Key{Owner: owner, Repo: repo}
	}
	if home.Owner == "" || home.Repo == "" {
		return graph.Key{}, fmt.Errorf("no repository to look up #%d in: run this inside a repository, or pass --repo OWNER/REPO", number)
	}
	home.Number = number
	return home, nil
}

// parseURL reads an Issue or pull request URL. looksLikeURL keeps a plain
// number from being reported as a broken URL.
func parseURL(arg string) (key graph.Key, looksLikeURL bool, err error) {
	if !strings.Contains(arg, "://") {
		return graph.Key{}, false, nil
	}
	u, err := url.Parse(arg)
	if err != nil {
		return graph.Key{}, true, fmt.Errorf("could not read the URL %q: %w", arg, err)
	}
	m := urlPath.FindStringSubmatch(u.Path)
	if m == nil {
		return graph.Key{}, true, fmt.Errorf("%q is not an Issue or pull request URL", arg)
	}
	number, _ := strconv.Atoi(m[3])
	// The kind in the path is not kept: issueOrPullRequest resolves a number
	// without it, and GitHub redirects between the two paths anyway.
	return graph.Key{Owner: m[1], Repo: m[2], Number: number}, true, nil
}
