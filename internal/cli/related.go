package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	relatedLimit     int
	relatedFacet     string
	relatedDepth     int
	relatedDirection string
)

var relatedCmd = &cobra.Command{
	Use:   "related <id>",
	Short: "Traverse a rooted relationship tree (depth 1, both directions by default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := parseFacet(relatedFacet, types.FacetPin)
		if err != nil {
			return err
		}
		if relatedDepth < 0 {
			return fmt.Errorf("depth must be at least 0")
		}
		if !types.ValidDirection(relatedDirection) {
			return fmt.Errorf("unknown direction %q (both, out, in)", relatedDirection)
		}
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		traversal, err := st.TraverseRelated(args[0], relatedDepth, types.Direction(relatedDirection), relatedLimit)
		if err != nil {
			return err
		}
		if jsonOut() {
			return emitJSON(jsonTraversal(f, traversal))
		}
		fmt.Println(facet.Traversal(traversal, f))
		if len(traversal.Links) == 0 && relatedDepth > 0 && !traversal.Truncated {
			fmt.Println("no links yet — `glyph link " + args[0] + " <other-id> --as related`")
		}
		return nil
	},
}

func init() {
	relatedCmd.Flags().IntVar(&relatedLimit, "limit", 20, "max nodes including the root")
	relatedCmd.Flags().StringVar(&relatedFacet, "facet", "", "disclosure level (default pin)")
	relatedCmd.Flags().IntVar(&relatedDepth, "depth", 1, "edge depth to traverse")
	relatedCmd.Flags().StringVar(&relatedDirection, "direction", "both", "edge direction: both, out, or in")
}

func jsonTraversal(f types.Facet, traversal *types.Traversal) map[string]any {
	edges := make([]map[string]any, 0, len(traversal.Links))
	rels := make(map[string]string, len(traversal.Links))
	for _, link := range traversal.Links {
		edges = append(edges, map[string]any{
			"id": link.Edge.ID, "src": link.Edge.Src, "dst": link.Edge.Dst, "rel": link.Edge.Rel,
			"created_at": link.Edge.CreatedAt,
			"from":       link.From, "to": link.To, "direction": link.Direction,
			"depth": link.Depth, "repeat": link.Repeat,
		})
		if !link.Repeat {
			rels[link.To] = link.Edge.Rel
		}
	}
	compatGlyphs := make([]*types.Glyph, 0, len(traversal.Glyphs)-1)
	for _, g := range traversal.Glyphs[1:] {
		clone := *g
		clone.Rel = rels[g.ID]
		compatGlyphs = append(compatGlyphs, &clone)
	}
	return map[string]any{
		"facet": f, "root": jsonGlyph(f, traversal.Root),
		"nodes": jsonGlyphs(f, traversal.Glyphs), "glyphs": jsonGlyphs(f, compatGlyphs), "edges": edges,
		"depth": traversal.Depth, "direction": traversal.Direction, "truncated": traversal.Truncated,
	}
}
