package cli_test

import (
	"io"
	"strings"
	"testing"

	"github.com/ko-da-k/gh-relations/internal/cli"
	"github.com/ko-da-k/gh-relations/internal/graph"
)

func TestParseFlagsDefaults(t *testing.T) {
	// Given: one argument and nothing else
	// When
	cfg, err := cli.ParseFlags([]string{"18"}, io.Discard)

	// Then: the documented defaults
	if err != nil {
		t.Fatalf("ParseFlags: %v", err)
	}
	if cfg.Target != "18" {
		t.Errorf("Target = %q, want \"18\"", cfg.Target)
	}
	if cfg.Walk.Depth != 4 {
		t.Errorf("Depth = %d, want 4", cfg.Walk.Depth)
	}
	if cfg.Walk.Scope != graph.ScopeOrg {
		t.Errorf("Scope = %v, want org", cfg.Walk.Scope)
	}
	if cfg.Walk.MaxNodes != 50 {
		t.Errorf("MaxNodes = %d, want 50", cfg.Walk.MaxNodes)
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name string
		// Given: the arguments
		args []string
		// Then
		check   func(*testing.T, cli.Config)
		wantErr bool
	}{
		{
			name: "no argument means the current branch's pull request",
			args: nil,
			check: func(t *testing.T, cfg cli.Config) {
				if cfg.Target != "" {
					t.Errorf("Target = %q, want it empty", cfg.Target)
				}
			},
		},
		{
			name: "the scope is read by name",
			args: []string{"--scope", "all", "18"},
			check: func(t *testing.T, cfg cli.Config) {
				if cfg.Walk.Scope != graph.ScopeAll {
					t.Errorf("Scope = %v, want all", cfg.Walk.Scope)
				}
			},
		},
		{
			name: "depth zero is allowed and means the root alone",
			args: []string{"--depth", "0", "18"},
			check: func(t *testing.T, cfg cli.Config) {
				if cfg.Walk.Depth != 0 {
					t.Errorf("Depth = %d, want 0", cfg.Walk.Depth)
				}
			},
		},
		{
			// The flag package stops at the first argument that is not a flag,
			// so without reordering this would read --depth as a second item.
			name: "flags may follow the item",
			args: []string{"18", "--depth", "2", "--no-color"},
			check: func(t *testing.T, cfg cli.Config) {
				if cfg.Target != "18" {
					t.Errorf("Target = %q, want \"18\"", cfg.Target)
				}
				if cfg.Walk.Depth != 2 {
					t.Errorf("Depth = %d, want 2", cfg.Walk.Depth)
				}
				if !cfg.NoColor {
					t.Error("NoColor should be set")
				}
			},
		},
		{
			name: "an item that starts with a dash can be given after --",
			args: []string{"--depth", "2", "--", "18"},
			check: func(t *testing.T, cfg cli.Config) {
				if cfg.Target != "18" {
					t.Errorf("Target = %q, want \"18\"", cfg.Target)
				}
			},
		},
		{
			name:    "an unknown scope is rejected",
			args:    []string{"--scope", "universe", "18"},
			wantErr: true,
		},
		{
			name:    "an unknown flag is rejected",
			args:    []string{"--verbose", "18"},
			wantErr: true,
		},
		{
			name:    "a negative depth is rejected",
			args:    []string{"--depth", "-1", "18"},
			wantErr: true,
		},
		{
			name:    "two items are rejected",
			args:    []string{"18", "14"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			cfg, err := cli.ParseFlags(tt.args, io.Discard)

			// Then
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestUsage(t *testing.T) {
	// Given: the help text
	var out strings.Builder
	_, err := cli.ParseFlags([]string{"-h"}, &out)
	if err != cli.ErrHelp {
		t.Fatalf("-h returned %v, want ErrHelp", err)
	}
	help := out.String()

	tests := []struct {
		// Given: what a reader has to be able to find in the help
		name string
		want []string
	}{
		{
			// The help has to state what cannot be seen.
			"the blind spot",
			[]string{"CANNOT SEE", "referred to"},
		},
		{
			"every flag",
			[]string{"--repo", "--depth", "--scope", "--max-nodes", "--no-color", "--help"},
		},
		{
			// A reader who has only ever written a bare number should find
			// the other three forms without guessing.
			"every form the argument takes",
			[]string{"ARGUMENTS", "#14", "ko-da-k/gh-relations#14", "/issues/14", "/pull/14", "current branch"},
		},
		{
			// The tree is unreadable without a key to the glyphs, and the
			// legend under the tree only lists the ones that turned up.
			"what the output means",
			[]string{"OUTPUT", "◯ issue", "◇ pull request", "▸ closes", "@ mentions",
				"+ not expanded", "↱ already drawn", "green open", "purple merged", "red closed", "grey",
				"clickable"},
		},
		{
			"runnable examples",
			[]string{"EXAMPLES", "gh relations 14"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When / Then
			for _, want := range tt.want {
				if !strings.Contains(help, want) {
					t.Errorf("the usage text does not mention %q", want)
				}
			}
		})
	}
}
