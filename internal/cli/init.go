package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/54rt1n/glyph/internal/config"
	"github.com/54rt1n/glyph/internal/store"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the .glyph store for this project",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := config.Init(".")
		if err != nil {
			return err
		}
		st, err := store.Open(proj.DBPath())
		if err != nil {
			return err
		}
		defer st.Close()
		ensureGitignore(proj)
		if jsonOut() {
			return emitJSON(map[string]string{"status": "ok", "store": proj.DBPath()})
		}
		fmt.Printf("initialized glyph store at %s\n", proj.Dir)
		fmt.Println("next: `glyph skill` to learn the tool, `glyph etch \"…\"` to write")
		return nil
	},
}

// ensureGitignore keeps the live DB out of the user's git without touching
// anything else (design rule: don't pollute the workspace).
func ensureGitignore(proj config.Project) {
	path := filepath.Join(proj.Dir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return
	}
	_ = os.WriteFile(path, []byte("*.db\n*.db-wal\n*.db-shm\n"), 0o644)
}
