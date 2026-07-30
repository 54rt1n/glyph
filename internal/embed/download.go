package embed

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/54rt1n/glyph/internal/config"
)

// hfCandidates lists, per required file, the repo-relative paths to try in
// order. sentence-transformers static models keep weights either at the root
// or under 0_StaticEmbedding/.
var hfCandidates = map[string][]string{
	weightsFile:   {"model.safetensors", "0_StaticEmbedding/model.safetensors"},
	tokenizerFile: {"tokenizer.json", "0_StaticEmbedding/tokenizer.json"},
}

// Download fetches the model's weights and tokenizer from HuggingFace into
// the user cache, reporting progress to progressW (may be nil). Returns the
// cache directory.
func Download(model string, progressW io.Writer) (string, error) {
	repo := Resolve(model)
	dir, err := config.ModelCacheDir(repo)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	for local, remotes := range hfCandidates {
		dest := filepath.Join(dir, local)
		if _, err := os.Stat(dest); err == nil {
			fmt.Fprintf(progressOr(progressW), "%s already present\n", local)
			continue
		}
		var lastErr error
		ok := false
		for _, remote := range remotes {
			url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", repo, remote)
			if err := fetch(client, url, dest, progressW); err != nil {
				lastErr = err
				continue
			}
			ok = true
			break
		}
		if !ok {
			return "", fmt.Errorf("download %s for %s: %w", local, repo, lastErr)
		}
	}
	return dir, nil
}

func fetch(client *http.Client, url, dest string, progressW io.Writer) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	fmt.Fprintf(progressOr(progressW), "downloading %s (%s)…\n", url, humanSize(resp.ContentLength))
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

func humanSize(n int64) string {
	switch {
	case n <= 0:
		return "unknown size"
	case n < 1<<20:
		return fmt.Sprintf("%d KB", n>>10)
	default:
		return fmt.Sprintf("%d MB", n>>20)
	}
}

func progressOr(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
