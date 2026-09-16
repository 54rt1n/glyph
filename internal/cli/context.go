package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/store"
	"github.com/54rt1n/glyph/internal/types"
)

var (
	contextBudget  int
	contextTag     string
	contextVerbose bool
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Orient with store size, active focus, and recent memory",
	Long: `Orient on the store: compact counts, all starred focus, and five additional recent glyphs.

--tag scopes every count and surfaces standing pins for that tag
(decisions first). Use it when you already know the topic and want
the standing decision without a noisy ask. --verbose adds inventory.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		stats, err := st.GetStats(5, store.ListFilter{Tag: contextTag})
		if err != nil {
			return err
		}
		if jsonOut() {
			recent := stats.Recent
			focus := stats.Focus
			stats.Recent = nil
			stats.Focus = nil
			out := map[string]any{"stats": stats}
			if len(focus) > 0 {
				out["focus"] = jsonGlyphs(types.FacetPin, focus)
			}
			focusIDs := make(map[string]bool, len(focus))
			for _, g := range focus {
				focusIDs[g.ID] = true
			}
			filteredRecent := recent[:0]
			for _, g := range recent {
				if !focusIDs[g.ID] {
					filteredRecent = append(filteredRecent, g)
				}
			}
			if stats.Tag != "" {
				out["standing"] = jsonGlyphs(types.FacetPin, filteredRecent)
			} else {
				out["recent"] = jsonGlyphs(types.FacetPin, filteredRecent)
			}
			return emitJSON(out)
		}
		fmt.Println(renderContext(stats, contextBudget, contextVerbose))
		return nil
	},
}

func init() {
	contextCmd.Flags().IntVar(&contextBudget, "budget", 0, "word budget for the summary (0 = unlimited)")
	contextCmd.Flags().StringVar(&contextTag, "tag", "", "standing view for one tag")
	contextCmd.Flags().BoolVarP(&contextVerbose, "verbose", "v", false, "include type, tag, reference, vector, and activity inventory")
}

func renderContext(s *store.Stats, budget int, verbose bool) string {
	var b strings.Builder
	if s.Tag != "" {
		fmt.Fprintf(&b, "glyph store [tag %s]: %d glyphs · %d edges", s.Tag, s.Glyphs, s.Edges)
	} else {
		fmt.Fprintf(&b, "glyph store: %d glyphs · %d edges", s.Glyphs, s.Edges)
	}
	if !s.LastEtch.IsZero() {
		fmt.Fprintf(&b, " · last etch %s", humanAgo(s.LastEtch))
	}
	b.WriteString("\n")
	if verbose {
		fmt.Fprintf(&b, "inventory: %d refs · %d vectors · today %d · this week %d\n", s.Refs, s.Vecs, s.Today, s.ThisWeek)
	}
	if verbose && len(s.ByType) > 0 {
		parts := make([]string, 0, len(s.ByType))
		for _, tc := range s.ByType {
			parts = append(parts, fmt.Sprintf("%s %d", tc.Type, tc.Count))
		}
		b.WriteString("by type:  " + strings.Join(parts, " · ") + "\n")
	}
	if (verbose || s.Tag != "") && len(s.TopTags) > 0 {
		parts := make([]string, 0, len(s.TopTags))
		for _, tc := range s.TopTags {
			parts = append(parts, fmt.Sprintf("%s(%d)", tc.Tag, tc.Count))
		}
		label := "top tags: "
		if s.Tag != "" {
			label = "co-tags:  "
		}
		b.WriteString(label + strings.Join(parts, " · ") + "\n")
	}
	seen := map[string]bool{}
	if len(s.Focus) > 0 {
		b.WriteString("focus:\n")
		lines := make([]string, 0, len(s.Focus))
		for _, g := range s.Focus {
			seen[g.ID] = true
			lines = append(lines, "  "+facet.Pin(g))
		}
		kept, dropped := facet.BudgetLines(lines, remainingBudget(budget, b.String()))
		b.WriteString(strings.Join(kept, "\n") + "\n")
		omitted := s.FocusTotal - len(kept)
		if omitted < dropped {
			omitted = dropped
		}
		if omitted > 0 {
			fmt.Fprintf(&b, "  context pack full; %d more starred glyphs — `glyph list --star`\n", omitted)
		}
	}
	if len(s.Recent) > 0 {
		lines := make([]string, 0, len(s.Recent))
		for _, g := range s.Recent {
			if seen[g.ID] {
				continue
			}
			lines = append(lines, "  "+facet.Pin(g))
		}
		if len(lines) > 0 {
			if s.Tag != "" {
				b.WriteString("standing:\n")
			} else {
				b.WriteString("recent:\n")
			}
			kept, dropped := facet.BudgetLines(lines, remainingBudget(budget, b.String()))
			b.WriteString(strings.Join(kept, "\n") + "\n")
			if dropped > 0 {
				fmt.Fprintf(&b, "  context pack full; %d more recent glyphs in store — `glyph list` / `glyph ask`\n", dropped)
			}
		}
	}
	b.WriteString("run `glyph skill` for full usage.")
	return b.String()
}

// remainingBudget subtracts words already emitted from the total budget.
func remainingBudget(budget int, sofar string) int {
	if budget <= 0 {
		return 0
	}
	rest := budget - facet.WordCount(sofar)
	if rest < 1 {
		rest = 1
	}
	return rest
}

func humanAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
