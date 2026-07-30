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
	_, err = tx.Exec(`INSERT INTO glyphs (id, body, type, meta, created_at, updated_at) VALUES (?,?,?,?,?,?)`,
		g.ID, g.Body, nullable(g.Type), nullable(g.Meta), g.CreatedAt.Unix(), g.UpdatedAt.Unix())
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
	return tx.Commit()
}

// AmendGlyph applies in-place changes to a glyph and bumps updated_at.
// body/typ are applied when non-nil. Returns the updated glyph and whether
// the body changed (caller may re-embed).
func (s *Store) AmendGlyph(gid string, body, typ *string, addTags, rmTags []string, addRefs, rmRefs []types.Ref) (*types.Glyph, bool, error) {
	if !s.Exists(gid) {
		return nil, false, ErrNotFound
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	bodyChanged := false
	if body != nil {
		res, err := tx.Exec(`UPDATE glyphs SET body = ?, updated_at = ? WHERE id = ? AND body != ?`, *body, now, gid, *body)
		if err != nil {
			return nil, false, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			bodyChanged = true
		}
	}
	if typ != nil {
		if _, err := tx.Exec(`UPDATE glyphs SET type = ?, updated_at = ? WHERE id = ?`, nullable(*typ), now, gid); err != nil {
			return nil, false, err
		}
	}
	for _, t := range addTags {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO glyph_tags (glyph_id, tag) VALUES (?,?)`, gid, t); err != nil {
			return nil, false, err
		}
	}
	for _, t := range rmTags {
		if _, err := tx.Exec(`DELETE FROM glyph_tags WHERE glyph_id = ? AND tag = ?`, gid, t); err != nil {
			return nil, false, err
		}
	}
	for _, r := range addRefs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO glyph_refs (id, glyph_id, kind, target, label, created_at) VALUES (?,?,?,?,?,?)`,
			id.Edge(), gid, r.Kind, r.Target, nullable(r.Label), now); err != nil {
			return nil, false, err
		}
	}
	for _, r := range rmRefs {
		if _, err := tx.Exec(`DELETE FROM glyph_refs WHERE glyph_id = ? AND kind = ? AND target = ?`, gid, r.Kind, r.Target); err != nil {
			return nil, false, err
		}
	}
	if len(addTags)+len(rmTags)+len(addRefs)+len(rmRefs) > 0 {
		if _, err := tx.Exec(`UPDATE glyphs SET updated_at = ? WHERE id = ?`, now, gid); err != nil {
			return nil, false, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	g, err := s.GetGlyph(gid)
	return g, bodyChanged, err
}

// GetGlyph fetches one glyph with tags and refs.
func (s *Store) GetGlyph(gid string) (*types.Glyph, error) {
	g := &types.Glyph{}
	var typ, meta sql.NullString
	var created, updated int64
	err := s.db.QueryRow(`SELECT id, body, type, meta, created_at, updated_at FROM glyphs WHERE id = ?`, gid).
		Scan(&g.ID, &g.Body, &typ, &meta, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.Type, g.Meta = typ.String, meta.String
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
	for _, gid := range []string{src, dst} {
		if !s.Exists(gid) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, gid)
		}
	}
	e := &types.Edge{ID: id.Edge(), Src: src, Dst: dst, Rel: rel, CreatedAt: time.Now()}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO edges (id, src, dst, rel, created_at) VALUES (?,?,?,?,?)`,
		e.ID, e.Src, e.Dst, e.Rel, e.CreatedAt.Unix())
	return e, err
}

// ListFilter narrows ListGlyphs.
type ListFilter struct {
	Type  string
	Tag   string
	Since time.Time
	Until time.Time
	Limit int
}

// ListGlyphs returns recent glyphs (newest first) matching the filter,
// with tags and refs attached.
func (s *Store) ListGlyphs(f ListFilter) ([]*types.Glyph, int, error) {
	where, args := []string{"1=1"}, []any{}
	if f.Type != "" {
		where, args = append(where, "g.type = ?"), append(args, f.Type)
	}
	if f.Tag != "" {
		where = append(where, "g.id IN (SELECT glyph_id FROM glyph_tags WHERE tag = ?)")
		args = append(args, f.Tag)
	}
	if !f.Since.IsZero() {
		where, args = append(where, "g.created_at >= ?"), append(args, f.Since.Unix())
	}
	if !f.Until.IsZero() {
		where, args = append(where, "g.created_at < ?"), append(args, f.Until.Unix())
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM glyphs g WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT g.id, g.body, g.type, g.meta, g.created_at, g.updated_at
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

// Related returns 1-hop neighbors of gid (both directions), annotated with
// the connecting relation.
func (s *Store) Related(gid string, limit int) ([]*types.Glyph, error) {
	if !s.Exists(gid) {
		return nil, ErrNotFound
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT g.id, g.body, g.type, g.meta, g.created_at, g.updated_at, e.rel
		FROM edges e JOIN glyphs g ON g.id = CASE WHEN e.src = ? THEN e.dst ELSE e.src END
		WHERE e.src = ? OR e.dst = ?
		ORDER BY e.created_at DESC LIMIT ?`, gid, gid, gid, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var gs []*types.Glyph
	for rows.Next() {
		g := &types.Glyph{}
		var typ, meta sql.NullString
		var created, updated int64
		if err := rows.Scan(&g.ID, &g.Body, &typ, &meta, &created, &updated, &g.Rel); err != nil {
			return nil, err
		}
		g.Type, g.Meta = typ.String, meta.String
		g.CreatedAt, g.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
		gs = append(gs, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachTagsRefs(gs); err != nil {
		return nil, err
	}
	return gs, nil
}

func scanGlyphs(rows *sql.Rows) ([]*types.Glyph, error) {
	defer rows.Close()
	var gs []*types.Glyph
	for rows.Next() {
		g := &types.Glyph{}
		var typ, meta sql.NullString
		var created, updated int64
		if err := rows.Scan(&g.ID, &g.Body, &typ, &meta, &created, &updated); err != nil {
			return nil, err
		}
		g.Type, g.Meta = typ.String, meta.String
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
