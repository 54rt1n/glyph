package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/store"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	amendType    string
	amendTags    []string
	amendRefs    []string
	amendAppend  bool
	amendSummary string
	amendStar    bool
	amendUnstar  bool
)

var amendCmd = &cobra.Command{
	Use:   "amend <id> [new body]",
	Short: "Revise a glyph in place — no near-duplicate etches",
	Long: `Update body, summary, focus, type, tags, or refs of an existing glyph.
--tag and --ref take +value to add and -value to remove:
  glyph amend g-a1b2 --tag +packing --tag -v0 --ref +url:https://… --ref -bead:bd-x7k2
--append keeps the existing body and adds the new text after a blank line:
  glyph amend g-a1b2 --append "COMPLETE: shipped the split"`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		gid := args[0]
		var body *string
		if len(args) == 2 {
			if strings.TrimSpace(args[1]) == "" {
				return fmt.Errorf("empty body")
			}
			body = &args[1]
		}
		if amendAppend && body == nil {
			return fmt.Errorf("--append requires a body")
		}
		var summary *string
		if cmd.Flags().Changed("summary") {
			cleaned, err := cleanSummary(amendSummary)
			if err != nil {
				return err
			}
			summary = &cleaned
		}
		var starred *bool
		if amendStar {
			v := true
			starred = &v
		} else if amendUnstar {
			v := false
			starred = &v
		}
		var typ *string
		if cmd.Flags().Changed("type") {
			typ = &amendType
		}
		addTags, rmTags, err := splitSigned(amendTags, "tag")
		if err != nil {
			return err
		}
		addRefStrs, rmRefStrs, err := splitSigned(amendRefs, "ref")
		if err != nil {
			return err
		}
		addRefs, err := parseRefs(addRefStrs)
		if err != nil {
			return err
		}
		rmRefs, err := parseRefs(rmRefStrs)
		if err != nil {
			return err
		}
		if body == nil && summary == nil && typ == nil && starred == nil && len(addTags)+len(rmTags)+len(addRefs)+len(rmRefs) == 0 {
			return fmt.Errorf("nothing to amend: pass a new body, --summary, --star/--unstar, --type, --tag +/-, or --ref +/-")
		}

		proj, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		var existing *types.Glyph
		if body != nil || summary != nil {
			existing, err = st.GetGlyph(gid)
			if err != nil {
				return err
			}
		}
		if amendAppend && body != nil {
			combined := appendBody(existing.Body, *body)
			body = &combined
		}
		bodyChanged := body != nil && existing != nil && existing.Body != *body
		summaryChanged := summary != nil && existing != nil && existing.Summary != *summary
		g, searchableChanged, err := st.AmendGlyph(gid, store.GlyphPatch{
			Body: body, Summary: summary, Type: typ, Starred: starred,
			AddTags: addTags, RmTags: rmTags, AddRefs: addRefs, RmRefs: rmRefs,
		})
		if err != nil {
			return err
		}
		if searchableChanged {
			embedGlyph(st, loadEmbedder(proj), g)
		}
		if jsonOut() {
			ack := writeAck(g)
			ack["body_changed"] = bodyChanged
			ack["summary_changed"] = summaryChanged
			if amendAppend {
				ack["appended"] = true
			}
			return emitJSON(ack)
		}
		fmt.Printf("amended %s\n", g.ID)
		return nil
	},
}

func init() {
	amendCmd.Flags().StringVarP(&amendType, "type", "t", "", "set glyph kind")
	amendCmd.Flags().StringVarP(&amendSummary, "summary", "s", "", "set one-line summary (empty clears)")
	amendCmd.Flags().BoolVar(&amendStar, "star", false, "add glyph to the active focus set")
	amendCmd.Flags().BoolVar(&amendUnstar, "unstar", false, "remove glyph from the active focus set")
	amendCmd.MarkFlagsMutuallyExclusive("star", "unstar")
	amendCmd.Flags().StringArrayVar(&amendTags, "tag", nil, "+tag to add, -tag to remove (repeatable)")
	amendCmd.Flags().StringArrayVar(&amendRefs, "ref", nil, "+kind:target to add, -kind:target to remove (repeatable)")
	amendCmd.Flags().BoolVar(&amendAppend, "append", false, "append body instead of replacing")
}

// appendBody keeps the existing note and adds extra after a blank line.
func appendBody(existing, extra string) string {
	return strings.TrimRight(existing, "\n") + "\n\n" + strings.TrimSpace(extra)
}

// splitSigned partitions +value / -value entries.
func splitSigned(vals []string, what string) (add, rm []string, err error) {
	for _, v := range vals {
		switch {
		case strings.HasPrefix(v, "+") && len(v) > 1:
			add = append(add, v[1:])
		case strings.HasPrefix(v, "-") && len(v) > 1:
			rm = append(rm, v[1:])
		default:
			return nil, nil, fmt.Errorf("amend --%s needs a +/- prefix: got %q (use +%s to add, -%s to remove)", what, v, v, v)
		}
	}
	return add, rm, nil
}

func parseRefs(strs []string) ([]types.Ref, error) {
	refs := make([]types.Ref, 0, len(strs))
	for _, s := range strs {
		r, err := parseRef(s)
		if err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, nil
}
