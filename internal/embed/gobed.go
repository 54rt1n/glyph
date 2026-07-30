package embed

import (
	"fmt"
	"os"

	"github.com/lee101/gobed"
)

// gobedEmbedder wraps gobed's EmbeddingModel behind the Embedder interface.
type gobedEmbedder struct {
	model *gobed.EmbeddingModel
	repo  string
}

// loadGobed loads weights from dir. gobed discovers files via the
// GOBED_MODEL_PATH env var and prints progress to stdout; both are contained
// here so the CLI surface stays clean.
func loadGobed(dir, repo string) (Embedder, error) {
	os.Setenv("GOBED_MODEL_PATH", dir)
	m, err := silenced(gobed.LoadModel)
	if err != nil {
		return nil, fmt.Errorf("load embedding model from %s: %w", dir, err)
	}
	return &gobedEmbedder{model: m, repo: repo}, nil
}

func (e *gobedEmbedder) Encode(text string) ([]float32, error) {
	return silenced(func() ([]float32, error) { return e.model.Encode(text) })
}

func (e *gobedEmbedder) Model() string { return e.repo }
func (e *gobedEmbedder) Dims() int     { return e.model.EmbedDim }

// silenced runs fn with os.Stdout swapped to /dev/null: gobed logs loading
// noise via fmt.Println, which would corrupt --json output.
func silenced[T any](fn func() (T, error)) (T, error) {
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fn()
	}
	defer devnull.Close()
	saved := os.Stdout
	os.Stdout = devnull
	defer func() { os.Stdout = saved }()
	return fn()
}
