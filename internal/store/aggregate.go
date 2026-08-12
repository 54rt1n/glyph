package store

import (
	"time"

	"github.com/54rt1n/glyph/internal/types"
)

// TagCount pairs a tag with its usage count.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// TypeCount pairs a glyph type with its count.
type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// Stats is the aggregate view backing `glyph context`.
type Stats struct {
	Glyphs    int            `json:"glyphs"`
	Edges     int            `json:"edges"`
	Refs      int            `json:"refs"`
	Vecs      int            `json:"vectors"`
	ByType    []TypeCount    `json:"by_type"`
	TopTags   []TagCount     `json:"top_tags"`
	Today     int            `json:"today"`
	ThisWeek  int            `json:"this_week"`
	LastEtch  time.Time      `json:"last_etch"`
	Recent    []*types.Glyph `json:"recent,omitempty"` // recent sticky pins (decisions first)
	Tag       string         `json:"tag,omitempty"`
}

// GetStats aggregates counts for the context summary. recentLimit caps the
// recent sticky pins (most recent `decision` glyphs, padded with most recent
// of any type when there are too few decisions). f.Tag (and f.Type, if set)
// scopes every count and the standing pin list.
func (s *Store) GetStats(recentLimit int, f ListFilter) (*Stats, error) {
	st := &Stats{Tag: f.Tag}
	cond, args := listWhere(ListFilter{Type: f.Type, Tag: f.Tag})
	inMatch := `SELECT g.id FROM glyphs g WHERE ` + cond

	row := func(q string, dest *int, qargs ...any) error {
		return s.db.QueryRow(q, qargs...).Scan(dest)
	}
	if err := row(`SELECT COUNT(*) FROM glyphs g WHERE `+cond, &st.Glyphs, args...); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM edges e WHERE e.src IN (`+inMatch+`) OR e.dst IN (`+inMatch+`)`,
		&st.Edges, append(append([]any{}, args...), args...)...); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM glyph_refs r WHERE r.glyph_id IN (`+inMatch+`)`, &st.Refs, args...); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM glyph_vecs v WHERE v.glyph_id IN (`+inMatch+`)`, &st.Vecs, args...); err != nil {
		return nil, err
	}

	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := now.AddDate(0, 0, -7)
	if err := row(`SELECT COUNT(*) FROM glyphs g WHERE `+cond+` AND g.created_at >= ?`,
		&st.Today, append(append([]any{}, args...), midnight.Unix())...); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM glyphs g WHERE `+cond+` AND g.created_at >= ?`,
		&st.ThisWeek, append(append([]any{}, args...), weekAgo.Unix())...); err != nil {
		return nil, err
	}
	var last int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(g.created_at), 0) FROM glyphs g WHERE `+cond, args...).Scan(&last); err != nil {
		return nil, err
	}
	if last > 0 {
		st.LastEtch = time.Unix(last, 0)
	}

	rows, err := s.db.Query(`SELECT COALESCE(g.type, ''), COUNT(*) FROM glyphs g WHERE `+cond+` GROUP BY g.type ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tc TypeCount
		if err := rows.Scan(&tc.Type, &tc.Count); err != nil {
			rows.Close()
			return nil, err
		}
		if tc.Type == "" {
			tc.Type = "(untyped)"
		}
		st.ByType = append(st.ByType, tc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tagQ := `SELECT t.tag, COUNT(*) FROM glyph_tags t WHERE t.glyph_id IN (` + inMatch + `)`
	tagArgs := append([]any{}, args...)
	if f.Tag != "" {
		tagQ += ` AND t.tag != ?`
		tagArgs = append(tagArgs, f.Tag)
	}
	tagQ += ` GROUP BY t.tag ORDER BY COUNT(*) DESC, t.tag LIMIT 8`
	rows, err = s.db.Query(tagQ, tagArgs...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			rows.Close()
			return nil, err
		}
		st.TopTags = append(st.TopTags, tc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if recentLimit > 0 {
		decFilter := ListFilter{Type: f.Type, Tag: f.Tag, Limit: recentLimit}
		if decFilter.Type == "" {
			decFilter.Type = "decision"
		}
		decisions, _, err := s.ListGlyphs(decFilter)
		if err != nil {
			return nil, err
		}
		st.Recent = decisions
		if f.Type == "" && len(st.Recent) < recentLimit {
			pad, _, err := s.ListGlyphs(ListFilter{Tag: f.Tag, Limit: recentLimit})
			if err != nil {
				return nil, err
			}
			seen := map[string]bool{}
			for _, g := range st.Recent {
				seen[g.ID] = true
			}
			for _, g := range pad {
				if len(st.Recent) >= recentLimit {
					break
				}
				if !seen[g.ID] {
					st.Recent = append(st.Recent, g)
				}
			}
		}
	}
	return st, nil
}
