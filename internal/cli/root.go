// Package cli wires the twelve glyph commands.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/config"
	"github.com/54rt1n/glyph/internal/embed"
	"github.com/54rt1n/glyph/internal/store"
	"github.com/54rt1n/glyph/internal/types"
)

var jsonFlag bool

// Version is the glyph release version.
const Version = "1.0.0"

var rootCmd = &cobra.Command{
	Use:           "glyph",
	Short:         "Local graph memory for agents — CLI-first, no worker required",
	Long:          "glyph v" + Version + " — local graph memory for agents, CLI-first, no worker required.",
	Version:       Version,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "glyph:", err)
		return 1
	}
	return 0
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "emit JSON (also GLYPH_FORMAT=json)")
	rootCmd.AddCommand(initCmd, etchCmd, amendCmd, askCmd, showCmd, listCmd,
		linkCmd, relatedCmd, forgetCmd, contextCmd, skillCmd, modelCmd)
}

func jsonOut() bool { return config.JSONOutput(jsonFlag) }

// openStore locates the project and opens its database.
func openStore() (config.Project, *store.Store, error) {
	proj, err := config.Find(".")
	if err != nil {
		return config.Project{}, nil, err
	}
	st, err := store.Open(proj.DBPath())
	if err != nil {
		return config.Project{}, nil, err
	}
	return proj, st, nil
}

// loadEmbedder returns the configured embedder or nil (BM25-only) — never an
// error the caller must handle; embedding is strictly optional.
func loadEmbedder(proj config.Project) embed.Embedder {
	settings, err := proj.LoadSettings()
	if err != nil {
		return nil
	}
	emb, err := embed.Load(settings)
	if err != nil {
		return nil
	}
	return emb
}

// embedGlyph stores the vector for a glyph if an embedder is available.
func embedGlyph(st *store.Store, emb embed.Embedder, gid, body string) {
	if emb == nil {
		return
	}
	if v, err := emb.Encode(body); err == nil && v != nil {
		_ = st.SetVec(gid, emb.Model(), v)
	}
}

func emitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// parseRef parses kind:target (target may itself contain colons, e.g. URLs).
func parseRef(s string) (types.Ref, error) {
	kind, target, ok := strings.Cut(s, ":")
	if !ok || kind == "" || target == "" {
		return types.Ref{}, fmt.Errorf("invalid ref %q: expected kind:target (e.g. url:https://…, path:main.go)", s)
	}
	// Policy: refs point outside the graph. If both ends are glyphs, that's a link.
	if kind == "glyph" {
		return types.Ref{}, fmt.Errorf("refs don't point at glyphs — use `glyph link <src> %s --as related` instead", target)
	}
	return types.Ref{Kind: kind, Target: target}, nil
}

// parseFacet validates a --facet value, with fallback when empty.
func parseFacet(s string, fallback types.Facet) (types.Facet, error) {
	if s == "" {
		return fallback, nil
	}
	if !types.ValidFacet(s) {
		return "", fmt.Errorf("unknown facet %q (id, pin, card, body, neighborhood, raw)", s)
	}
	return types.Facet(s), nil
}

// timeFilters holds the shared --today/--since/--until query flags.
type timeFilters struct {
	today bool
	since string
	until string
}

func (t *timeFilters) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&t.today, "today", false, "only glyphs etched today")
	cmd.Flags().StringVar(&t.since, "since", "", "only glyphs etched on/after DATE (2006-01-02 or RFC3339)")
	cmd.Flags().StringVar(&t.until, "until", "", "only glyphs etched before DATE")
}

func (t *timeFilters) resolve() (since, until time.Time, err error) {
	if t.today {
		now := time.Now()
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	if t.since != "" {
		since, err = parseDate(t.since)
		if err != nil {
			return
		}
	}
	if t.until != "" {
		until, err = parseDate(t.until)
	}
	return
}

func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if ts, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q (want 2006-01-02 or RFC3339)", s)
}
