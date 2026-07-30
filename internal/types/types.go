// Package types holds the shared domain structs for glyph.
package types

import "time"

// Facet is a disclosure level — a view over a glyph, not a stored object.
type Facet string

const (
	FacetID           Facet = "id"
	FacetPin          Facet = "pin"
	FacetCard         Facet = "card"
	FacetBody         Facet = "body"
	FacetNeighborhood Facet = "neighborhood"
	FacetRaw          Facet = "raw"
)

// ValidFacet reports whether s names a known facet.
func ValidFacet(s string) bool {
	switch Facet(s) {
	case FacetID, FacetPin, FacetCard, FacetBody, FacetNeighborhood, FacetRaw:
		return true
	}
	return false
}

// Ref is a loose reference from a glyph to anything addressable outside the
// graph (url, path, bead id, uuid, …). Kinds are conventions, not an enum.
type Ref struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
}

func (r Ref) String() string {
	if r.Label != "" {
		return r.Kind + ":" + r.Target + " (" + r.Label + ")"
	}
	return r.Kind + ":" + r.Target
}

// Glyph is a durable knowledge node.
type Glyph struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	Type      string    `json:"type,omitempty"`
	Meta      string    `json:"meta,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Tags      []string  `json:"tags,omitempty"`
	Refs      []Ref     `json:"refs,omitempty"`

	// Score is set on retrieval results (ask); zero otherwise.
	Score float64 `json:"score,omitempty"`
	// Rel is set on related results: the edge relation that reached this glyph.
	Rel string `json:"rel,omitempty"`
}

// Edge is a typed glyph ↔ glyph link.
type Edge struct {
	ID        string    `json:"id"`
	Src       string    `json:"src"`
	Dst       string    `json:"dst"`
	Rel       string    `json:"rel"`
	CreatedAt time.Time `json:"created_at"`
}
