package cli

import (
	"strings"
	"testing"

	"github.com/54rt1n/glyph/internal/id"
)

func TestParseGraph(t *testing.T) {
	g, err := parseGraph([]byte(`{
		"etch": [
			{"id": "dec", "type": "decision", "tags": ["runtime"], "refs": ["path:foo.go"], "body": "Split it"},
			{"id": "n1", "type": "note", "body": "PR1", "refs": [{"kind": "url", "target": "https://x"}]}
		],
		"link": [{"src": "dec", "dst": "n1", "as": "supports"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Etch) != 2 || g.Etch[0].Type != "decision" || len(g.Etch[0].Tags) != 1 {
		t.Fatalf("etch = %+v", g.Etch)
	}
	if len(g.Etch[0].Refs) != 1 || g.Etch[0].Refs[0].Kind != "path" {
		t.Fatalf("string refs = %+v", g.Etch[0].Refs)
	}
	if len(g.Etch[1].Refs) != 1 || g.Etch[1].Refs[0].Target != "https://x" {
		t.Fatalf("object refs = %+v", g.Etch[1].Refs)
	}
	if g.Link[0].rel() != "supports" {
		t.Fatalf("rel = %q", g.Link[0].rel())
	}

	if _, err := parseGraph([]byte(`{}`)); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("empty graph err = %v", err)
	}
	if _, err := parseGraph([]byte(`{"etch":[{"body":"  "}]}`)); err == nil || !strings.Contains(err.Error(), "empty body") {
		t.Fatalf("blank body err = %v", err)
	}
	if _, err := parseGraph([]byte(`{"link":[{"src":"a"}]}`)); err == nil || !strings.Contains(err.Error(), "src and dst") {
		t.Fatalf("partial link err = %v", err)
	}
}

func TestResolveGraphAliases(t *testing.T) {
	parsed, err := parseGraph([]byte(`{
		"etch": [
			{"id": "dec", "type": "decision", "body": "Standing decision"},
			{"id": "n1", "body": "note one"}
		],
		"link": [
			{"src": "dec", "dst": "n1", "as": "supports"},
			{"src": "dec", "dst": "g-abcd", "rel": "related"}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	exists := func(gid string) bool { return gid == "g-abcd" }
	got, err := resolveGraph(parsed, exists)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Glyphs) != 2 || len(got.Edges) != 2 {
		t.Fatalf("resolved %d glyphs %d edges", len(got.Glyphs), len(got.Edges))
	}
	dec := got.Aliases["dec"]
	n1 := got.Aliases["n1"]
	if !id.Valid(dec) || !id.Valid(n1) || dec == n1 {
		t.Fatalf("aliases = %v", got.Aliases)
	}
	if got.Edges[0].Src != dec || got.Edges[0].Dst != n1 || got.Edges[0].Rel != "supports" {
		t.Fatalf("edge0 = %+v", got.Edges[0])
	}
	if got.Edges[1].Dst != "g-abcd" || got.Edges[1].Rel != "related" {
		t.Fatalf("edge1 = %+v", got.Edges[1])
	}

	if _, err := resolveGraph(graphFile{
		Etch: []graphEtch{{ID: "a", Body: "one"}, {ID: "a", Body: "two"}},
	}, exists); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("dup alias err = %v", err)
	}
	if _, err := resolveGraph(graphFile{
		Link: []graphLink{{Src: "missing", Dst: "g-abcd"}},
	}, exists); err == nil || !strings.Contains(err.Error(), "not in the etch list") {
		t.Fatalf("unknown src err = %v", err)
	}
	if _, err := resolveGraph(graphFile{
		Etch: []graphEtch{{ID: "g-abcd", Body: "clash"}},
	}, exists); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing id err = %v", err)
	}
}

func TestWriteAck(t *testing.T) {
	ack := writeAck(sampleGlyph())
	if ack["id"] != "g-a1b2" {
		t.Fatalf("id = %v", ack["id"])
	}
	if ack["type"] != "decision" {
		t.Fatalf("type = %v", ack["type"])
	}
	tags, _ := ack["tags"].([]string)
	if len(tags) != 2 {
		t.Fatalf("tags = %v", ack["tags"])
	}
	line, _ := ack["line"].(string)
	if line == "" || strings.Contains(line, "Second") {
		t.Fatalf("line = %q", line)
	}
}

func TestAppendBody(t *testing.T) {
	got := appendBody("Started the split.\n", "COMPLETE: shipped")
	want := "Started the split.\n\nCOMPLETE: shipped"
	if got != want {
		t.Fatalf("got %q", got)
	}
}
