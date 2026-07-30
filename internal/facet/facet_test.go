package facet

import (
	"strings"
	"testing"
	"time"

	"github.com/54rt1n/glyph/internal/types"
)

func sample() *types.Glyph {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	return &types.Glyph{
		ID:        "g-a1b2",
		Body:      "Pins by default; deepen with show.\nSecond line should never appear in a pin.",
		Type:      "decision",
		Tags:      []string{"retrieval", "v0"},
		Refs:      []types.Ref{{Kind: "url", Target: "https://example.com"}, {Kind: "bead", Target: "bd-x7k2"}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestPinIsOneLine(t *testing.T) {
	p := Pin(sample())
	if strings.Contains(p, "\n") {
		t.Fatalf("pin has newline: %q", p)
	}
	for _, want := range []string{"g-a1b2", "decision", "Pins by default", "↗2"} {
		if !strings.Contains(p, want) {
			t.Fatalf("pin %q missing %q", p, want)
		}
	}
	if strings.Contains(p, "Second line") {
		t.Fatalf("pin leaked second line: %q", p)
	}
}

func TestPinLongBodyTruncated(t *testing.T) {
	g := sample()
	g.Body = strings.Repeat("word ", 100)
	p := Pin(g)
	if !strings.Contains(p, "…") {
		t.Fatalf("long pin not truncated: %q", p)
	}
}

func TestBodyShowsEverything(t *testing.T) {
	b := Body(sample())
	for _, want := range []string{"Second line", "tags: retrieval, v0", "url:https://example.com", "bead:bd-x7k2"} {
		if !strings.Contains(b, want) {
			t.Fatalf("body missing %q:\n%s", want, b)
		}
	}
}

func TestNeighborhoodAppendsPins(t *testing.T) {
	n := sample()
	n.ID = "g-c3d4"
	n.Rel = "supports"
	out := Neighborhood(sample(), []*types.Glyph{n})
	if !strings.Contains(out, "neighbors:") || !strings.Contains(out, "g-c3d4") {
		t.Fatalf("neighborhood:\n%s", out)
	}
}

func TestTruncationNote(t *testing.T) {
	if got := TruncationNote(8, 40, "--limit"); !strings.Contains(got, "8 of 40") {
		t.Fatalf("note = %q", got)
	}
	if got := TruncationNote(5, 5, "--limit"); got != "" {
		t.Fatalf("no-cut note = %q", got)
	}
}

func TestBudgetLines(t *testing.T) {
	lines := []string{"one two three", "four five six", "seven eight nine"}
	kept, dropped := BudgetLines(lines, 7)
	if len(kept) != 2 || dropped != 1 {
		t.Fatalf("kept=%d dropped=%d", len(kept), dropped)
	}
	// budget 0 = unlimited
	kept, dropped = BudgetLines(lines, 0)
	if len(kept) != 3 || dropped != 0 {
		t.Fatalf("unlimited kept=%d dropped=%d", len(kept), dropped)
	}
	// first line always kept even over budget
	kept, _ = BudgetLines(lines, 1)
	if len(kept) != 1 {
		t.Fatalf("first line not kept: %v", kept)
	}
}
