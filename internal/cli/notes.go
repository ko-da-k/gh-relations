package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ko-da-k/gh-relations/internal/graph"
)

// Notes writes what the tree cannot say for itself. It is written to stderr, so
// that a piped tree stays clean.
func Notes(w io.Writer, g *graph.Graph, opts graph.Options) {
	if g.Len() == 1 {
		fmt.Fprint(w, "No links found. GitHub reports a bare \"#14\" written in prose only from the side\n"+
			"that was referred to, so a reference this item wrote itself, with no closing keyword,\n"+
			"no task-list checkbox and no entry in the Relationships sidebar, does not appear here.\n")
		return
	}

	reasons := map[graph.StopReason]int{}
	partial := 0
	for _, n := range g.Nodes {
		if n.Stop != graph.StopNone {
			reasons[n.Stop]++
		}
		if n.PartialLinks {
			partial++
		}
	}

	var parts []string
	for reason, count := range reasons {
		parts = append(parts, fmt.Sprintf("%d by --%s", count, reason))
	}
	sort.Strings(parts)
	if len(parts) > 0 {
		fmt.Fprintf(w, "Marked \"+\": %s. Raise the limit to go further.\n", strings.Join(parts, ", "))
	}
	if partial > 0 {
		fmt.Fprintf(w, "%d item(s) have more links than one page holds; only the first page was read.\n", partial)
	}
}
