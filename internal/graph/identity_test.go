package graph_test

import (
	"testing"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

func TestKeyDisplay(t *testing.T) {
	home := graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 14}

	tests := []struct {
		name string
		// Given: a key seen from a tree rooted in ko-da-k/gh-relations
		in graph.Key
		// Then: how a reader of that tree writes it
		want string
	}{
		{"same repository is shortened", graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 13}, "#13"},
		{"a sibling of the same owner keeps the repository and drops the owner", graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: 2}, "gh-relations-fixtures#2"},
		{"another owner is written out in full", graph.Key{Owner: "another_owner", Repo: "gh-relations", Number: 7}, "another_owner/gh-relations#7"},
		// The three forms have to stay distinguishable: a reader works out how
		// far away an item is from the length of its reference alone.
		{"a repository whose name holds no slash cannot be read as an owner", graph.Key{Owner: "ko-da-k", Repo: "gh", Number: 1}, "gh#1"},
		{"the root itself is shortened", home, "#14"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When: the key is written for that tree
			got := tt.in.Display(home)

			// Then
			if got != tt.want {
				t.Errorf("Display() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestKeyLessIsATotalOrder(t *testing.T) {
	tests := []struct {
		name string
		// Given: two keys
		a, b graph.Key
		// Then: whether a sorts before b
		want bool
	}{
		{
			"number decides inside one repository",
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 2},
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 10},
			true,
		},
		{
			"the same number in another repository is a different key",
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 2},
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations-fixtures", Number: 2},
			true,
		},
		{
			"owner outranks repository",
			graph.Key{Owner: "another_owner", Repo: "gh-relations-fixtures", Number: 1},
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 1},
			true,
		},
		{
			"a key does not sort before itself",
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 1},
			graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 1},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got := tt.a.Less(tt.b)

			// Then
			if got != tt.want {
				t.Errorf("%v.Less(%v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if tt.a != tt.b && tt.b.Less(tt.a) == got {
				t.Errorf("Less is not antisymmetric for %v and %v", tt.a, tt.b)
			}
		})
	}
}

func TestParseNameWithOwner(t *testing.T) {
	tests := []struct {
		name string
		// Given: the API's nameWithOwner and a number
		in      string
		number  int
		want    graph.Key
		wantErr bool
	}{
		{"a well formed pair", "ko-da-k/gh-relations", 14, graph.Key{Owner: "ko-da-k", Repo: "gh-relations", Number: 14}, false},
		{"no slash", "gh-relations", 14, graph.Key{}, true},
		{"empty owner", "/gh-relations", 14, graph.Key{}, true},
		{"empty repository", "ko-da-k/", 14, graph.Key{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			got, err := graph.ParseNameWithOwner(tt.in, tt.number)

			// Then
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}
