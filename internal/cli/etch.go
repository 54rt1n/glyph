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
	etchType    string
	etchTags    []string
	etchRefs    []string
	etchGraph   string
	etchSummary string
	etchStar    bool
)

var etchCmd = &cobra.Command{
	Use:   "etch [body]",
	Short: "Store text as a glyph (timestamped automatically)",
	Long: `Body comes from the argument, or stdin when piped. Summary is the optional
one-line scanning label; star adds the glyph to active focus. Tags classify; refs cite.

--graph FILE applies a small JSON graph (etch + link) in one transaction.
Ids in the file are aliases remapped to real g-xxxx ids; links may also
name existing glyphs. Use --graph - to read the graph from stdin.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if etchGraph != "" {
			if len(args) > 0 {
				return fmt.Errorf("etch --graph does not take a body argument")
			}
			return runEtchGraph(etchGraph)
		}
		body, err := bodyFromArgsOrStdin(args)
		if err != nil {
			return err
		}
		summary, err := cleanSummary(etchSummary)
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
			Summary:   summary,
			Body:      body,
			Type:      etchType,
			Starred:   etchStar,
			Tags:      etchTags,
			Refs:      refs,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := st.CreateGlyph(g); err != nil {
			return err
		}
		embedGlyph(st, loadEmbedder(proj), g)
		if jsonOut() {
			return emitJSON(writeAck(g))
		}
		fmt.Println(g.ID)
		return nil
	},
}

func init() {
	etchCmd.Flags().StringVarP(&etchType, "type", "t", "", "glyph kind: note, decision, fact, …")
	etchCmd.Flags().StringVarP(&etchSummary, "summary", "s", "", "one-line scanning summary")
	etchCmd.Flags().BoolVar(&etchStar, "star", false, "add glyph to the active focus set")
	etchCmd.Flags().StringArrayVar(&etchTags, "tag", nil, "freeform label (repeatable)")
	etchCmd.Flags().StringArrayVar(&etchRefs, "ref", nil, "kind:target reference (repeatable)")
	etchCmd.Flags().StringVar(&etchGraph, "graph", "", "JSON file of etch+link (\"-\" = stdin)")
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
