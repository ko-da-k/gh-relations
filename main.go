// Command gh-relations draws the Issues and pull requests linked to one of
// them as a tree.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/cli/go-gh/v2/pkg/term"

	"github.com/ko-da-k/gh-relations/internal/cli"
	"github.com/ko-da-k/gh-relations/internal/gh"
	"github.com/ko-da-k/gh-relations/internal/graph"
	"github.com/ko-da-k/gh-relations/internal/render"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := run(ctx, os.Args[1:])
	switch {
	case err == nil, errors.Is(err, cli.ErrHelp):
		return
	case errors.Is(err, cli.ErrReported):
		// The flag package has already written the problem and the usage.
	default:
		fmt.Fprintf(os.Stderr, "gh relations: %v\n", err)
	}
	os.Exit(1)
}

func run(ctx context.Context, args []string) error {
	terminal := term.FromEnv()

	cfg, err := cli.ParseFlags(args, terminal.ErrOut())
	if err != nil {
		return err
	}

	root, err := resolveRoot(cfg)
	if err != nil {
		return err
	}

	client, err := gh.NewClient()
	if err != nil {
		return err
	}

	g, err := graph.Walk(ctx, client, root, cfg.Walk)
	if err != nil {
		return err
	}

	if err := render.Tree(terminal.Out(), g, renderOptions(terminal, cfg)); err != nil {
		return err
	}
	cli.Notes(terminal.ErrOut(), g, cfg.Walk)
	return nil
}

func renderOptions(terminal term.Term, cfg cli.Config) render.Options {
	width, _, err := terminal.Size()
	if err != nil || width < 1 {
		width = 0
	}
	return render.Options{
		Color: !cfg.NoColor && terminal.IsColorEnabled(),
		// An escape sequence, so only to a terminal. It does not follow
		// --no-color, which is about what carries the state.
		Hyperlinks: terminal.IsTerminalOutput(),
		Width:      width,
		Legend:     true,
	}
}

// resolveRoot works out which item to start from, falling back to the pull
// request of the branch the command was run on.
func resolveRoot(cfg cli.Config) (graph.Key, error) {
	var current graph.Key
	if repo, err := repository.Current(); err == nil {
		current = graph.Key{Owner: repo.Owner, Repo: repo.Name}
	}

	target := cfg.Target
	if target == "" {
		found, err := currentBranchPullRequest()
		if err != nil {
			return graph.Key{}, err
		}
		target = found
	}
	return cli.ResolveTarget(target, cfg.Repo, current)
}

// currentBranchPullRequest asks gh for the pull request of the current branch.
// The extension is always run by gh, so gh is on the path.
func currentBranchPullRequest() (string, error) {
	const hint = "give an Issue or pull request as a number, OWNER/REPO#N or a URL; see gh relations --help"

	path, err := exec.LookPath("gh")
	if err != nil {
		return "", fmt.Errorf("gh is not on the path, so the current branch cannot be resolved: %s", hint)
	}
	out, err := exec.Command(path, "pr", "view", "--json", "url", "--jq", ".url").Output()
	if err != nil {
		return "", fmt.Errorf("no pull request for the current branch: %s", hint)
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("no pull request for the current branch: %s", hint)
	}
	return url, nil
}
