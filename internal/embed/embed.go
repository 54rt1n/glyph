// Package embed provides the one embedding integration path: local static
// embeddings via gobed, with weights fetched from HuggingFace by
// `glyph model get`. No model downloaded → not configured → BM25-only.
package embed

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/54rt1n/glyph/internal/config"
)

// DefaultModel is the curated default (what gobed was built around).
const DefaultModel = "sentence-transformers/static-retrieval-mrl-en-v1"

// Aliases maps short names to HuggingFace repos.
var Aliases = map[string]string{
	"default": DefaultModel,
	"mrl":     DefaultModel,
}

// Resolve expands an alias to a repo name (pass-through otherwise).
func Resolve(name string) string {
	if repo, ok := Aliases[name]; ok {
		return repo
	}
	return name
}

// Embedder encodes text into vectors.
type Embedder interface {
	Encode(text string) ([]float32, error)
	Model() string
	Dims() int
}

// ErrNotConfigured signals no usable model; callers should proceed BM25-only.
var ErrNotConfigured = errors.New("no embedding model configured")

// weightsFile and tokenizerFile are the filenames gobed expects inside
// GOBED_MODEL_PATH.
const (
	weightsFile   = "real_model.safetensors"
	tokenizerFile = "tokenizer.json"
)

// Downloaded reports whether the model's weights exist in the cache.
func Downloaded(model string) bool {
	dir, err := config.ModelCacheDir(Resolve(model))
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, weightsFile))
	return err == nil
}

// Load returns the embedder for the model named in settings, or
// ErrNotConfigured when settings name no model or its weights are absent.
func Load(settings config.Settings) (Embedder, error) {
	if settings.Model == "" {
		return nil, ErrNotConfigured
	}
	repo := Resolve(settings.Model)
	if !Downloaded(repo) {
		return nil, ErrNotConfigured
	}
	dir, err := config.ModelCacheDir(repo)
	if err != nil {
		return nil, err
	}
	return loadGobed(dir, repo)
}
