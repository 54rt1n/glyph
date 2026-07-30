package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// skillText is the agent-facing manual — SKILL.md-shaped so it can be
// dropped into an agent's skill/instructions system verbatim.
const skillText = `---
name: glyph
description: Local graph memory — etch knowledge, link it, retrieve it thinly with progressive disclosure.
---

# Glyph — agent memory skill

Glyph stores project knowledge as a graph of **glyphs** (short-id nodes like g-a1b2).
Memory is **progressive**: bulk commands return one-line **pins**; deepen on purpose.

## Session loop

    glyph context                 # orient: counts, types, tags, activity
    glyph ask "<question>"        # candidates as pins (hybrid BM25 + vectors)
    glyph show g-a1b2             # spend budget on ONE node (full body + refs)
    glyph show g-a1b2 --facet neighborhood   # one-hop expand
    glyph related g-a1b2          # neighbors as pins

## Writing (same energy as daily notes — timestamps are automatic)

    glyph etch --type note "Tried X; Y failed because Z"
    glyph etch --type decision --tag retrieval --ref url:https://… "We chose A over B"
    glyph link g-a1b2 g-c3d4 --as supports

**Knowledge changed? Amend, don't re-etch a near-duplicate:**

    glyph amend g-a1b2 "Updated conclusion"
    glyph amend g-a1b2 --tag +packing --tag -draft --ref +path:internal/store/store.go

## Reading with filters (time is a query, never a write flag)

    glyph list --type note --today
    glyph list --since 2026-07-01 --tag retrieval
    glyph ask "what did we decide about retrieval?" --limit 8
    glyph ask "open questions" --type decision --today   # ask takes the same filters as list

## The three relations — do not collapse them

- **Tags** classify: what kind/topic is this? (--tag retrieval)
- **Refs** cite: loose reference to anything outside the graph (--ref url:… , path:… , bead:… , uuid:…)
- **Links** connect two glyphs with a typed edge (glyph link A B --as supports)

Never put a URL in a tag. Never ref another glyph — link it.

## Facet ladder (disclosure levels, cheapest first)

    id → pin → card → body → neighborhood

Defaults: list/ask/related return **pin**; show returns **body**. Use --facet to override.
When output says "showing N of M", the window is full — memory is not empty.
Budgets are counted in **words** (not tokens; ~4 words ≈ 3 tokens). context defaults to --budget 300.

## Retrieval quality

ask always works: BM25 keyword search is the floor. If an embedding model is
configured (glyph model), semantic matches blend in — same command, better ranking.
No model, no problem: results are keyword + tag matches only.

## Rules of thumb

- Etch freely and briefly; one idea per glyph.
- Deepen intentionally: one show at a time, not full bodies in bulk.
- Amend stale knowledge; forget wrong knowledge (glyph forget g-x).
- Add --json for machine-readable output (or GLYPH_FORMAT=json).
`

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Print the agent usage skill (teaches progressive disclosure)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonOut() {
			return emitJSON(map[string]string{"skill": skillText})
		}
		fmt.Print(skillText)
		return nil
	},
}
