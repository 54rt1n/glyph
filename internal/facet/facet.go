// Package facet renders glyphs at progressive disclosure levels. All
// commands share these renderers so density logic never forks.
package facet

import (
	"fmt"
	"strings"

	"github.com/54rt1n/glyph/internal/types"
)

const pinLineWidth = 72

// DisplayLine returns a glyph's explicit summary, falling back to the first
// body line for glyphs created before summaries existed.
func DisplayLine(g *types.Glyph, width int) string {
	if summary := strings.TrimSpace(g.Summary); summary != "" {
		return FirstLine(summary, width)
	}
	return FirstLine(g.Body, width)
}

// Pin renders the one-line view: focus marker, id, type, summary, refs, score.
func Pin(g *types.Glyph) string {
	var b strings.Builder
	if g.Starred {
		b.WriteString("★ ")
	}
	fmt.Fprintf(&b, "%-8s", g.ID)
	typ := g.Type
	if typ == "" {
		typ = "-"
	}
	fmt.Fprintf(&b, "  %-9s", typ)
	if g.Rel != "" {
		fmt.Fprintf(&b, " ←%s→", g.Rel)
	}
	b.WriteString("  " + DisplayLine(g, pinLineWidth))
	if n := len(g.Refs); n > 0 {
		fmt.Fprintf(&b, "  ↗%d", n)
	}
	if g.Score > 0 {
		fmt.Fprintf(&b, "  [%.3f]", g.Score)
	}
	return b.String()
}

// Card renders pin plus tags and a short preview.
func Card(g *types.Glyph) string {
	var b strings.Builder
	b.WriteString(Pin(g))
	if len(g.Tags) > 0 {
		b.WriteString("\n    tags: " + strings.Join(g.Tags, ", "))
	}
	if len(g.Refs) > 0 {
		hints := make([]string, 0, len(g.Refs))
		for _, r := range g.Refs {
			hints = append(hints, r.Kind+":"+Truncate(r.Target, 40))
		}
		b.WriteString("\n    refs: " + strings.Join(hints, "  "))
	}
	if preview := Truncate(strings.TrimSpace(g.Body), 200); len(preview) > pinLineWidth {
		b.WriteString("\n    " + preview)
	}
	return b.String()
}

// Body renders the full glyph: body text, tags, refs, timestamps.
func Body(g *types.Glyph) string {
	var b strings.Builder
	if g.Starred {
		b.WriteString("★ ")
	}
	fmt.Fprintf(&b, "%s", g.ID)
	if g.Type != "" {
		fmt.Fprintf(&b, "  %s", g.Type)
	}
	fmt.Fprintf(&b, "  etched %s", g.CreatedAt.Format("2006-01-02 15:04"))
	if !g.UpdatedAt.Equal(g.CreatedAt) {
		fmt.Fprintf(&b, "  amended %s", g.UpdatedAt.Format("2006-01-02 15:04"))
	}
	b.WriteString("\n\n")
	if g.Summary != "" {
		b.WriteString("summary: " + strings.TrimSpace(g.Summary) + "\n\n")
	}
	b.WriteString(strings.TrimSpace(g.Body) + "\n")
	if len(g.Tags) > 0 {
		b.WriteString("\ntags: " + strings.Join(g.Tags, ", ") + "\n")
	}
	if len(g.Refs) > 0 {
		b.WriteString("refs:\n")
		for _, r := range g.Refs {
			b.WriteString("  " + r.String() + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// Neighborhood renders body plus neighbor pins.
func Neighborhood(g *types.Glyph, neighbors []*types.Glyph) string {
	var b strings.Builder
	b.WriteString(Body(g))
	if len(neighbors) > 0 {
		b.WriteString("\n\nneighbors:\n")
		for _, n := range neighbors {
			b.WriteString("  " + Pin(n) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// Traversal renders a rooted, direction-aware relationship tree. The stored
// graph may contain cycles and diamonds; links marked Repeat are shown once
// with a return marker and never recursively expanded.
func Traversal(t *types.Traversal, f types.Facet) string {
	if t == nil || t.Root == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(Render(f, t.Root, nil))
	if len(t.Links) > 0 {
		b.WriteString("\n")
		writeTraversalChildren(&b, t, f, t.Root.ID, "")
	}
	if t.Truncated {
		b.WriteString("\n… traversal truncated; raise --limit or reduce --depth")
	}
	return strings.TrimRight(b.String(), "\n")
}

// NeighborhoodTraversal renders a full root glyph followed by the same
// direction-aware depth-one branches used by related.
func NeighborhoodTraversal(t *types.Traversal) string {
	if t == nil || t.Root == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(Body(t.Root))
	if len(t.Links) > 0 {
		b.WriteString("\n\nneighbors:\n")
		writeTraversalChildren(&b, t, types.FacetPin, t.Root.ID, "")
	}
	if t.Truncated {
		b.WriteString("\n… neighborhood truncated")
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeTraversalChildren(b *strings.Builder, t *types.Traversal, f types.Facet, from, prefix string) {
	links := make([]*types.TraversalLink, 0)
	for _, link := range t.Links {
		if link.From == from {
			links = append(links, link)
		}
	}
	byID := make(map[string]*types.Glyph, len(t.Glyphs))
	for _, g := range t.Glyphs {
		byID[g.ID] = g
	}
	for i, link := range links {
		last := i == len(links)-1
		connector, continuation := "├─", "│  "
		if last {
			connector, continuation = "└─", "   "
		}
		arrow := "→"
		if link.Direction == types.DirectionIn {
			arrow = "←"
		}
		g := byID[link.To]
		if g == nil {
			continue
		}
		line := Render(f, g, nil)
		if link.Repeat {
			line += "  ↩"
		}
		lead := prefix + connector + " " + link.Edge.Rel + " " + arrow + " "
		b.WriteString(indentContinuation(line, lead, prefix+continuation+"  ") + "\n")
		if !link.Repeat {
			writeTraversalChildren(b, t, f, link.To, prefix+continuation)
		}
	}
}

func indentContinuation(s, firstPrefix, restPrefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		if i == 0 {
			lines[i] = firstPrefix + lines[i]
		} else {
			lines[i] = restPrefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// Render renders one glyph at the given facet (neighborhood needs neighbors
// supplied by the caller).
func Render(f types.Facet, g *types.Glyph, neighbors []*types.Glyph) string {
	switch f {
	case types.FacetID:
		return g.ID
	case types.FacetPin:
		return Pin(g)
	case types.FacetCard:
		return Card(g)
	case types.FacetNeighborhood:
		return Neighborhood(g, neighbors)
	default:
		return Body(g)
	}
}

// List renders a slice of glyphs at a facet, one entry per glyph.
func List(f types.Facet, gs []*types.Glyph) string {
	lines := make([]string, 0, len(gs))
	for _, g := range gs {
		lines = append(lines, Render(f, g, nil))
	}
	sep := "\n"
	if f == types.FacetCard {
		sep = "\n\n"
	}
	return strings.Join(lines, sep)
}

// TruncationNote reports honestly when output was cut: agents should learn
// the window is full, not that memory is empty.
func TruncationNote(shown, total int, raise string) string {
	if shown >= total {
		return ""
	}
	return fmt.Sprintf("showing %d of %d matches; raise %s or narrow the query", shown, total, raise)
}

// FirstLine returns the first line of s, truncated to width.
func FirstLine(s string, width int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return Truncate(s, width)
}

// Truncate cuts s to at most width runes, appending … when cut.
func Truncate(s string, width int) string {
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	return string(r[:width-1]) + "…"
}

// WordCount counts whitespace-separated words.
func WordCount(s string) int {
	return len(strings.Fields(s))
}

// BudgetLines appends lines until the word budget is spent, returning the
// kept lines and how many were dropped.
func BudgetLines(lines []string, budget int) (kept []string, dropped int) {
	if budget <= 0 {
		return lines, 0
	}
	used := 0
	for i, ln := range lines {
		w := WordCount(ln)
		if used+w > budget && i > 0 {
			return lines[:i], len(lines) - i
		}
		used += w
	}
	return lines, 0
}
