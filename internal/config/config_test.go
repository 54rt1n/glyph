package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	proj, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(root); mustEval(t, proj.Root) != want {
		t.Fatalf("Find root = %q, want %q", proj.Root, root)
	}
}

func TestFindNotInitialized(t *testing.T) {
	_, err := Find(t.TempDir())
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("err = %v, want ErrNotInitialized", err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	proj, err := Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := proj.LoadSettings() // missing file → zero settings
	if err != nil || s.Model != "" {
		t.Fatalf("empty load = %+v, %v", s, err)
	}
	s.Model = "sentence-transformers/static-retrieval-mrl-en-v1"
	if err := proj.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got, err := proj.LoadSettings()
	if err != nil || got.Model != s.Model {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
