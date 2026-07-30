package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/54rt1n/glyph/internal/types"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "glyph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mkGlyph(t *testing.T, s *Store, id, body, typ string, tags []string, refs []types.Ref) *types.Glyph {
	t.Helper()
	now := time.Now()
	g := &types.Glyph{ID: id, Body: body, Type: typ, Tags: tags, Refs: refs, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateGlyph(g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestCRUDRoundTrip(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "hello graph memory", "note",
		[]string{"v0", "retrieval"}, []types.Ref{{Kind: "url", Target: "https://example.com"}})
	g, err := s.GetGlyph("g-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if g.Body != "hello graph memory" || g.Type != "note" {
		t.Fatalf("got %+v", g)
	}
	if len(g.Tags) != 2 || len(g.Refs) != 1 {
		t.Fatalf("tags=%v refs=%v", g.Tags, g.Refs)
	}
	if _, err := s.GetGlyph("g-nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing glyph err = %v", err)
	}
}

func TestDeleteCascades(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "src", "note", []string{"x"}, []types.Ref{{Kind: "path", Target: "a.go"}})
	mkGlyph(t, s, "g-bbbb", "dst", "note", nil, nil)
	if _, err := s.CreateEdge("g-aaaa", "g-bbbb", "supports"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVec("g-aaaa", "m", []float32{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteGlyph("g-aaaa"); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.VecCount(); n != 0 {
		t.Fatalf("vecs not cascaded: %d", n)
	}
	gs, err := s.Related("g-bbbb", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 0 {
		t.Fatalf("edges not cascaded: %v", gs)
	}
	if err := s.DeleteGlyph("g-aaaa"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("double delete err = %v", err)
	}
}

func TestAmend(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "old body", "note", []string{"draft"}, []types.Ref{{Kind: "url", Target: "https://old"}})
	body := "new body"
	typ := "decision"
	g, changed, err := s.AmendGlyph("g-aaaa", &body, &typ,
		[]string{"final"}, []string{"draft"},
		[]types.Ref{{Kind: "path", Target: "x.go"}}, []types.Ref{{Kind: "url", Target: "https://old"}})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("body change not reported")
	}
	if g.Body != "new body" || g.Type != "decision" {
		t.Fatalf("got %+v", g)
	}
	if len(g.Tags) != 1 || g.Tags[0] != "final" {
		t.Fatalf("tags = %v", g.Tags)
	}
	if len(g.Refs) != 1 || g.Refs[0].Kind != "path" {
		t.Fatalf("refs = %v", g.Refs)
	}
	// amending with same body reports no change
	_, changed, err = s.AmendGlyph("g-aaaa", &body, nil, nil, nil, nil, nil)
	if err != nil || changed {
		t.Fatalf("same-body amend: changed=%v err=%v", changed, err)
	}
	if _, _, err := s.AmendGlyph("g-nope", &body, nil, nil, nil, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing amend err = %v", err)
	}
}

func TestListFilters(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "a note", "note", []string{"x"}, nil)
	mkGlyph(t, s, "g-bbbb", "a decision", "decision", []string{"x", "y"}, nil)
	mkGlyph(t, s, "g-cccc", "another note", "note", nil, nil)

	gs, total, err := s.ListGlyphs(ListFilter{Type: "note"})
	if err != nil || total != 2 || len(gs) != 2 {
		t.Fatalf("type filter: %d/%d %v", len(gs), total, err)
	}
	gs, total, err = s.ListGlyphs(ListFilter{Tag: "y"})
	if err != nil || total != 1 || gs[0].ID != "g-bbbb" {
		t.Fatalf("tag filter: %v %d %v", gs, total, err)
	}
	gs, total, err = s.ListGlyphs(ListFilter{Since: time.Now().Add(time.Hour)})
	if err != nil || total != 0 || len(gs) != 0 {
		t.Fatalf("future since: %v %d %v", gs, total, err)
	}
	gs, total, err = s.ListGlyphs(ListFilter{Limit: 2})
	if err != nil || total != 3 || len(gs) != 2 {
		t.Fatalf("limit: %d/%d %v", len(gs), total, err)
	}
}

func TestRelatedBothDirections(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "center", "note", nil, nil)
	mkGlyph(t, s, "g-bbbb", "outgoing", "note", nil, nil)
	mkGlyph(t, s, "g-cccc", "incoming", "note", nil, nil)
	if _, err := s.CreateEdge("g-aaaa", "g-bbbb", "supports"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateEdge("g-cccc", "g-aaaa", "refutes"); err != nil {
		t.Fatal(err)
	}
	gs, err := s.Related("g-aaaa", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 2 {
		t.Fatalf("related = %v", gs)
	}
	rels := map[string]string{}
	for _, g := range gs {
		rels[g.ID] = g.Rel
	}
	if rels["g-bbbb"] != "supports" || rels["g-cccc"] != "refutes" {
		t.Fatalf("rels = %v", rels)
	}
}

func TestFTSSyncAndSearch(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "retrieval pins by default", "decision", nil, nil)
	mkGlyph(t, s, "g-bbbb", "unrelated musings about cooking", "note", nil, nil)

	hits, err := s.Search("What did we decide about retrieval?", 10, nil, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != "g-aaaa" {
		t.Fatalf("hits = %v", hits)
	}

	// update propagates through the FTS trigger
	body := "cooking pasta perfectly"
	if _, _, err := s.AmendGlyph("g-aaaa", &body, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	hits, err = s.Search("retrieval", 10, nil, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("stale FTS entry survived amend: %v", hits)
	}

	// delete propagates too
	if err := s.DeleteGlyph("g-bbbb"); err != nil {
		t.Fatal(err)
	}
	hits, err = s.Search("musings", 10, nil, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("stale FTS entry survived delete: %v", hits)
	}
}

func TestSearchMatchesTags(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "pins by default", "decision", []string{"retrieval"}, nil)
	mkGlyph(t, s, "g-bbbb", "cooking notes", "note", nil, nil)
	hits, err := s.Search("what did we decide about retrieval?", 10, nil, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "g-aaaa" {
		t.Fatalf("tag search hits = %v", hits)
	}
	// body matches still outrank tag-only matches
	mkGlyph(t, s, "g-cccc", "retrieval strategy in the body", "note", nil, nil)
	hits, err = s.Search("retrieval", 10, nil, ListFilter{})
	if err != nil || len(hits) != 2 || hits[0].ID != "g-cccc" {
		t.Fatalf("mixed hits = %v err=%v", hits, err)
	}
}

func TestSearchWithFilter(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "retrieval as a decision", "decision", []string{"v0"}, nil)
	mkGlyph(t, s, "g-bbbb", "retrieval as a note", "note", nil, nil)

	hits, err := s.Search("retrieval", 10, nil, ListFilter{Type: "decision"})
	if err != nil || len(hits) != 1 || hits[0].ID != "g-aaaa" {
		t.Fatalf("type-filtered hits = %v err=%v", hits, err)
	}
	hits, err = s.Search("retrieval", 10, nil, ListFilter{Tag: "v0"})
	if err != nil || len(hits) != 1 || hits[0].ID != "g-aaaa" {
		t.Fatalf("tag-filtered hits = %v err=%v", hits, err)
	}
	hits, err = s.Search("retrieval", 10, nil, ListFilter{Since: time.Now().Add(time.Hour)})
	if err != nil || len(hits) != 0 {
		t.Fatalf("future-since hits = %v err=%v", hits, err)
	}
}

// fakeEmbedder maps known texts to fixed vectors.
type fakeEmbedder struct{ vecs map[string][]float32 }

func (f *fakeEmbedder) Encode(text string) ([]float32, error) {
	if v, ok := f.vecs[text]; ok {
		return v, nil
	}
	return []float32{0, 0, 1}, nil
}
func (f *fakeEmbedder) Model() string { return "fake" }

func TestHybridSearchBlendsVectors(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "pins by default for output", "decision", nil, nil)
	mkGlyph(t, s, "g-bbbb", "retrieval strategy notes", "note", nil, nil)
	if err := s.SetVec("g-aaaa", "fake", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVec("g-bbbb", "fake", []float32{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	// query has no lexical overlap with g-aaaa but its vector matches
	emb := &fakeEmbedder{vecs: map[string][]float32{"disclosure defaults": {1, 0, 0}}}
	hits, err := s.Search("disclosure defaults", 10, emb, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].ID != "g-aaaa" {
		t.Fatalf("hybrid hits = %v, want g-aaaa first", hits)
	}
	// BM25-only (nil embedder) still works
	hits, err = s.Search("retrieval strategy", 10, nil, ListFilter{})
	if err != nil || len(hits) == 0 || hits[0].ID != "g-bbbb" {
		t.Fatalf("bm25 hits = %v err=%v", hits, err)
	}
}

func TestVecRoundTrip(t *testing.T) {
	in := []float32{1.5, -2.25, 0, 3.14159}
	out := DecodeVec(EncodeVec(in))
	if len(out) != len(in) {
		t.Fatalf("len = %d", len(out))
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("v[%d] = %v, want %v", i, out[i], in[i])
		}
	}
}

func TestStats(t *testing.T) {
	s := testStore(t)
	mkGlyph(t, s, "g-aaaa", "decision one", "decision", []string{"x"}, []types.Ref{{Kind: "url", Target: "https://a"}})
	mkGlyph(t, s, "g-bbbb", "note one", "note", []string{"x", "y"}, nil)
	if _, err := s.CreateEdge("g-aaaa", "g-bbbb", "supports"); err != nil {
		t.Fatal(err)
	}
	st, err := s.GetStats(5)
	if err != nil {
		t.Fatal(err)
	}
	if st.Glyphs != 2 || st.Edges != 1 || st.Refs != 1 {
		t.Fatalf("stats = %+v", st)
	}
	if st.Today != 2 {
		t.Fatalf("today = %d", st.Today)
	}
	if len(st.Recent) != 2 || st.Recent[0].Type != "decision" {
		t.Fatalf("recent = %v", st.Recent)
	}
	if len(st.TopTags) == 0 || st.TopTags[0].Tag != "x" {
		t.Fatalf("tags = %v", st.TopTags)
	}
}
