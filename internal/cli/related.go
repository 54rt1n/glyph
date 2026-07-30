package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	relatedLimit int
	relatedFacet string
)

var relatedCmd = &cobra.Command{
	Use:   "related <id>",
	Short: "1-hop neighbors of a glyph (thin pins by default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := parseFacet(relatedFacet, types.FacetPin)
		if err != nil {
			return err
		}
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		gs, err := st.Related(args[0], relatedLimit)
		if err != nil {
			return err
		}
		if jsonOut() {
			return emitJSON(map[string]any{"facet": f, "glyphs": jsonGlyphs(f, gs)})
		}
		if len(gs) == 0 {
			fmt.Println("no links yet — `glyph link " + args[0] + " <other-id> --as related`")
			return nil
		}
		fmt.Println(facet.List(f, gs))
		return nil
	},
}

func init() {
	relatedCmd.Flags().IntVar(&relatedLimit, "limit", 20, "max results")
	relatedCmd.Flags().StringVar(&relatedFacet, "facet", "", "disclosure level (default pin)")
}
