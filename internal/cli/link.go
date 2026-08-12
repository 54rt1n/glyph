package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var linkRel string

var linkCmd = &cobra.Command{
	Use:   "link <src-id> <dst-id> [dst-id...]",
	Short: "Connect glyphs with a typed edge (one src, one or more dests)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		es, err := st.CreateEdges(args[0], args[1:], linkRel)
		if err != nil {
			return err
		}
		if jsonOut() {
			if len(es) == 1 {
				return emitJSON(es[0])
			}
			return emitJSON(map[string]any{"edges": es})
		}
		for _, e := range es {
			fmt.Printf("%s -%s-> %s\n", e.Src, e.Rel, e.Dst)
		}
		return nil
	},
}

func init() {
	linkCmd.Flags().StringVar(&linkRel, "as", "related", "relation name (supports, related, …)")
}
