package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/id"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	etchType string
	etchTags []string
	etchRefs []string
)

var etchCmd = &cobra.Command{
	Use:   "etch [body]",
	Short: "Store text as a glyph (timestamped automatically)",
	Long:  "Body comes from the argument, or stdin when piped. Tags classify; refs cite.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := bodyFromArgsOrStdin(args)
		if err != nil {
			return err
		}
		refs := make([]types.Ref, 0, len(etchRefs))
		for _, s := range etchRefs {
			r, err := parseRef(s)
			if err != nil {
				return err
			}
			refs = append(refs, r)
		}
		proj, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		now := time.Now()
		g := &types.Glyph{
			ID:        id.Generate(st.Exists),
			Body:      body,
			Type:      etchType,
			Tags:      etchTags,
			Refs:      refs,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := st.CreateGlyph(g); err != nil {
			return err
		}
		embedGlyph(st, loadEmbedder(proj), g.ID, g.Body)
		if jsonOut() {
			return emitJSON(map[string]string{"id": g.ID})
		}
		fmt.Println(g.ID)
		return nil
	},
}

func init() {
	etchCmd.Flags().StringVarP(&etchType, "type", "t", "", "glyph kind: note, decision, fact, …")
	etchCmd.Flags().StringArrayVar(&etchTags, "tag", nil, "freeform label (repeatable)")
	etchCmd.Flags().StringArrayVar(&etchRefs, "ref", nil, "kind:target reference (repeatable)")
}

func bodyFromArgsOrStdin(args []string) (string, error) {
	if len(args) == 1 && args[0] != "-" {
		if strings.TrimSpace(args[0]) == "" {
			return "", fmt.Errorf("empty body")
		}
		return args[0], nil
	}
	st, err := os.Stdin.Stat()
	if err == nil && (st.Mode()&os.ModeCharDevice) == 0 || len(args) == 1 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		body := strings.TrimSpace(string(b))
		if body == "" {
			return "", fmt.Errorf("empty body on stdin")
		}
		return body, nil
	}
	return "", fmt.Errorf("no body: pass text as an argument or pipe it on stdin")
}
