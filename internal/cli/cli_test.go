package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/54rt1n/glyph/internal/types"
)

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
