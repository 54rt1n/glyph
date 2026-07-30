package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var forgetCmd = &cobra.Command{
	Use:   "forget <id>",
	Short: "Delete a glyph (edges, tags, refs, vectors cascade)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		if err := st.DeleteGlyph(args[0]); err != nil {
			return err
		}
		if jsonOut() {
			return emitJSON(map[string]string{"forgotten": args[0]})
		}
		fmt.Printf("forgot %s\n", args[0])
		return nil
	},
}
