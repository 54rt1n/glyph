package store

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/54rt1n/glyph/internal/types"
)

type traversalCandidate struct {
	edge      *types.Edge
	glyph     *types.Glyph
	direction types.Direction
}

// TraverseRelated unfolds the graph around root breadth-first. Direction is
// evaluated at every visited node. Each stored edge is emitted at most once;
// edges to an already discovered glyph are marked Repeat and are not expanded.
func (s *Store) TraverseRelated(root string, depth int, direction types.Direction, limit int) (*types.Traversal, error) {
	if depth < 0 {
		return nil, fmt.Errorf("depth must be at least 0")
	}
	if !types.ValidDirection(string(direction)) {
		return nil, fmt.Errorf("unknown direction %q (both, out, in)", direction)
	}
	if limit <= 0 {
		limit = 20
	}
	rootGlyph, err := s.GetGlyph(root)
	if err != nil {
		return nil, err
	}
	out := &types.Traversal{
		Root: rootGlyph, Glyphs: []*types.Glyph{rootGlyph}, Depth: depth, Direction: direction,
	}
	if depth == 0 || limit == 1 {
		out.Truncated = depth > 0 && limit == 1 && s.hasEdges(root, direction)
		return out, nil
	}

	type queued struct {
		id    string
		depth int
	}
	queue := []queued{{id: root, depth: 0}}
	seenGlyphs := map[string]bool{root: true}
	seenEdges := map[string]bool{}
	newGlyphs := make([]*types.Glyph, 0, limit-1)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= depth {
			continue
		}
		candidates, err := s.connections(current.id, direction)
		if err != nil {
			return nil, err
		}
		for _, candidate := range candidates {
			if seenEdges[candidate.edge.ID] {
				continue
			}
			seenEdges[candidate.edge.ID] = true
			nextDepth := current.depth + 1
			link := &types.TraversalLink{
				Edge: candidate.edge, From: current.id, To: candidate.glyph.ID,
				Direction: candidate.direction, Depth: nextDepth,
			}
			if seenGlyphs[candidate.glyph.ID] {
				link.Repeat = true
				out.Links = append(out.Links, link)
				continue
			}
			if len(out.Glyphs) >= limit {
				out.Truncated = true
				return finishTraversal(s, out, newGlyphs)
			}
			seenGlyphs[candidate.glyph.ID] = true
			out.Glyphs = append(out.Glyphs, candidate.glyph)
			newGlyphs = append(newGlyphs, candidate.glyph)
			out.Links = append(out.Links, link)
			queue = append(queue, queued{id: candidate.glyph.ID, depth: nextDepth})
		}
	}
	return finishTraversal(s, out, newGlyphs)
}

func finishTraversal(s *Store, traversal *types.Traversal, glyphs []*types.Glyph) (*types.Traversal, error) {
	if err := s.attachTagsRefs(glyphs); err != nil {
		return nil, err
	}
	return traversal, nil
}

func (s *Store) hasEdges(gid string, direction types.Direction) bool {
	var one int
	var err error
	switch direction {
	case types.DirectionOut:
		err = s.db.QueryRow(`SELECT 1 FROM edges WHERE src = ? LIMIT 1`, gid).Scan(&one)
	case types.DirectionIn:
		err = s.db.QueryRow(`SELECT 1 FROM edges WHERE dst = ? LIMIT 1`, gid).Scan(&one)
	default:
		err = s.db.QueryRow(`SELECT 1 FROM edges WHERE src = ? OR dst = ? LIMIT 1`, gid, gid).Scan(&one)
	}
	return err == nil
}

func (s *Store) connections(gid string, direction types.Direction) ([]traversalCandidate, error) {
	condition := `e.src = ? OR e.dst = ?`
	args := []any{gid, gid, gid}
	switch direction {
	case types.DirectionOut:
		condition = `e.src = ?`
		args = []any{gid, gid}
	case types.DirectionIn:
		condition = `e.dst = ?`
		args = []any{gid, gid}
	}
	rows, err := s.db.Query(`
		SELECT e.id, e.src, e.dst, e.rel, e.created_at,
		       g.id, g.summary, g.body, g.type, g.meta, g.starred, g.created_at, g.updated_at
		FROM edges e
		JOIN glyphs g ON g.id = CASE WHEN e.src = ? THEN e.dst ELSE e.src END
		WHERE `+condition+`
		ORDER BY e.created_at, e.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []traversalCandidate
	for rows.Next() {
		e := &types.Edge{}
		g := &types.Glyph{}
		var summary, typ, meta sql.NullString
		var edgeCreated, created, updated int64
		if err := rows.Scan(
			&e.ID, &e.Src, &e.Dst, &e.Rel, &edgeCreated,
			&g.ID, &summary, &g.Body, &typ, &meta, &g.Starred, &created, &updated,
		); err != nil {
			return nil, err
		}
		e.CreatedAt = time.Unix(edgeCreated, 0)
		g.Summary, g.Type, g.Meta = summary.String, typ.String, meta.String
		g.CreatedAt, g.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
		d := types.DirectionIn
		if e.Src == gid {
			d = types.DirectionOut
		}
		out = append(out, traversalCandidate{edge: e, glyph: g, direction: d})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.edge.CreatedAt.Equal(b.edge.CreatedAt) {
			return a.edge.CreatedAt.Before(b.edge.CreatedAt)
		}
		if a.direction != b.direction {
			return a.direction == types.DirectionOut
		}
		if a.edge.Src != b.edge.Src {
			return a.edge.Src < b.edge.Src
		}
		if a.edge.Dst != b.edge.Dst {
			return a.edge.Dst < b.edge.Dst
		}
		if a.edge.Rel != b.edge.Rel {
			return a.edge.Rel < b.edge.Rel
		}
		return a.edge.ID < b.edge.ID
	})
	return out, nil
}
