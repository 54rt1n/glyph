package cli

import (
	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/types"
)

// jsonGlyphs shapes glyphs for JSON output at the given facet — the same
// disclosure ladder as the human renderer, so agents get thin views too.
func jsonGlyphs(f types.Facet, gs []*types.Glyph) []any {
	out := make([]any, 0, len(gs))
	for _, g := range gs {
		out = append(out, jsonGlyph(f, g))
	}
	return out
}

func jsonGlyph(f types.Facet, g *types.Glyph) any {
	switch f {
	case types.FacetID:
		return g.ID
	case types.FacetPin:
		m := map[string]any{"id": g.ID, "line": facet.FirstLine(g.Body, 72)}
		addCommon(m, g, false)
		return m
	case types.FacetCard:
		m := map[string]any{"id": g.ID, "preview": facet.Truncate(g.Body, 200)}
		addCommon(m, g, true)
		return m
	default: // body, neighborhood, raw
		return g
	}
}

func addCommon(m map[string]any, g *types.Glyph, full bool) {
	if g.Type != "" {
		m["type"] = g.Type
	}
	if g.Score > 0 {
		m["score"] = g.Score
	}
	if g.Rel != "" {
		m["rel"] = g.Rel
	}
	if n := len(g.Refs); n > 0 {
		if full {
			m["refs"] = g.Refs
		} else {
			m["refs"] = n
		}
	}
	if full && len(g.Tags) > 0 {
		m["tags"] = g.Tags
	}
}
