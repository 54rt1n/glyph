// Package config locates the project store and holds per-project settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName    = ".glyph"
	DBName     = "glyph.db"
	ConfigName = "config.json"
)

// ErrNotInitialized is returned when no .glyph directory is found.
var ErrNotInitialized = errors.New("no .glyph store found — run `glyph init` in your project root")

// Project describes a located glyph store.
type Project struct {
	Root string // directory containing .glyph
	Dir  string // the .glyph directory
}

func (p Project) DBPath() string     { return filepath.Join(p.Dir, DBName) }
func (p Project) ConfigPath() string { return filepath.Join(p.Dir, ConfigName) }

// Find walks up from dir looking for a .glyph directory.
func Find(dir string) (Project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Project{}, err
	}
	for {
		g := filepath.Join(dir, DirName)
		if st, err := os.Stat(g); err == nil && st.IsDir() {
			return Project{Root: dir, Dir: g}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Project{}, ErrNotInitialized
		}
		dir = parent
	}
}

// Init creates .glyph under root. It is idempotent.
func Init(root string) (Project, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Project{}, err
	}
	g := filepath.Join(root, DirName)
	if err := os.MkdirAll(g, 0o755); err != nil {
		return Project{}, err
	}
	return Project{Root: root, Dir: g}, nil
}

// Settings is the per-project config stored in .glyph/config.json.
type Settings struct {
	// Model is the active embedding model (HuggingFace repo or alias); empty = BM25-only.
	Model string `json:"model,omitempty"`
}

// LoadSettings reads .glyph/config.json; a missing file yields zero settings.
func (p Project) LoadSettings() (Settings, error) {
	var s Settings
	b, err := os.ReadFile(p.ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", p.ConfigPath(), err)
	}
	return s, nil
}

// SaveSettings writes .glyph/config.json.
func (p Project) SaveSettings(s Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p.ConfigPath(), append(b, '\n'), 0o644)
}

// JSONOutput reports whether output should be JSON, combining the --json flag
// with the GLYPH_FORMAT env var.
func JSONOutput(flag bool) bool {
	if flag {
		return true
	}
	return strings.EqualFold(os.Getenv("GLYPH_FORMAT"), "json")
}

// ModelCacheDir returns the user-level directory holding downloaded model
// weights for the given model name (slashes sanitized).
func ModelCacheDir(model string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	safe := strings.ReplaceAll(model, "/", "--")
	return filepath.Join(base, "glyph", "models", safe), nil
}

// ModelsRoot returns the root cache directory for all models.
func ModelsRoot() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "glyph", "models"), nil
}
