package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/54rt1n/glyph/internal/facet"
	"github.com/54rt1n/glyph/internal/id"
	"github.com/54rt1n/glyph/internal/types"
)

// graphFile is the --graph payload: etch nodes, then link edges.
type graphFile struct {
	Etch []graphEtch `json:"etch"`
	Link []graphLink `json:"link"`
}

type graphEtch struct {
	ID   string    `json:"id"`
	Type string    `json:"type"`
	Tags []string  `json:"tags"`
	Refs graphRefs `json:"refs"`
	Body string    `json:"body"`
}

type graphLink struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
	As  string `json:"as"`
	Rel string `json:"rel"`
}

func (l graphLink) rel() string {
	if l.As != "" {
		return l.As
	}
	if l.Rel != "" {
		return l.Rel
	}
	return "related"
}

// graphRefs accepts ["kind:target", ...] or [{"kind":"...","target":"..."}, ...].
type graphRefs []types.Ref

func (r *graphRefs) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var strs []string
	if err := json.Unmarshal(b, &strs); err == nil {
		out := make([]types.Ref, 0, len(strs))
		for _, s := range strs {
			ref, err := parseRef(s)
			if err != nil {
				return err
			}
			out = append(out, ref)
		}
		*r = out
		return nil
	}
	var objs []types.Ref
	if err := json.Unmarshal(b, &objs); err != nil {
		return fmt.Errorf("refs: want [\"kind:target\"] or [{\"kind\",\"target\"}]")
	}
	for _, ref := range objs {
		if ref.Kind == "" || ref.Target == "" {
			return fmt.Errorf("refs: kind and target are required")
		}
		if ref.Kind == "glyph" {
			return fmt.Errorf("refs don't point at glyphs — use link")
		}
	}
	*r = objs
	return nil
}

type resolvedGraph struct {
	Glyphs  []*types.Glyph
	Edges   []*types.Edge
	Aliases map[string]string // caller id → real g-xxxx
}

func parseGraph(data []byte) (graphFile, error) {
	var g graphFile
	if err := json.Unmarshal(data, &g); err != nil {
		return g, fmt.Errorf("graph json: %w", err)
	}
	if len(g.Etch) == 0 && len(g.Link) == 0 {
		return g, fmt.Errorf("graph is empty: need etch and/or link")
	}
	for i, e := range g.Etch {
		if strings.TrimSpace(e.Body) == "" {
			return g, fmt.Errorf("etch[%d]: empty body", i)
		}
	}
	for i, l := range g.Link {
		if l.Src == "" || l.Dst == "" {
			return g, fmt.Errorf("link[%d]: src and dst are required", i)
		}
	}
	return g, nil
}

func resolveGraph(g graphFile, exists func(string) bool) (*resolvedGraph, error) {
	now := time.Now()
	taken := map[string]bool{}
	existsAll := func(gid string) bool { return taken[gid] || exists(gid) }
	out := &resolvedGraph{Aliases: map[string]string{}}

	for i, e := range g.Etch {
		alias := strings.TrimSpace(e.ID)
		if alias != "" {
			if _, dup := out.Aliases[alias]; dup {
				return nil, fmt.Errorf("etch[%d]: duplicate id %q", i, alias)
			}
		}
		var real string
		switch {
		case alias != "" && id.Valid(alias):
			if exists(alias) {
				return nil, fmt.Errorf("etch[%d]: %s already exists — amend it", i, alias)
			}
			real = alias
		default:
			real = id.Generate(existsAll)
		}
		taken[real] = true
		if alias != "" {
			out.Aliases[alias] = real
		}
		out.Aliases[real] = real
		out.Glyphs = append(out.Glyphs, &types.Glyph{
			ID:        real,
			Body:      strings.TrimSpace(e.Body),
			Type:      e.Type,
			Tags:      e.Tags,
			Refs:      []types.Ref(e.Refs),
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	for i, l := range g.Link {
		src, err := resolveEnd(l.Src, out.Aliases, exists)
		if err != nil {
			return nil, fmt.Errorf("link[%d] src: %w", i, err)
		}
		dst, err := resolveEnd(l.Dst, out.Aliases, exists)
		if err != nil {
			return nil, fmt.Errorf("link[%d] dst: %w", i, err)
		}
		out.Edges = append(out.Edges, &types.Edge{
			ID:        id.Edge(),
			Src:       src,
			Dst:       dst,
			Rel:       l.rel(),
			CreatedAt: now,
		})
	}
	return out, nil
}

func resolveEnd(name string, aliases map[string]string, exists func(string) bool) (string, error) {
	if real, ok := aliases[name]; ok {
		return real, nil
	}
	if exists(name) {
		return name, nil
	}
	return "", fmt.Errorf("%q is not in the etch list and not in the store", name)
}

func runEtchGraph(path string) error {
	data, err := readGraphFile(path)
	if err != nil {
		return err
	}
	parsed, err := parseGraph(data)
	if err != nil {
		return err
	}
	proj, st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	resolved, err := resolveGraph(parsed, st.Exists)
	if err != nil {
		return err
	}
	if err := st.ApplyGraph(resolved.Glyphs, resolved.Edges); err != nil {
		return err
	}
	emb := loadEmbedder(proj)
	for _, g := range resolved.Glyphs {
		embedGlyph(st, emb, g.ID, g.Body)
	}
	if jsonOut() {
		return emitJSON(graphAck(resolved))
	}
	printGraphAck(resolved)
	return nil
}

func readGraphFile(path string) ([]byte, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(string(b))) == 0 {
			return nil, fmt.Errorf("empty graph on stdin")
		}
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func graphAck(r *resolvedGraph) map[string]any {
	glyphs := make([]any, 0, len(r.Glyphs))
	rev := map[string]string{}
	for alias, real := range r.Aliases {
		if alias != real {
			rev[real] = alias
		}
	}
	for _, g := range r.Glyphs {
		ack := writeAck(g)
		if alias, ok := rev[g.ID]; ok {
			ack["alias"] = alias
		}
		glyphs = append(glyphs, ack)
	}
	links := make([]map[string]string, 0, len(r.Edges))
	for _, e := range r.Edges {
		links = append(links, map[string]string{"src": e.Src, "dst": e.Dst, "rel": e.Rel})
	}
	return map[string]any{"glyphs": glyphs, "links": links}
}

func printGraphAck(r *resolvedGraph) {
	for _, g := range r.Glyphs {
		fmt.Println(facet.Pin(g))
	}
	for _, e := range r.Edges {
		fmt.Printf("%s -%s-> %s\n", e.Src, e.Rel, e.Dst)
	}
}
