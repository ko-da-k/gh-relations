package cli_test

import (
	"strings"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/cli"
	"github.com/ko-da-k/gh-relations/internal/graph"
)

func TestResolveTarget(t *testing.T) {
	current := graph.Key{Owner: "ko-da-k", Repo: "gh-relations"}

	tests := []struct {
		name string
		// Given: the argument and --repo, in a checkout of ko-da-k/gh-relations
		arg  string
		repo string
		// Then: the item to start from
		want    graph.Key
		wantErr bool
	}{
		{
			name: "a bare number uses the current repository",
			arg:  "14",
			want: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 14},
		},
		{
			name: "a number may be written with a hash",
			arg:  "#14",
			want: graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 14},
		},
		{
			name: "--repo decides where a bare number is looked up",
			arg:  "14",
			repo: "ko-da-k/gh-relations-fixtures",
			want: graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: 14},
		},
		{
			name: "an issue URL carries its own repository",
			arg:  "https://github.com/another_owner/gh-relations/issues/9",
			want: graph.Key{Owner: "another_owner", Repo: "gh-relations", Number: 9},
		},
		{
			name: "a pull request URL is read the same way",
			arg:  "https://github.com/another_owner/gh-relations/pull/18",
			want: graph.Key{Owner: "another_owner", Repo: "gh-relations", Number: 18},
		},
		{
			name: "a URL with a trailing path still resolves",
			arg:  "https://github.com/another_owner/gh-relations/pull/18/files",
			want: graph.Key{Owner: "another_owner", Repo: "gh-relations", Number: 18},
		},
		{
			name: "a URL on another host works too",
			arg:  "https://github.example.test/ko-da-k/gh-relations-fixtures/issues/3",
			want: graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: 3},
		},
		{
			// A URL says which repository it means, so --repo has nothing to add.
			name: "a URL wins over --repo",
			arg:  "https://github.com/another_owner/gh-relations/issues/9",
			repo: "ko-da-k/gh-relations-fixtures",
			want: graph.Key{Owner: "another_owner", Repo: "gh-relations", Number: 9},
		},
		{
			name: "OWNER/REPO#NUMBER is accepted",
			arg:  "ko-da-k/gh-relations-fixtures#7",
			want: graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: 7},
		},
		{
			name:    "a URL that is not an Issue or pull request is rejected",
			arg:     "https://github.com/ko-da-k/gh-relations",
			wantErr: true,
		},
		{
			name:    "a word that is not a number is rejected",
			arg:     "fourteen",
			wantErr: true,
		},
		{
			name:    "--repo without a slash is rejected",
			arg:     "14",
			repo:    "gh-relations",
			wantErr: true,
		},
		{
			name:    "an empty argument is rejected",
			arg:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := cli.ResolveTarget(tt.arg, tt.repo, current)

			// Then
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("a bare number outside a repository is a helpful error", func(t *testing.T) {
		// Given: no current repository and no --repo
		// When
		_, err := cli.ResolveTarget("14", "", graph.Key{})

		// Then
		if err == nil || !strings.Contains(err.Error(), "--repo") {
			t.Errorf("err = %v, want it to mention --repo", err)
		}
	})

	t.Run("the kind in a URL path is not kept", func(t *testing.T) {
		// Given: the same number written as an issue and as a pull request.
		// issueOrPullRequest resolves a number without being told which it is,
		// and GitHub redirects between the two paths anyway.
		issue, err1 := cli.ResolveTarget("https://github.com/ko-da-k/gh-relations/issues/14", "", current)
		pull, err2 := cli.ResolveTarget("https://github.com/ko-da-k/gh-relations/pull/14", "", current)

		// Then
		if err1 != nil || err2 != nil {
			t.Fatalf("errors: %v %v", err1, err2)
		}
		if issue != pull {
			t.Errorf("%v and %v should resolve to the same item", issue, pull)
		}
	})
}
