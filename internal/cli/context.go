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
	contextBudget int
	contextTag    string
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Store summary: counts, types, tags, activity (bd prime-style)",
	Long: `Orient on the store: counts, types, top tags, recent activity.

--tag scopes every count and surfaces standing pins for that tag
(decisions first). Use it when you already know the topic and want
the standing decision without a noisy ask.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		limit := 5
		if contextTag != "" {
			limit = 12
		}
		stats, err := st.GetStats(limit, store.ListFilter{Tag: contextTag})
		if err != nil {
			return err
		}
		if jsonOut() {
			recent := stats.Recent
			stats.Recent = nil
			out := map[string]any{"stats": stats}
			if stats.Tag != "" {
				out["standing"] = jsonGlyphs(types.FacetPin, recent)
			} else {
				out["recent"] = jsonGlyphs(types.FacetPin, recent)
			}
			return emitJSON(out)
		}
		fmt.Println(renderContext(stats, contextBudget))
		return nil
	},
}

func init() {
	contextCmd.Flags().IntVar(&contextBudget, "budget", 300, "word budget for the summary (0 = unlimited)")
	contextCmd.Flags().StringVar(&contextTag, "tag", "", "standing view for one tag")
}

func renderContext(s *store.Stats, budget int) string {
	var b strings.Builder
	if s.Tag != "" {
		fmt.Fprintf(&b, "glyph store [tag %s]: %d glyphs · %d edges · %d refs", s.Tag, s.Glyphs, s.Edges, s.Refs)
	} else {
		fmt.Fprintf(&b, "glyph store: %d glyphs · %d edges · %d refs", s.Glyphs, s.Edges, s.Refs)
	}
	if s.Vecs > 0 {
		fmt.Fprintf(&b, " · %d vectors", s.Vecs)
	}
	if !s.LastEtch.IsZero() {
		fmt.Fprintf(&b, "        last etch: %s", humanAgo(s.LastEtch))
	}
	b.WriteString("\n")
	if len(s.ByType) > 0 {
		parts := make([]string, 0, len(s.ByType))
		for _, tc := range s.ByType {
			parts = append(parts, fmt.Sprintf("%s %d", tc.Type, tc.Count))
		}
		b.WriteString("by type:  " + strings.Join(parts, " · ") + "\n")
	}
	if len(s.TopTags) > 0 {
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
	fmt.Fprintf(&b, "activity: today %d · this week %d\n", s.Today, s.ThisWeek)
	b.WriteString("facets:   pin → card → body → neighborhood   (defaults: bulk=pin, show=body)\n")
	if len(s.Recent) > 0 {
		if s.Tag != "" {
			b.WriteString("standing:\n")
		} else {
			b.WriteString("recent:\n")
		}
		lines := make([]string, 0, len(s.Recent))
		for _, g := range s.Recent {
			lines = append(lines, "  "+facet.Pin(g))
		}
		kept, dropped := facet.BudgetLines(lines, remainingBudget(budget, b.String()))
		b.WriteString(strings.Join(kept, "\n") + "\n")
		if dropped > 0 {
			fmt.Fprintf(&b, "  context pack full; %d more recent glyphs in store — `glyph list` / `glyph ask`\n", dropped)
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
