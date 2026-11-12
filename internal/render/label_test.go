package render_test

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
	"golang.org/x/text/width"

	"github.com/ko-da-k/gh-relations/internal/graph"
	"github.com/ko-da-k/gh-relations/internal/graph/graphtest"
	"github.com/ko-da-k/gh-relations/internal/render"
)

// twoNodes builds a root with one child on a closes edge, so a test can choose
// both nodes' kinds and states.
func twoNodes(root, child graph.Node) *graph.Graph {
	g := graph.NewGraph(root.Key)
	g.Add(root)
	g.Add(child)
	e := graph.Edge{From: child.Key, To: root.Key, Type: graph.EdgeCloses}
	g.AddEdge(e, true)
	g.Via[child.Key] = e
	return g
}

func TestShapeIsTheKind(t *testing.T) {
	tests := []struct {
		name string
		// Given: a node of some kind
		kind graph.Kind
		// Then: the glyph it is drawn with
		want string
	}{
		{"an issue is a circle", graph.KindIssue, "◯"},
		{"a pull request is a diamond", graph.KindPullRequest, "◇"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			g := twoNodes(
				graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root"},
				graph.Node{Key: graphtest.A(2), Kind: tt.kind, Title: "child", Hop: 1},
			)

			// When
			got := draw(t, g, render.Options{})

			// Then
			if !strings.Contains(lineWith(got, "#2"), tt.want+" #2") {
				t.Errorf("want the child drawn with %q:\n%s", tt.want, got)
			}
		})
	}
}

func TestStateIsTheColour(t *testing.T) {
	const (
		green   = "\x1b[32m"
		magenta = "\x1b[35m"
		red     = "\x1b[31m"
		grey    = "\x1b[90m"
	)

	tests := []struct {
		name string
		// Given: a node in some state
		kind  graph.Kind
		state graph.State
		// Then: the colour it is drawn in
		want string
	}{
		{"open is green", graph.KindIssue, graph.StateOpen, green},
		{"merged is purple", graph.KindPullRequest, graph.StateMerged, magenta},
		{"closed is red", graph.KindIssue, graph.StateClosed, red},
		// Grey carries two states; the shape is what separates them.
		{"a draft pull request is grey", graph.KindPullRequest, graph.StateDraft, grey},
		{"an issue closed as not planned is the same grey", graph.KindIssue, graph.StateNotPlanned, grey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			g := twoNodes(
				graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root"},
				graph.Node{Key: graphtest.A(2), Kind: tt.kind, State: tt.state, Title: "child", Hop: 1},
			)

			// When
			got := lineWith(draw(t, g, render.Options{Color: true}), "#2")

			// Then: the colour is there and the state is not spelled out
			if !strings.Contains(got, tt.want) {
				t.Errorf("line %q does not carry the colour %q", got, tt.want)
			}
			if strings.Contains(got, tt.state.String()+":") {
				t.Errorf("line %q spells out a state the colour already carries", got)
			}
		})
	}

	t.Run("the shape separates the two grey states", func(t *testing.T) {
		// Given: a draft pull request and an issue closed as not planned
		g := twoNodes(
			graph.Node{Key: graphtest.A(1), Kind: graph.KindPullRequest, State: graph.StateDraft, Title: "draft"},
			graph.Node{Key: graphtest.A(2), Kind: graph.KindIssue, State: graph.StateNotPlanned, Title: "not planned", Hop: 1},
		)

		// When
		got := draw(t, g, render.Options{Color: true})

		// Then
		if !strings.Contains(got, "◇ #1") || !strings.Contains(got, "◯ #2") {
			t.Errorf("the two grey nodes should differ by shape:\n%s", got)
		}
	})
}

func TestWithoutColourTheStateIsWrittenIn(t *testing.T) {
	tests := []struct {
		name string
		// Given: a state
		state graph.State
		// Then: how it reads inline
		want string
	}{
		{"open", graph.StateOpen, "◯ #2 open: child"},
		{"closed", graph.StateClosed, "◯ #2 closed: child"},
		{"not planned", graph.StateNotPlanned, "◯ #2 not planned: child"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			g := twoNodes(
				graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root"},
				graph.Node{Key: graphtest.A(2), Kind: graph.KindIssue, State: tt.state, Title: "child", Hop: 1},
			)

			// When
			got := draw(t, g, render.Options{})

			// Then: left aligned, so no column has to line up
			if !strings.Contains(got, tt.want) {
				t.Errorf("want %q in:\n%s", tt.want, got)
			}
			if strings.Contains(got, "\x1b[") {
				t.Error("no escape sequence should be written without colour")
			}
		})
	}
}

func TestSigilsSurviveWithoutColour(t *testing.T) {
	// Given: the sigil is the only thing that says which kind of link a line is,
	// so unlike the colour it is never dropped
	// When
	got := drawSample(t, graphtest.A(2), 4, render.Options{})

	// Then
	for _, sigil := range []string{"▸", "▴", "▾", "⊘", "~", "=", "▪", "@"} {
		if !strings.Contains(got, sigil) {
			t.Errorf("sigil %q is missing from the plain tree", sigil)
		}
	}
}

func TestEverySigilIsOneColumnEverywhere(t *testing.T) {
	// Given: every glyph that can follow an indent
	//
	// The shape comes right after the sigil, so a sigil whose width changes with
	// the terminal's East Asian setting would put sibling shapes in different
	// columns. That is the condition this pins down; the shapes and connectors
	// are Ambiguous on purpose and are handled by the indent being uniform.
	glyphs := []string{"▸", "▴", "▾", "⊘", "⊚", "~", "=", "▪", "▫", "@", "↱", "+"}

	for _, g := range glyphs {
		t.Run(g, func(t *testing.T) {
			// When
			for _, r := range g {
				kind := width.LookupRune(r).Kind()

				// Then
				if kind == width.EastAsianAmbiguous {
					t.Errorf("U+%04X is Ambiguous, so its width depends on the terminal", r)
				}
			}
			if got := uniseg.StringWidth(g); got != 1 {
				t.Errorf("width = %d, want 1", got)
			}
		})
	}
}

func TestLegend(t *testing.T) {
	t.Run("it explains the shapes, the sigils and the markers that appeared", func(t *testing.T) {
		// Given: a tree one level deep from an issue, which has no relates,
		// duplicate or tracked link, and whose children are all unexpanded
		// When
		got := drawSample(t, graphtest.A(3), 1, render.Options{Legend: true})

		// Then: one labelled row per kind of glyph, and nothing for a kind
		// that did not appear
		legend := strings.TrimSpace(got[strings.LastIndex(got, "\n\n"):])
		want := "kind   ◯ issue  ◇ pull request\n" +
			// The sigils are a list of adjectives without this line: a bare
			// "blocked" does not say which of the two lines it is about, and
			// GitHub's own sidebar groups the other way round.
			"link   what this line is to its parent above:\n" +
			"       ▸ closes/closed by  ▴ parent of  ⊘ blocks  @ mentions\n" +
			"mark   + not expanded"
		if legend != want {
			t.Errorf("legend =\n%s\nwant\n%s", legend, want)
		}
	})

	t.Run("without colour there is no colour row, because every line says its state", func(t *testing.T) {
		// Given / When
		got := drawSample(t, graphtest.A(3), 1, render.Options{Legend: true})

		// Then
		if strings.Contains(got, "state ") {
			t.Errorf("a colour key was written for an uncoloured tree:\n%s", got)
		}
	})

	t.Run("with colour the states that appeared are listed in their colour", func(t *testing.T) {
		// Given: a walk deep enough to reach a merged pull request, a draft
		// one and an issue closed as not planned
		// When
		got := drawSample(t, graphtest.A(2), 4, render.Options{Legend: true, Color: true})

		// Then: the key names each state once, in its own colour, so that the
		// reader can tell which colour meant what
		legend := got[strings.LastIndex(got, "\n\n"):]
		row := lineWith(legend, "state ")
		want := "state  " +
			"\x1b[32mopen\x1b[0m  " +
			"\x1b[35mmerged\x1b[0m  " +
			"\x1b[31mclosed\x1b[0m  " +
			"\x1b[90mdraft\x1b[0m  " +
			"\x1b[90mnot planned\x1b[0m"
		if row != want {
			t.Errorf("state row = %q, want %q", row, want)
		}
	})

	t.Run("a tree with no sigils has no legend at all", func(t *testing.T) {
		// Given: a graph of one node. There is no link to explain, so a key
		// would be longer than the output it explains -- not even the shape
		// row is written.
		g := graph.NewGraph(graphtest.A(1))
		g.Add(graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "alone"})

		// When
		got := draw(t, g, render.Options{Legend: true})

		// Then
		if got != "◯ #1 open: alone\n" {
			t.Errorf("got %q, want the single line and nothing else", got)
		}
	})

	t.Run("it is left out when it was not asked for", func(t *testing.T) {
		// Given / When
		got := drawSample(t, graphtest.A(3), 1, render.Options{})

		// Then
		if strings.Contains(got, "\nkind   ") {
			t.Errorf("a legend was written without being asked for:\n%s", got)
		}
	})
}

func TestTitleIsTruncatedByDisplayWidth(t *testing.T) {
	// Given: a title of full-width characters, which are two columns each and
	// three bytes each, so neither a byte count nor a rune count would do
	long := strings.Repeat("あ", 40)
	g := twoNodes(
		graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root"},
		graph.Node{Key: graphtest.A(2), Kind: graph.KindIssue, Title: long, Hop: 1},
	)

	// When
	got := lineWith(draw(t, g, render.Options{Width: 40}), "#2")

	// Then: it fits, even counting every Ambiguous glyph in the prefix as two
	// columns, and it says it was cut
	if !strings.HasSuffix(got, "...") {
		t.Errorf("line %q should end in an ellipsis", got)
	}
	if w := uniseg.StringWidth(got); w > 40 {
		t.Errorf("line is %d columns, want at most 40: %q", w, got)
	}
	if strings.Contains(got, "\ufffd") {
		t.Error("a character was cut in half")
	}
}

// stripEscapes removes every escape sequence, which is what a terminal that
// understands them leaves on screen.
func stripEscapes(s string) string {
	var b strings.Builder
	for {
		start := strings.IndexByte(s, 0x1b)
		if start < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:start])
		rest := s[start:]
		switch {
		case strings.HasPrefix(rest, "\x1b]"):
			// An OSC runs until ST, which is ESC \. The URL inside it may hold
			// any other character, so nothing shorter than ST will do.
			end := strings.Index(rest[2:], "\x1b\\")
			if end < 0 {
				return b.String()
			}
			s = rest[2+end+2:]
		case strings.HasPrefix(rest, "\x1b["):
			// A CSI runs until a byte in 0x40..0x7e, which for a colour is "m".
			end := strings.IndexFunc(rest[2:], func(r rune) bool { return r >= 0x40 && r <= 0x7e })
			if end < 0 {
				return b.String()
			}
			s = rest[2+end+1:]
		default:
			return b.String()
		}
	}
}

func TestReferenceIsATerminalHyperlink(t *testing.T) {
	// Given: an item whose URL the API answered with
	g := twoNodes(
		graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root", URL: "https://example.test/ko-da-k/gh-relations/issues/1"},
		graph.Node{Key: graphtest.A(2), Kind: graph.KindIssue, Title: "child", Hop: 1, URL: "https://example.test/ko-da-k/gh-relations/issues/2"},
	)

	t.Run("it is left plain when it was not asked for", func(t *testing.T) {
		// Given / When
		got := draw(t, g, render.Options{})

		// Then: a redirected or piped tree carries no escape at all
		if strings.ContainsRune(got, 0x1b) {
			t.Errorf("an escape sequence was written without being asked for: %q", got)
		}
	})

	t.Run("the link covers the reference and nothing else", func(t *testing.T) {
		// Given: a colour tree, where the state needs no word of its own
		// When
		got := lineWith(draw(t, g, render.Options{Hyperlinks: true, Color: true}), "#2")

		// Then: the reference is the only click target. It is what names the
		// item, and a link over the title as well would make most of the line
		// clickable for no more use. The whole line is stated outright, because
		// what matters here is where the two sequences fall relative to the
		// colour and the title, which a Contains check would not pin down.
		const url = "https://example.test/ko-da-k/gh-relations/issues/2"
		want := "└▸ " + "\x1b[32m" + "◯ " +
			"\x1b]8;;" + url + "\x1b\\" + "#2" + "\x1b]8;;\x1b\\" +
			"\x1b[0m" + "  child"
		if got != want {
			t.Errorf("line = %q, want %q", got, want)
		}
		if strings.Count(got, "\x1b]8;;") != 2 {
			t.Errorf("line = %q, want exactly one hyperlink", got)
		}
		if seen := stripEscapes(got); seen != "└▸ ◯ #2  child" {
			t.Errorf("what the terminal shows = %q, want the line unchanged", seen)
		}
	})

	t.Run("a colourless tree is still clickable", func(t *testing.T) {
		// Given: --no-color, which moves the state into the line and is a
		// separate switch, so it must not take the link with it
		// When
		got := lineWith(draw(t, g, render.Options{Hyperlinks: true}), "#2")

		// Then
		if !strings.Contains(got, "\x1b]8;;") {
			t.Errorf("line = %q, want a hyperlink", got)
		}
		if seen := stripEscapes(got); seen != "└▸ ◯ #2 open: child" {
			t.Errorf("what the terminal shows = %q, want the state still written in", seen)
		}
	})
}

func TestOnlyAnHTTPURLIsLinked(t *testing.T) {
	tests := []struct {
		name string
		// Given: whatever the API answered with as the item's URL
		url string
	}{
		{"a file URL", "file:///etc/passwd"},
		{"a custom scheme", "javascript:alert(1)"},
		{"no URL at all", ""},
		// An OSC sequence ends at an ESC, so a URL holding one could close the
		// link early and write the rest straight to the terminal.
		{"an escape inside the URL", "https://example.test/a\x1b\\;echo hi"},
		{"a newline inside the URL", "https://example.test/a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			g := twoNodes(
				graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root"},
				graph.Node{Key: graphtest.A(2), Kind: graph.KindIssue, Title: "child", Hop: 1, URL: tt.url},
			)

			// When
			got := lineWith(draw(t, g, render.Options{Hyperlinks: true}), "#2")

			// Then: the line is drawn, with no sequence opened for it
			if strings.Contains(got, "\x1b]8;;") {
				t.Errorf("line = %q, want no hyperlink for %q", got, tt.url)
			}
			if !strings.Contains(got, "#2") {
				t.Errorf("line = %q, want the reference still drawn", got)
			}
		})
	}
}

func TestHyperlinksDoNotMoveTheTruncation(t *testing.T) {
	// Given: a title that has to be cut, and a URL long enough that counting it
	// as text would eat the whole budget
	long := strings.Repeat("あ", 40)
	g := twoNodes(
		graph.Node{Key: graphtest.A(1), Kind: graph.KindIssue, Title: "root"},
		graph.Node{Key: graphtest.A(2), Kind: graph.KindIssue, Title: long, Hop: 1,
			URL: "https://example.test/ko-da-k/gh-relations/issues/2"},
	)

	// When
	plain := lineWith(draw(t, g, render.Options{Width: 40}), "#2")
	linked := lineWith(draw(t, g, render.Options{Width: 40, Hyperlinks: true}), "#2")

	// Then: an escape sequence costs no columns, so the reader sees the same
	// line -- and the same line is the one the budget allows, which is stated
	// outright rather than by comparison, because measuring the URL as text
	// would shorten both of them by the same amount.
	want := "\u2514\u25b8 \u25ef #2 open: " + strings.Repeat("\u3042", 10) + "..."
	if plain != want {
		t.Errorf("without links the line is %q, want %q", plain, want)
	}
	if got := stripEscapes(linked); got != want {
		t.Errorf("with links the terminal shows %q, want %q", got, want)
	}
}
