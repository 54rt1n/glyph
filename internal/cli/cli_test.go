package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/54rt1n/glyph/internal/config"
	"github.com/54rt1n/glyph/internal/store"
	"github.com/54rt1n/glyph/internal/types"
)

func sampleGlyph() *types.Glyph {
	return &types.Glyph{
		ID:   "g-a1b2",
		Body: "Pins by default; deepen with show.\nSecond line stays out of the ack.",
		Type: "decision",
		Tags: []string{"retrieval", "v0"},
	}
}

func TestEnsureGitignoreAddsMigrationLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("*.db\ncustom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ensureGitignore(config.Project{Dir: dir})
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"*.db", "custom", "*.migrate.lock"} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("gitignore missing %q: %s", want, b)
		}
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		in      string
		want    types.Ref
		wantErr string
	}{
		{in: "url:https://arxiv.org/abs/2501.13956", want: types.Ref{Kind: "url", Target: "https://arxiv.org/abs/2501.13956"}},
		{in: "path:internal/store/store.go", want: types.Ref{Kind: "path", Target: "internal/store/store.go"}},
		{in: "bead:bd-x7k2", want: types.Ref{Kind: "bead", Target: "bd-x7k2"}},
		{in: "url:http://host:8080/x?a=b:c", want: types.Ref{Kind: "url", Target: "http://host:8080/x?a=b:c"}},
		{in: "noseparator", wantErr: "expected kind:target"},
		{in: ":target-only", wantErr: "expected kind:target"},
		{in: "kind:", wantErr: "expected kind:target"},
		{in: "glyph:g-a1b2", wantErr: "use `glyph link"},
	}
	for _, tt := range tests {
		got, err := parseRef(tt.in)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("parseRef(%q) err = %v, want containing %q", tt.in, err, tt.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseRef(%q) err = %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseRef(%q) = %+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestCleanSummary(t *testing.T) {
	got, err := cleanSummary("  Quill: chapter trade pending  ")
	if err != nil || got != "Quill: chapter trade pending" {
		t.Fatalf("clean summary = %q err=%v", got, err)
	}
	if _, err := cleanSummary("first\nsecond"); err == nil {
		t.Fatal("multiline summary accepted")
	}
}

func TestSplitSigned(t *testing.T) {
	add, rm, err := splitSigned([]string{"+packing", "-v0", "+final"}, "tag")
	if err != nil {
		t.Fatal(err)
	}
	if len(add) != 2 || add[0] != "packing" || add[1] != "final" {
		t.Fatalf("add = %v", add)
	}
	if len(rm) != 1 || rm[0] != "v0" {
		t.Fatalf("rm = %v", rm)
	}
	// signed refs keep their kind:target payload intact
	add, _, err = splitSigned([]string{"+url:https://example.com"}, "ref")
	if err != nil || add[0] != "url:https://example.com" {
		t.Fatalf("signed ref add = %v err=%v", add, err)
	}
	for _, bad := range []string{"nosign", "+", "-"} {
		if _, _, err := splitSigned([]string{bad}, "tag"); err == nil {
			t.Errorf("splitSigned(%q) accepted, want error", bad)
		}
	}
}

func TestTimeFiltersResolve(t *testing.T) {
	tf := timeFilters{today: true}
	since, until, err := tf.resolve()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !since.Equal(midnight) || !until.IsZero() {
		t.Fatalf("today: since=%v until=%v", since, until)
	}

	tf = timeFilters{since: "2026-07-01", until: "2026-07-30"}
	since, until, err = tf.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if since.Year() != 2026 || since.Month() != 7 || since.Day() != 1 {
		t.Fatalf("since = %v", since)
	}
	if until.Day() != 30 {
		t.Fatalf("until = %v", until)
	}

	tf = timeFilters{since: "2026-07-29T15:04:05Z"}
	if since, _, err = tf.resolve(); err != nil || since.IsZero() {
		t.Fatalf("rfc3339: %v %v", since, err)
	}

	tf = timeFilters{since: "yesterday"}
	if _, _, err = tf.resolve(); err == nil || !strings.Contains(err.Error(), "invalid date") {
		t.Fatalf("bad date err = %v", err)
	}
}

func TestRenderContextStanding(t *testing.T) {
	g := sampleGlyph()
	out := renderContext(&store.Stats{
		Glyphs: 2, Edges: 1, Tag: "agent-runtime",
		ByType:  []store.TypeCount{{Type: "decision", Count: 1}, {Type: "note", Count: 1}},
		TopTags: []store.TagCount{{Tag: "v0", Count: 1}},
		Recent:  []*types.Glyph{g},
	}, 300, false)
	for _, want := range []string{"tag agent-runtime", "standing:", "g-a1b2", "co-tags:", "v0(1)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("standing context missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\nrecent:") {
		t.Fatalf("tagged context still says recent:\n%s", out)
	}
}

func TestRenderContextFocusDeduplicatesRecentAndVerboseInventory(t *testing.T) {
	focus := sampleGlyph()
	focus.Starred = true
	recent := sampleGlyph()
	recent.ID = "g-c3d4"
	stats := &store.Stats{
		Glyphs: 2, Edges: 1, Refs: 4, Vecs: 2, Today: 2, ThisWeek: 2,
		Focus: []*types.Glyph{focus}, Recent: []*types.Glyph{focus, recent},
		ByType: []store.TypeCount{{Type: "decision", Count: 2}},
	}
	out := renderContext(stats, 300, false)
	for _, want := range []string{"2 glyphs · 1 edges", "focus:", "recent:", "★", "g-c3d4"} {
		if !strings.Contains(out, want) {
			t.Fatalf("context missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, focus.ID) != 1 || strings.Contains(out, "inventory:") {
		t.Fatalf("default context duplicate/inventory:\n%s", out)
	}
	verbose := renderContext(stats, 300, true)
	if !strings.Contains(verbose, "inventory: 4 refs · 2 vectors") || !strings.Contains(verbose, "by type:") {
		t.Fatalf("verbose context:\n%s", verbose)
	}
}

func TestContextDefaultsToUnlimitedBudget(t *testing.T) {
	if got := contextCmd.Flags().Lookup("budget").DefValue; got != "0" {
		t.Fatalf("context budget default = %q, want unlimited", got)
	}
}

func TestJSONGlyphUsesSummaryAndStar(t *testing.T) {
	g := sampleGlyph()
	g.Summary = "scan me"
	g.Starred = true
	m := jsonGlyph(types.FacetPin, g).(map[string]any)
	if m["line"] != "scan me" || m["summary"] != "scan me" || m["starred"] != true {
		t.Fatalf("pin json = %#v", m)
	}
}

func TestJSONTraversalKeepsRelatedGlyphsAndAddsEdges(t *testing.T) {
	root, child := sampleGlyph(), sampleGlyph()
	child.ID = "g-c3d4"
	traversal := &types.Traversal{
		Root: root, Glyphs: []*types.Glyph{root, child}, Depth: 1, Direction: types.DirectionBoth,
		Links: []*types.TraversalLink{{
			Edge: &types.Edge{ID: "e-1", Src: root.ID, Dst: child.ID, Rel: "supports"},
			From: root.ID, To: child.ID, Direction: types.DirectionOut, Depth: 1,
		}},
	}
	out := jsonTraversal(types.FacetPin, traversal)
	if nodes := out["nodes"].([]any); len(nodes) != 2 {
		t.Fatalf("nodes = %#v", nodes)
	}
	glyphs := out["glyphs"].([]any)
	if len(glyphs) != 1 || glyphs[0].(map[string]any)["rel"] != "supports" {
		t.Fatalf("compat glyphs = %#v", glyphs)
	}
	if edges := out["edges"].([]map[string]any); len(edges) != 1 || edges[0]["direction"] != types.DirectionOut {
		t.Fatalf("edges = %#v", edges)
	}
}

func TestParseFacet(t *testing.T) {
	f, err := parseFacet("", types.FacetPin)
	if err != nil || f != types.FacetPin {
		t.Fatalf("fallback: %v %v", f, err)
	}
	f, err = parseFacet("neighborhood", types.FacetPin)
	if err != nil || f != types.FacetNeighborhood {
		t.Fatalf("explicit: %v %v", f, err)
	}
	if _, err := parseFacet("gigantic", types.FacetPin); err == nil {
		t.Fatal("unknown facet accepted")
	}
}
