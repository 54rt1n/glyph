// Package facet renders glyphs at progressive disclosure levels. All
// commands share these renderers so density logic never forks.
package facet

import (
	"fmt"
	"strings"

	"github.com/54rt1n/glyph/internal/types"
)

const pinLineWidth = 72

// Pin renders the one-line view: id, type, first line, ref count, score.
func Pin(g *types.Glyph) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s", g.ID)
	typ := g.Type
	if typ == "" {
		typ = "-"
	}
	fmt.Fprintf(&b, "  %-9s", typ)
	if g.Rel != "" {
		fmt.Fprintf(&b, " ←%s→", g.Rel)
	}
	b.WriteString("  " + FirstLine(g.Body, pinLineWidth))
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
	fmt.Fprintf(&b, "%s", g.ID)
	if g.Type != "" {
		fmt.Fprintf(&b, "  %s", g.Type)
	}
	fmt.Fprintf(&b, "  etched %s", g.CreatedAt.Format("2006-01-02 15:04"))
	if !g.UpdatedAt.Equal(g.CreatedAt) {
		fmt.Fprintf(&b, "  amended %s", g.UpdatedAt.Format("2006-01-02 15:04"))
	}
	b.WriteString("\n\n" + strings.TrimSpace(g.Body) + "\n")
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
