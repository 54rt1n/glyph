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
	Recent    []*types.Glyph `json:"recent"` // recent sticky pins (decisions first)
}

// GetStats aggregates counts for the context summary. recentLimit caps the
// recent sticky pins (most recent `decision` glyphs, padded with most recent
// of any type when there are too few decisions).
func (s *Store) GetStats(recentLimit int) (*Stats, error) {
	st := &Stats{}
	row := func(q string, dest *int, args ...any) error { return s.db.QueryRow(q, args...).Scan(dest) }
	if err := row(`SELECT COUNT(*) FROM glyphs`, &st.Glyphs); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM edges`, &st.Edges); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM glyph_refs`, &st.Refs); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM glyph_vecs`, &st.Vecs); err != nil {
		return nil, err
	}

	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekAgo := now.AddDate(0, 0, -7)
	if err := row(`SELECT COUNT(*) FROM glyphs WHERE created_at >= ?`, &st.Today, midnight.Unix()); err != nil {
		return nil, err
	}
	if err := row(`SELECT COUNT(*) FROM glyphs WHERE created_at >= ?`, &st.ThisWeek, weekAgo.Unix()); err != nil {
		return nil, err
	}
	var last int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(created_at), 0) FROM glyphs`).Scan(&last); err != nil {
		return nil, err
	}
	if last > 0 {
		st.LastEtch = time.Unix(last, 0)
	}

	rows, err := s.db.Query(`SELECT COALESCE(type, ''), COUNT(*) FROM glyphs GROUP BY type ORDER BY COUNT(*) DESC`)
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

	rows, err = s.db.Query(`SELECT tag, COUNT(*) FROM glyph_tags GROUP BY tag ORDER BY COUNT(*) DESC, tag LIMIT 8`)
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
		decisions, _, err := s.ListGlyphs(ListFilter{Type: "decision", Limit: recentLimit})
		if err != nil {
			return nil, err
		}
		st.Recent = decisions
		if len(st.Recent) < recentLimit {
			pad, _, err := s.ListGlyphs(ListFilter{Limit: recentLimit})
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
