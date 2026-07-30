package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/store"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	askLimit int
	askFacet string
	askType  string
	askTag   string
	askTime  timeFilters
)

var askCmd = &cobra.Command{
	Use:   "ask <query>",
	Short: "Query memory (hybrid BM25 + vectors; thin pins by default)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := parseFacet(askFacet, types.FacetPin)
		if err != nil {
			return err
		}
		since, until, err := askTime.resolve()
		if err != nil {
			return err
		}
		proj, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		hits, err := st.Search(args[0], askLimit, loadEmbedder(proj),
			store.ListFilter{Type: askType, Tag: askTag, Since: since, Until: until})
		if err != nil {
			return err
		}
		gs := make([]*types.Glyph, 0, len(hits))
		for _, h := range hits {
			g, err := st.GetGlyph(h.ID)
			if err != nil {
				continue
			}
			g.Score = h.Score
			gs = append(gs, g)
		}
		if jsonOut() {
			return emitJSON(map[string]any{"facet": f, "hits": jsonGlyphs(f, gs)})
		}
		if len(gs) == 0 {
			fmt.Println("no matches — try broader terms, or `glyph list` for recent glyphs")
			return nil
		}
		fmt.Println(facet.List(f, gs))
		return nil
	},
}

func init() {
	askCmd.Flags().IntVar(&askLimit, "limit", 10, "max results")
	askCmd.Flags().StringVar(&askFacet, "facet", "", "disclosure level (default pin)")
	askCmd.Flags().StringVarP(&askType, "type", "t", "", "filter by glyph kind")
	askCmd.Flags().StringVar(&askTag, "tag", "", "filter by tag")
	askTime.register(askCmd)
}
