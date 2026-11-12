package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// ErrHelp is returned when the user asked for the usage text, which has already
// been written. It is not a failure.
var ErrHelp = flag.ErrHelp

// ErrReported wraps an error the flag package has already written out together
// with the usage text, so that the caller does not print it twice.
var ErrReported = errors.New("already reported")

// Config is one invocation of the command.
type Config struct {
	// Target is the argument naming the Issue or pull request, empty when the
	// command should fall back to the current branch's pull request.
	Target string

	// Repo is --repo, used only when Target is a bare number.
	Repo string

	// Walk is how far and where to follow links.
	Walk graph.Options

	// NoColor writes the state into the line instead of colouring it.
	NoColor bool
}

const usage = `Draw the Issues and pull requests linked to one of them as a tree.

USAGE
  gh relations [<item>] [flags]

ARGUMENTS
  <item>  the Issue or pull request to start from, written in any of these
          ways:

            14                                                 in the current repository
            #14                                                the same, with a hash
            ko-da-k/gh-relations#14                            in another repository
            https://github.com/ko-da-k/gh-relations/issues/14  by URL
            https://github.com/ko-da-k/gh-relations/pull/14    by URL

          An Issue and a pull request never share a number within a
          repository, so the kind is never needed: /issues/14 and /pull/14
          name the same item. With no <item> at all, the pull request of the
          current branch is used.

FLAGS
  --repo OWNER/REPO     the repository a bare number is looked up in. Defaults
                        to the repository of the current directory. A URL or
                        an OWNER/REPO#N argument says which repository it
                        means, so it wins over this
  --depth N             how many hops out to expand (default 4). Every link
                        kind counts as one hop, so the number is the length of
                        the path and not a count of any one kind
  --scope repo|org|all  where expanding may happen (default org). "org" means
                        the same owner. An item outside the scope is still
                        drawn, it is only left unexpanded, so a narrower scope
                        can only cost fewer requests
  --max-nodes N         stop once this many items are in the graph (default
                        50). This is the one bound that can leave an edge
                        pointing at an item that is not drawn
  --no-color            write the state as a word in every line instead of as
                        a colour. This also happens on its own when the output
                        is not a terminal, or when NO_COLOR is set
  -h, --help            show this text

OUTPUT
  One line is one item, and the glyph in front of it says what that item is to
  its parent, the line above it. The subject is the item on the line and the
  object is the item above, which is the opposite way round from GitHub's
  Relationships sidebar, where the headings say what the item being viewed is.

    ◯ issue            ◇ pull request
    ▸ closes/closed by (the ◇ closes the ◯)
    ▴ parent of        ▾ sub of
    ⊘ blocks           ⊚ blocked by
    ~ relates to       = duplicate (either way; the API does not always say)
    ▪ tracks           ▫ tracked in
    @ mentions
    + not expanded, stopped by --depth, --scope or --max-nodes
    ↱ already drawn higher up; its own links are under that first copy

  Two items joined by several links at once are one line with the sigils
  stacked, "▾▫", rather than one row per link. The extra links land on the
  line that is already there, so an item is not drawn again under its own
  child -- bar a bare reference the parent itself wrote, which has nowhere else
  to go.

  Colour carries the state: green open, purple merged, red closed, and grey
  for both draft and closed as not planned, which the shape tells apart. A
  legend of just the glyphs that appeared is printed under the tree.

  A reference is as short as it can be while still saying where the item is:
  "#14" in the starting item's repository, "gh-relations#14" under the same
  owner, "another/gh-relations#14" anywhere else.

  Every reference is a link to the item it names, which a terminal that
  supports OSC 8 hyperlinks makes clickable -- the reference, not the whole
  line. Which modifier does it is up to the terminal, usually Cmd+click on
  macOS, and Terminal.app ignores these links altogether. --no-color does not
  turn this off; writing to anything but a terminal does.

  The tree goes to stdout and everything else to stderr, so a pipe or a
  redirect gets the tree on its own.

EXAMPLES
  gh relations 14
  gh relations ko-da-k/gh-relations#14 --depth 2
  gh relations https://github.com/ko-da-k/gh-relations/pull/18 --no-color
  gh relations 14 --scope all --max-nodes 100

WHAT IT CANNOT SEE
  GitHub reports a bare "#14" written in prose only from the side that was
  referred to. A reference the starting item itself wrote, with no closing
  keyword, no task-list checkbox and no entry in the Relationships sidebar,
  therefore does not appear under it. Walking one more hop usually finds such
  a reference by another route.
`

// valueFlags are the flags that take a separate argument, which is what lets
// the item be written before or after the flags, as gh's own commands accept it.
var valueFlags = []string{"repo", "depth", "scope", "max-nodes"}

// ParseFlags reads one invocation. The returned error is already worded for the
// user, unless it wraps ErrReported or is ErrHelp.
func ParseFlags(args []string, stderr io.Writer) (Config, error) {
	cfg := Config{Walk: graph.DefaultOptions()}

	fs := flag.NewFlagSet("gh relations", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprint(stderr, usage) }

	var scope string
	fs.StringVar(&cfg.Repo, "repo", "", "")
	fs.IntVar(&cfg.Walk.Depth, "depth", cfg.Walk.Depth, "")
	fs.StringVar(&scope, "scope", cfg.Walk.Scope.String(), "")
	fs.IntVar(&cfg.Walk.MaxNodes, "max-nodes", cfg.Walk.MaxNodes, "")
	fs.BoolVar(&cfg.NoColor, "no-color", false, "")

	flagArgs, positional := splitArgs(args)
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Config{}, ErrHelp
		}
		return Config{}, fmt.Errorf("%w: %w", ErrReported, err)
	}

	parsedScope, err := graph.ParseScope(scope)
	if err != nil {
		return Config{}, err
	}
	cfg.Walk.Scope = parsedScope

	if len(positional) > 1 {
		return Config{}, fmt.Errorf("want one Issue or pull request, got %d: %s", len(positional), strings.Join(positional, " "))
	}
	if len(positional) == 1 {
		cfg.Target = positional[0]
	}

	if err := cfg.Walk.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// splitArgs separates the flags from the item. The flag package stops at the
// first argument that is not a flag, so pulling the item out first is what lets
// it sit anywhere on the line.
func splitArgs(args []string) (flagArgs, positional []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			return flagArgs, positional
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flagArgs = append(flagArgs, arg)

		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		if slices.Contains(valueFlags, name) && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return flagArgs, positional
}
