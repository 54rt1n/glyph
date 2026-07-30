package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var linkRel string

var linkCmd = &cobra.Command{
	Use:   "link <src-id> <dst-id>",
	Short: "Connect two glyphs with a typed edge",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		e, err := st.CreateEdge(args[0], args[1], linkRel)
		if err != nil {
			return err
		}
		if jsonOut() {
			return emitJSON(e)
		}
		fmt.Printf("%s -%s-> %s\n", e.Src, e.Rel, e.Dst)
		return nil
	},
}

func init() {
	linkCmd.Flags().StringVar(&linkRel, "as", "related", "relation name (supports, related, …)")
}
