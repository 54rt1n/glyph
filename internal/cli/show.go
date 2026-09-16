package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/types"
)

var showFacet string

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show one glyph (full body + refs; --facet neighborhood for 1-hop)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := parseFacet(showFacet, types.FacetBody)
		if err != nil {
			return err
		}
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		g, err := st.GetGlyph(args[0])
		if err != nil {
			return err
		}
		var traversal *types.Traversal
		if f == types.FacetNeighborhood {
			traversal, err = st.TraverseRelated(g.ID, 1, types.DirectionBoth, 11)
			if err != nil {
				return err
			}
		}
		if jsonOut() {
			out := map[string]any{"facet": f, "glyph": jsonGlyph(f, g)}
			if f == types.FacetNeighborhood {
				related := jsonTraversal(types.FacetPin, traversal)
				out["neighbors"] = related["glyphs"]
				out["edges"] = related["edges"]
			}
			return emitJSON(out)
		}
		if f == types.FacetNeighborhood {
			fmt.Println(facet.NeighborhoodTraversal(traversal))
		} else {
			fmt.Println(facet.Render(f, g, nil))
		}
		return nil
	},
}

func init() {
	showCmd.Flags().StringVar(&showFacet, "facet", "", "disclosure level (default body)")
}
