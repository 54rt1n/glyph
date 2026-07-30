package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/config"
	"github.com/54rt1n/glyph/internal/embed"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Select/download the embedding model (HuggingFace weights)",
	Long: `Without arguments, shows the active model and whether its weights are cached.
No model configured → glyph runs BM25-only, which is always safe.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := config.Find(".")
		if err != nil {
			return err
		}
		settings, err := proj.LoadSettings()
		if err != nil {
			return err
		}
		status := map[string]any{"model": settings.Model, "downloaded": false, "active": false}
		if settings.Model != "" {
			repo := embed.Resolve(settings.Model)
			status["model"] = repo
			status["downloaded"] = embed.Downloaded(repo)
			status["active"] = embed.Downloaded(repo)
			if dir, err := config.ModelCacheDir(repo); err == nil {
				status["cache"] = dir
			}
		}
		if jsonOut() {
			return emitJSON(status)
		}
		if settings.Model == "" {
			fmt.Println("no embedding model configured — ask runs BM25-only")
			fmt.Println("enable with: glyph model get default && glyph model use default")
			return nil
		}
		fmt.Printf("model: %s\n", status["model"])
		if status["downloaded"].(bool) {
			fmt.Printf("weights: downloaded (%s)\n", status["cache"])
			fmt.Println("hybrid ask: active")
		} else {
			fmt.Println("weights: NOT downloaded — run `glyph model get " + settings.Model + "`")
			fmt.Println("hybrid ask: inactive (BM25-only)")
		}
		return nil
	},
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List curated model aliases and cached downloads",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		type entry struct {
			Name       string `json:"name"`
			Repo       string `json:"repo"`
			Downloaded bool   `json:"downloaded"`
		}
		var entries []entry
		seen := map[string]bool{}
		for alias, repo := range embed.Aliases {
			if seen[repo] {
				continue
			}
			seen[repo] = true
			entries = append(entries, entry{Name: alias, Repo: repo, Downloaded: embed.Downloaded(repo)})
		}
		if root, err := config.ModelsRoot(); err == nil {
			if dirs, err := os.ReadDir(root); err == nil {
				for _, d := range dirs {
					repo := strings.ReplaceAll(d.Name(), "--", "/")
					if !seen[repo] {
						seen[repo] = true
						entries = append(entries, entry{Name: repo, Repo: repo, Downloaded: embed.Downloaded(repo)})
					}
				}
			}
		}
		if jsonOut() {
			return emitJSON(entries)
		}
		for _, e := range entries {
			mark := " "
			if e.Downloaded {
				mark = "✓"
			}
			fmt.Printf("%s %-10s %s\n", mark, e.Name, e.Repo)
		}
		fmt.Println("\nget with: glyph model get <name|repo>   activate: glyph model use <name|repo>")
		return nil
	},
}

var modelGetCmd = &cobra.Command{
	Use:   "get <name|repo>",
	Short: "Download model weights from HuggingFace to the user cache",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := embed.Download(args[0], os.Stderr)
		if err != nil {
			return err
		}
		if jsonOut() {
			return emitJSON(map[string]string{"model": embed.Resolve(args[0]), "cache": dir})
		}
		fmt.Printf("downloaded %s → %s\n", embed.Resolve(args[0]), dir)
		fmt.Printf("activate with: glyph model use %s\n", args[0])
		return nil
	},
}

var modelUseCmd = &cobra.Command{
	Use:   "use <name|repo>",
	Short: "Set the active embedding model for this project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, st, err := openStore()
		if err != nil {
			return err
		}
		defer st.Close()
		repo := embed.Resolve(args[0])
		settings, err := proj.LoadSettings()
		if err != nil {
			return err
		}
		// Dims are locked per store: switching models invalidates old vectors.
		if settings.Model != "" && embed.Resolve(settings.Model) != repo {
			if n, err := st.VecCount(); err == nil && n > 0 {
				fmt.Fprintf(os.Stderr, "switching models: clearing %d stored vectors (re-embed happens lazily on etch/amend)\n", n)
				if err := st.ClearVecs(); err != nil {
					return err
				}
			}
		}
		settings.Model = repo
		if err := proj.SaveSettings(settings); err != nil {
			return err
		}
		if !embed.Downloaded(repo) {
			fmt.Fprintf(os.Stderr, "note: weights not downloaded yet — run `glyph model get %s`\n", args[0])
		}
		if jsonOut() {
			return emitJSON(map[string]any{"model": repo, "downloaded": embed.Downloaded(repo)})
		}
		fmt.Printf("active model: %s\n", repo)
		return nil
	},
}

func init() {
	modelCmd.AddCommand(modelListCmd, modelGetCmd, modelUseCmd)
}
