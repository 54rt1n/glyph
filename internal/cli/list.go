package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/store"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	listType  string
	listTag   string
	listLimit int
	listFacet string
	listTime  timeFilters
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent or filtered glyphs (thin pins by default)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := parseFacet(listFacet, types.FacetPin)
		if err != nil {
			return err
		}
		since, until, err := listTime.resolve()
		if err != nil {
			return err
		}
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		gs, total, err := st.ListGlyphs(store.ListFilter{
			Type: listType, Tag: listTag, Since: since, Until: until, Limit: listLimit,
		})
		if err != nil {
			return err
		}
		if jsonOut() {
			return emitJSON(map[string]any{"facet": f, "total": total, "glyphs": jsonGlyphs(f, gs)})
		}
		if len(gs) == 0 {
			fmt.Println("no glyphs match — `glyph etch \"…\"` to write the first one")
			return nil
		}
		fmt.Println(facet.List(f, gs))
		if note := facet.TruncationNote(len(gs), total, "--limit"); note != "" {
			fmt.Println(note)
		}
		return nil
	},
}

func init() {
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "filter by glyph kind")
	listCmd.Flags().StringVar(&listTag, "tag", "", "filter by tag")
	listCmd.Flags().IntVar(&listLimit, "limit", 20, "max results")
	listCmd.Flags().StringVar(&listFacet, "facet", "", "disclosure level (default pin)")
	listTime.register(listCmd)
}
