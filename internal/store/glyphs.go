package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/54rt1n/glyph/internal/id"
	"github.com/54rt1n/glyph/internal/types"
)

// ErrNotFound is returned when a glyph id does not exist.
var ErrNotFound = errors.New("glyph not found")

// Exists reports whether a glyph id is present.
func (s *Store) Exists(gid string) bool {
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM glyphs WHERE id = ?`, gid).Scan(&one)
	return err == nil
}

// CreateGlyph inserts g (id must be set) with its tags and refs.
func (s *Store) CreateGlyph(g *types.Glyph) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertGlyph(tx, g); err != nil {
		return err
	}
	return tx.Commit()
}

func insertGlyph(tx *sql.Tx, g *types.Glyph) error {
	_, err := tx.Exec(`INSERT INTO glyphs (id, summary, body, type, meta, starred, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		g.ID, nullable(g.Summary), g.Body, nullable(g.Type), nullable(g.Meta), g.Starred, g.CreatedAt.Unix(), g.UpdatedAt.Unix())
	if err != nil {
		return err
	}
	for _, t := range g.Tags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO glyph_tags (glyph_id, tag) VALUES (?,?)`, g.ID, t); err != nil {
			return err
		}
	}
	for _, r := range g.Refs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO glyph_refs (id, glyph_id, kind, target, label, created_at) VALUES (?,?,?,?,?,?)`,
			id.Edge(), g.ID, r.Kind, r.Target, nullable(r.Label), g.CreatedAt.Unix()); err != nil {
			return err
		}
	}
	return nil
}

// GlyphPatch describes an in-place glyph update. Pointer fields distinguish
// "unchanged" from setting the corresponding value to empty/false.
type GlyphPatch struct {
	Body    *string
	Summary *string
	Type    *string
	Starred *bool
	AddTags []string
	RmTags  []string
	AddRefs []types.Ref
	RmRefs  []types.Ref
}

// AmendGlyph applies an in-place patch and bumps updated_at. It reports
// whether searchable text changed so the caller can refresh the embedding.
func (s *Store) AmendGlyph(gid string, p GlyphPatch) (*types.Glyph, bool, error) {
	if !s.Exists(gid) {
		return nil, false, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	searchableChanged := false
	if p.Body != nil {
		res, err := tx.Exec(`UPDATE glyphs SET body = ?, updated_at = ? WHERE id = ? AND body != ?`, *p.Body, now, gid, *p.Body)
		if err != nil {
			return nil, false, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			searchableChanged = true
		}
	}
	if p.Summary != nil {
		res, err := tx.Exec(`UPDATE glyphs SET summary = ?, updated_at = ? WHERE id = ? AND COALESCE(summary, '') != ?`, nullable(*p.Summary), now, gid, *p.Summary)
		if err != nil {
			return nil, false, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			searchableChanged = true
		}
	}
	if p.Type != nil {
		if _, err := tx.Exec(`UPDATE glyphs SET type = ?, updated_at = ? WHERE id = ?`, nullable(*p.Type), now, gid); err != nil {
			return nil, false, err
		}
	}
	if p.Starred != nil {
		if _, err := tx.Exec(`UPDATE glyphs SET starred = ?, updated_at = ? WHERE id = ?`, *p.Starred, now, gid); err != nil {
			return nil, false, err
		}
	}
	for _, t := range p.AddTags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO glyph_tags (glyph_id, tag) VALUES (?,?)`, gid, t); err != nil {
			return nil, false, err
		}
	}
	for _, t := range p.RmTags {
		if _, err := tx.Exec(`DELETE FROM glyph_tags WHERE glyph_id = ? AND tag = ?`, gid, t); err != nil {
			return nil, false, err
		}
	}
	for _, r := range p.AddRefs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO glyph_refs (id, glyph_id, kind, target, label, created_at) VALUES (?,?,?,?,?,?)`,
			id.Edge(), gid, r.Kind, r.Target, nullable(r.Label), now); err != nil {
			return nil, false, err
		}
	}
	for _, r := range p.RmRefs {
		if _, err := tx.Exec(`DELETE FROM glyph_refs WHERE glyph_id = ? AND kind = ? AND target = ?`, gid, r.Kind, r.Target); err != nil {
			return nil, false, err
		}
	}
	if len(p.AddTags)+len(p.RmTags)+len(p.AddRefs)+len(p.RmRefs) > 0 {
		if _, err := tx.Exec(`UPDATE glyphs SET updated_at = ? WHERE id = ?`, now, gid); err != nil {
			return nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	g, err := s.GetGlyph(gid)
	return g, searchableChanged, err
}

// GetGlyph fetches one glyph with tags and refs.
func (s *Store) GetGlyph(gid string) (*types.Glyph, error) {
	g := &types.Glyph{}
	var summary, typ, meta sql.NullString
	var created, updated int64
	err := s.db.QueryRow(`SELECT id, summary, body, type, meta, starred, created_at, updated_at FROM glyphs WHERE id = ?`, gid).
		Scan(&g.ID, &summary, &g.Body, &typ, &meta, &g.Starred, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.Summary, g.Type, g.Meta = summary.String, typ.String, meta.String
	g.CreatedAt, g.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
	if err := s.attachTagsRefs([]*types.Glyph{g}); err != nil {
		return nil, err
	}
	return g, nil
}

// DeleteGlyph removes a glyph; edges/tags/refs/vecs cascade.
func (s *Store) DeleteGlyph(gid string) error {
	res, err := s.db.Exec(`DELETE FROM glyphs WHERE id = ?`, gid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateEdge links src → dst with rel. Duplicate (src,dst,rel) is a no-op.
func (s *Store) CreateEdge(src, dst, rel string) (*types.Edge, error) {
	es, err := s.CreateEdges(src, []string{dst}, rel)
	if err != nil {
		return nil, err
	}
	return es[0], nil
}

// CreateEdges links src → each dst with the same rel, atomically.
// Duplicate (src,dst,rel) rows are ignored. dsts are de-duplicated in order.
func (s *Store) CreateEdges(src string, dsts []string, rel string) ([]*types.Edge, error) {
	if src == "" {
		return nil, fmt.Errorf("empty source id")
	}
	seen := map[string]bool{}
	uniq := make([]string, 0, len(dsts))
	for _, d := range dsts {
		if d == "" {
			return nil, fmt.Errorf("empty destination id")
		}
		if !seen[d] {
			seen[d] = true
			uniq = append(uniq, d)
		}
	}
	if len(uniq) == 0 {
		return nil, fmt.Errorf("no destinations")
	}
	for _, gid := range append([]string{src}, uniq...) {
		if !s.Exists(gid) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, gid)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now()
	out := make([]*types.Edge, 0, len(uniq))
	for _, dst := range uniq {
		e := &types.Edge{ID: id.Edge(), Src: src, Dst: dst, Rel: rel, CreatedAt: now}
		if err := insertEdge(tx, e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func insertEdge(tx *sql.Tx, e *types.Edge) error {
	_, err := tx.Exec(`INSERT OR IGNORE INTO edges (id, src, dst, rel, created_at) VALUES (?,?,?,?,?)`,
		e.ID, e.Src, e.Dst, e.Rel, e.CreatedAt.Unix())
	return err
}

// ApplyGraph inserts glyphs then edges in one transaction. Glyph ids must
// already be set; edge ends must be in the batch or already in the store.
func (s *Store) ApplyGraph(gs []*types.Glyph, edges []*types.Edge) error {
	known := map[string]bool{}
	for _, g := range gs {
		known[g.ID] = true
	}
	for _, e := range edges {
		for _, gid := range []string{e.Src, e.Dst} {
			if !known[gid] && !s.Exists(gid) {
				return fmt.Errorf("%w: %s", ErrNotFound, gid)
			}
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, g := range gs {
		if err := insertGlyph(tx, g); err != nil {
			return err
		}
	}
	for _, e := range edges {
		if err := insertEdge(tx, e); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListFilter narrows ListGlyphs.
type ListFilter struct {
	Type      string
	Tag       string
	Starred   bool
	Unstarred bool
	Since     time.Time
	Until     time.Time
	Limit     int
}

// listWhere builds a glyphs-aliased-as-g predicate from f.
func listWhere(f ListFilter) (string, []any) {
	where, args := []string{"1=1"}, []any{}
	if f.Type != "" {
		where, args = append(where, "g.type = ?"), append(args, f.Type)
	}
	if f.Tag != "" {
		where = append(where, "g.id IN (SELECT glyph_id FROM glyph_tags WHERE tag = ?)")
		args = append(args, f.Tag)
	}
	if f.Starred {
		where = append(where, "g.starred = 1")
	}
	if f.Unstarred {
		where = append(where, "g.starred = 0")
	}
	if !f.Since.IsZero() {
		where, args = append(where, "g.created_at >= ?"), append(args, f.Since.Unix())
	}
	if !f.Until.IsZero() {
		where, args = append(where, "g.created_at < ?"), append(args, f.Until.Unix())
	}
	return strings.Join(where, " AND "), args
}

// ListGlyphs returns recent glyphs (newest first) matching the filter,
// with tags and refs attached.
func (s *Store) ListGlyphs(f ListFilter) ([]*types.Glyph, int, error) {
	cond, args := listWhere(f)

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM glyphs g WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT g.id, g.summary, g.body, g.type, g.meta, g.starred, g.created_at, g.updated_at
		FROM glyphs g WHERE `+cond+` ORDER BY g.created_at DESC, g.id LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, 0, err
	}
	gs, err := scanGlyphs(rows)
	if err != nil {
		return nil, 0, err
	}
	if err := s.attachTagsRefs(gs); err != nil {
		return nil, 0, err
	}
	return gs, total, nil
}

// Related returns 1-hop neighbors of gid in both directions. It is retained
// as a thin compatibility helper; TraverseRelated exposes edge direction and
// recursive traversal.
func (s *Store) Related(gid string, limit int) ([]*types.Glyph, error) {
	if limit <= 0 {
		limit = 20
	}
	t, err := s.TraverseRelated(gid, 1, types.DirectionBoth, limit+1)
	if err != nil {
		return nil, err
	}
	gs := append([]*types.Glyph(nil), t.Glyphs[1:]...)
	rels := make(map[string]string, len(t.Links))
	for _, link := range t.Links {
		if !link.Repeat {
			rels[link.To] = link.Edge.Rel
		}
	}
	for _, g := range gs {
		g.Rel = rels[g.ID]
	}
	if limit > 0 && len(gs) > limit {
		gs = gs[:limit]
	}
	return gs, nil
}

func scanGlyphs(rows *sql.Rows) ([]*types.Glyph, error) {
	defer rows.Close()
	var gs []*types.Glyph
	for rows.Next() {
		g := &types.Glyph{}
		var summary, typ, meta sql.NullString
		var created, updated int64
		if err := rows.Scan(&g.ID, &summary, &g.Body, &typ, &meta, &g.Starred, &created, &updated); err != nil {
			return nil, err
		}
		g.Summary, g.Type, g.Meta = summary.String, typ.String, meta.String
		g.CreatedAt, g.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
		gs = append(gs, g)
	}
	return gs, rows.Err()
}

// attachTagsRefs fills Tags and Refs on the given glyphs.
func (s *Store) attachTagsRefs(gs []*types.Glyph) error {
	byID := map[string]*types.Glyph{}
	ids := make([]any, 0, len(gs))
	ph := make([]string, 0, len(gs))
	for _, g := range gs {
		byID[g.ID] = g
		ids = append(ids, g.ID)
		ph = append(ph, "?")
	}
	if len(ids) == 0 {
		return nil
	}
	in := strings.Join(ph, ",")
	rows, err := s.db.Query(`SELECT glyph_id, tag FROM glyph_tags WHERE glyph_id IN (`+in+`) ORDER BY tag`, ids...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var gid, tag string
		if err := rows.Scan(&gid, &tag); err != nil {
			rows.Close()
			return err
		}
		byID[gid].Tags = append(byID[gid].Tags, tag)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	rows, err = s.db.Query(`SELECT glyph_id, kind, target, COALESCE(label,'') FROM glyph_refs WHERE glyph_id IN (`+in+`) ORDER BY created_at`, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var gid string
		var r types.Ref
		if err := rows.Scan(&gid, &r.Kind, &r.Target, &r.Label); err != nil {
			return err
		}
		byID[gid].Refs = append(byID[gid].Refs, r)
	}
	return rows.Err()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
