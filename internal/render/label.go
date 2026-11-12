package render

import (
	"strings"

	"github.com/rivo/uniseg"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// The state is a colour when the terminal can show one. Draft and "closed as
// not planned" share grey on purpose: the shape tells them apart.
const (
	ansiReset   = "\x1b[0m"
	ansiGreen   = "\x1b[32m"
	ansiMagenta = "\x1b[35m"
	ansiRed     = "\x1b[31m"
	ansiGrey    = "\x1b[90m"
)

// An OSC 8 hyperlink: the URL, the text the reader clicks, and an empty URL to
// close it. It costs no columns either way, since a terminal that does not know
// the sequence drops it.
//
// See https://gist.github.com/egmontkob/eb114294efbcd5adb1944c9f3cb5feda
const (
	osc8Open  = "\x1b]8;;"
	osc8Close = "\x1b\\"
)

// linkable says whether a URL may be written into an OSC 8 sequence. The URL is
// whatever the API answered with, and an OSC sequence ends at an ESC, so a URL
// carrying a control character would close the link early and write the rest
// straight to the terminal.
func linkable(url string) bool {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return false
	}
	return strings.IndexFunc(url, func(r rune) bool { return r < 0x20 || r == 0x7f }) < 0
}

func colorFor(s graph.State) string {
	switch s {
	case graph.StateMerged:
		return ansiMagenta
	case graph.StateClosed:
		return ansiRed
	case graph.StateDraft, graph.StateNotPlanned:
		return ansiGrey
	default:
		return ansiGreen
	}
}

func shapeFor(k graph.Kind) string {
	if k == graph.KindPullRequest {
		return shapePull
	}
	return shapeIssue
}

// label builds one whole line: the prefix the caller worked out, then the
// shape, the reference, the state when there is no colour to carry it, and the
// title. The hyperlink covers the reference alone, which is what names the item.
func (t *tree) label(prefix string, n *graph.Node, backRef bool) string {
	shape := shapeFor(n.Kind)
	ref := n.Key.Display(t.g.Root)
	t.kinds[n.Kind] = true
	t.states[n.State] = true

	state := ""
	separator := "  "
	if !t.opts.Color {
		// Left aligned, so that no column has to line up.
		state = " " + n.State.String() + ":"
		separator = " "
	}

	suffix := ""
	switch {
	case backRef:
		suffix = "  " + markerBack
		t.markers[markerBack] = true
	case n.Stop != graph.StopNone:
		suffix = "  " + markerUnxpd
		t.markers[markerUnxpd] = true
	}

	title := strings.TrimSpace(n.Title)
	if t.opts.Width > 0 {
		spent := pessimisticWidth(prefix+shape+" "+ref+state+separator) + pessimisticWidth(suffix)
		title = truncate(title, t.opts.Width-spent)
	}

	link := t.opts.Hyperlinks && linkable(n.URL)
	var b strings.Builder
	b.WriteString(prefix)
	if t.opts.Color {
		b.WriteString(colorFor(n.State))
	}
	b.WriteString(shape)
	b.WriteString(" ")
	if link {
		b.WriteString(osc8Open + n.URL + osc8Close)
	}
	b.WriteString(ref)
	if link {
		b.WriteString(osc8Open + osc8Close)
	}
	b.WriteString(state)
	if t.opts.Color {
		b.WriteString(ansiReset)
	}
	if title != "" {
		b.WriteString(separator)
		b.WriteString(title)
	}
	b.WriteString(suffix)
	return b.String()
}

// ambiguousGlyphs are the only East Asian Ambiguous runes this package writes:
// the three connectors and the two shapes. It is written out rather than looked
// up because this package emits a closed alphabet; an internal test checks the
// list against the Unicode data in both directions.
const ambiguousGlyphs = "├└│◯◇"

// pessimisticWidth counts every East Asian Ambiguous rune as two columns.
// uniseg fixes them at one, but a terminal set to the East Asian legacy width
// draws them in two, so a line is budgeted for the worse of the two.
func pessimisticWidth(s string) int {
	total := 0
	for _, r := range s {
		if strings.ContainsRune(ambiguousGlyphs, r) {
			total += 2
			continue
		}
		total += uniseg.StringWidth(string(r))
	}
	return total
}

// truncate cuts a title to fit, measured in display columns and cut between
// grapheme clusters so that no character is broken in half.
func truncate(s string, maxWidth int) string {
	const ellipsis = "..."
	if maxWidth <= 0 {
		return ""
	}
	if uniseg.StringWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= len(ellipsis) {
		return strings.Repeat(".", maxWidth)
	}

	budget := maxWidth - len(ellipsis)
	var b strings.Builder
	used := 0
	state := -1
	rest := s
	for len(rest) > 0 {
		var cluster string
		var clusterWidth int
		cluster, rest, clusterWidth, state = uniseg.FirstGraphemeClusterInString(rest, state)
		if used+clusterWidth > budget {
			break
		}
		b.WriteString(cluster)
		used += clusterWidth
	}
	return b.String() + ellipsis
}
