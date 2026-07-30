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
		var neighbors []*types.Glyph
		if f == types.FacetNeighborhood {
			neighbors, err = st.Related(g.ID, 10)
			if err != nil {
				return err
			}
		}
		if jsonOut() {
			out := map[string]any{"facet": f, "glyph": jsonGlyph(f, g)}
			if f == types.FacetNeighborhood {
				out["neighbors"] = jsonGlyphs(types.FacetPin, neighbors)
			}
			return emitJSON(out)
		}
		fmt.Println(facet.Render(f, g, neighbors))
		return nil
	},
}

func init() {
	showCmd.Flags().StringVar(&showFacet, "facet", "", "disclosure level (default body)")
}
