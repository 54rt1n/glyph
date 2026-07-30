package store

import (
	"encoding/binary"
	"math"
	"sort"
	"strings"
)

// Embedder is the minimal encoding surface search needs; satisfied by
// internal/embed implementations. A nil Embedder means BM25-only.
type Embedder interface {
	Encode(text string) ([]float32, error)
	Model() string
}

// Hit is one search result: an id with a fused score.
type Hit struct {
	ID    string
	Score float64
}

const rrfK = 60 // standard reciprocal-rank-fusion constant

// Search ranks glyphs for the query: BM25 over FTS5 always, blended with
// cosine similarity over stored vectors via reciprocal rank fusion when an
// embedder is available and vectors exist. filter narrows candidates by
// type/tag/time before the limit is applied (its Limit field is ignored).
func (s *Store) Search(query string, limit int, emb Embedder, filter ListFilter) ([]Hit, error) {
	if limit <= 0 {
		limit = 10
	}
	pool := limit * 5
	if pool < 50 {
		pool = 50
	}

	lexical, err := s.searchFTS(query, pool)
	if err != nil {
		return nil, err
	}
	// Tag hits rank after body hits: exact tag matches on query tokens keep
	// tag-only vocabulary (e.g. `--tag retrieval`) from being a lexical miss.
	tagHits, err := s.searchTags(query, pool)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, gid := range lexical {
		seen[gid] = true
	}
	for _, gid := range tagHits {
		if !seen[gid] {
			lexical = append(lexical, gid)
		}
	}

	var semantic []string
	if emb != nil {
		if qv, err := emb.Encode(query); err == nil && qv != nil {
			semantic, err = s.searchVec(qv, emb.Model(), pool)
			if err != nil {
				return nil, err
			}
		}
	}

	fused := map[string]float64{}
	for rank, gid := range lexical {
		fused[gid] += 1.0 / float64(rrfK+rank+1)
	}
	for rank, gid := range semantic {
		fused[gid] += 1.0 / float64(rrfK+rank+1)
	}
	hits := make([]Hit, 0, len(fused))
	for gid, score := range fused {
		hits = append(hits, Hit{ID: gid, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})
	hits, err = s.filterHits(hits, filter)
	if err != nil {
		return nil, err
	}
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}

// filterHits keeps only hits whose glyphs satisfy the type/tag/time filter.
func (s *Store) filterHits(hits []Hit, f ListFilter) ([]Hit, error) {
	if f.Type == "" && f.Tag == "" && f.Since.IsZero() && f.Until.IsZero() || len(hits) == 0 {
		return hits, nil
	}
	where, args := []string{}, []any{}
	if f.Type != "" {
		where, args = append(where, "type = ?"), append(args, f.Type)
	}
	if f.Tag != "" {
		where = append(where, "id IN (SELECT glyph_id FROM glyph_tags WHERE tag = ?)")
		args = append(args, f.Tag)
	}
	if !f.Since.IsZero() {
		where, args = append(where, "created_at >= ?"), append(args, f.Since.Unix())
	}
	if !f.Until.IsZero() {
		where, args = append(where, "created_at < ?"), append(args, f.Until.Unix())
	}
	ph := make([]string, len(hits))
	for i, h := range hits {
		ph[i] = "?"
		args = append(args, h.ID)
	}
	rows, err := s.db.Query(`SELECT id FROM glyphs WHERE `+strings.Join(where, " AND ")+
		` AND id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keep := map[string]bool{}
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return nil, err
		}
		keep[gid] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := hits[:0]
	for _, h := range hits {
		if keep[h.ID] {
			out = append(out, h)
		}
	}
	return out, nil
}

// searchFTS returns glyph ids ranked by BM25. The query is tokenized and
// OR-joined so natural-language questions favor recall; BM25 handles rank.
func (s *Store) searchFTS(query string, limit int) ([]string, error) {
	match := ftsQuery(query)
	if match == "" {
		return nil, nil
	}
	rows, err := s.db.Query(`
		SELECT g.id FROM glyphs_fts f JOIN glyphs g ON g.rowid = f.rowid
		WHERE glyphs_fts MATCH ? ORDER BY bm25(glyphs_fts) LIMIT ?`, match, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return nil, err
		}
		ids = append(ids, gid)
	}
	return ids, rows.Err()
}

// searchTags returns ids of glyphs whose tags exactly match any query token
// (case-insensitive), most recent first.
func (s *Store) searchTags(query string, limit int) ([]string, error) {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil, nil
	}
	ph := make([]string, len(tokens))
	args := make([]any, len(tokens))
	for i, tok := range tokens {
		ph[i] = "?"
		args[i] = strings.ToLower(tok)
	}
	rows, err := s.db.Query(`
		SELECT DISTINCT t.glyph_id FROM glyph_tags t JOIN glyphs g ON g.id = t.glyph_id
		WHERE lower(t.tag) IN (`+strings.Join(ph, ",")+`)
		ORDER BY g.created_at DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return nil, err
		}
		ids = append(ids, gid)
	}
	return ids, rows.Err()
}

func tokenize(q string) []string {
	return strings.FieldsFunc(q, func(r rune) bool {
		return !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9')
	})
}

// ftsQuery turns free text into a safe FTS5 OR-query of quoted tokens.
func ftsQuery(q string) string {
	fields := tokenize(q)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		terms = append(terms, `"`+f+`"`)
	}
	return strings.Join(terms, " OR ")
}

// searchVec brute-force ranks stored vectors for the model by cosine
// similarity against qv.
func (s *Store) searchVec(qv []float32, model string, limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT glyph_id, vec FROM glyph_vecs WHERE model = ?`, model)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type scored struct {
		id  string
		sim float64
	}
	var all []scored
	for rows.Next() {
		var gid string
		var blob []byte
		if err := rows.Scan(&gid, &blob); err != nil {
			return nil, err
		}
		v := DecodeVec(blob)
		if len(v) != len(qv) {
			continue
		}
		all = append(all, scored{gid, cosine(qv, v)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].sim > all[j].sim })
	if len(all) > limit {
		all = all[:limit]
	}
	ids := make([]string, len(all))
	for i, sc := range all {
		ids[i] = sc.id
	}
	return ids, nil
}

// SetVec stores (replacing) the embedding for a glyph.
func (s *Store) SetVec(gid, model string, vec []float32) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO glyph_vecs (glyph_id, model, dims, vec) VALUES (?,?,?,?)`,
		gid, model, len(vec), EncodeVec(vec))
	return err
}

// ClearVecs drops all stored vectors (used when switching models).
func (s *Store) ClearVecs() error {
	_, err := s.db.Exec(`DELETE FROM glyph_vecs`)
	return err
}

// VecCount returns how many glyphs have stored vectors.
func (s *Store) VecCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM glyph_vecs`).Scan(&n)
	return n, err
}

// EncodeVec packs a float32 slice little-endian.
func EncodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

// DecodeVec unpacks a little-endian float32 blob.
func DecodeVec(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
