# Glyph

Local graph memory for AI agents — CLI-first, no external services required.

Glyph lets agents **store**, **link**, and **retrieve** project knowledge as a durable graph.
Each unit of knowledge is a **glyph** (a short-id node like `g-a1b2`). Edges connect glyphs;
**refs** point at the outside world; **tags** classify. Everything lives in a single SQLite
file under `.glyph/` — no daemon, no service, one process per command.

It is a formalized, addressable, budgeted evolution of the `NOTES_yyyy-mm-dd.md` habit:
agents keep writing short updates; Glyph clocks them, links them, and discloses them
thinly into context via **facets** (progressive disclosure).

See [OVERVIEW.md](OVERVIEW.md) for the full design.

## Install

```bash
go install github.com/54rt1n/glyph/cmd/glyph@latest
```

Or build from source:

```bash
git clone https://github.com/54rt1n/glyph
cd glyph
make build          # → bin/glyph
```

## Quickstart

```bash
glyph init                      # create .glyph/ in your project
glyph skill                     # print the agent usage skill (learn the tool once)

# write (timestamps are automatic — no date ceremony)
glyph etch --type note "Tried X; Y failed because Z"
glyph etch --type decision --tag retrieval \
  --ref url:https://arxiv.org/abs/2501.13956 \
  "Pins by default; deepen with show"

# connect and revise
glyph link g-a1b2 g-c3d4 --as supports
glyph amend g-a1b2 "Updated conclusion" --tag +final --tag -draft

# retrieve wide and cheap (one-line pins), deepen on purpose
glyph ask "what did we decide about retrieval?" --type decision
glyph list --type note --today
glyph show g-a1b2
glyph show g-a1b2 --facet neighborhood
glyph context                   # session summary: counts, types, tags, activity
```

Add `--json` to any command (or set `GLYPH_FORMAT=json`) for machine-readable output.

## Commands

| Command | Role |
|---------|------|
| `glyph init` | Create the `.glyph/` store for this project |
| `glyph etch` | Store text as a glyph (`--type`, `--tag`, `--ref kind:target`) |
| `glyph amend` | Revise a glyph in place (`--tag +x/-x`, `--ref +k:t/-k:t`) |
| `glyph ask` | Query memory — hybrid BM25 + vectors, thin pins by default |
| `glyph show` | One glyph in full (`--facet neighborhood` for 1-hop) |
| `glyph list` | Recent or filtered glyphs (`--type`, `--tag`, `--today`, `--since`) |
| `glyph link` | Connect two glyphs with a typed edge (`--as supports`) |
| `glyph related` | 1-hop neighbors as pins |
| `glyph forget` | Delete a glyph (edges/tags/refs/vectors cascade) |
| `glyph context` | Store summary: counts, top tags, activity, recent pins |
| `glyph skill` | Print the agent usage skill (drop into your agent's skill system) |
| `glyph model` | Select/download the embedding model |

## Semantic search (optional)

`ask` always works: BM25 full-text search is the floor, with exact tag matches blended in.
To add semantic ranking, download the local embedding model once (~119 MB, runs on CPU,
no API key, no service):

```bash
glyph model get default
glyph model use default
```

New and amended glyphs are embedded automatically; `ask` blends vector similarity into
its ranking via reciprocal rank fusion. Remove the model at any time — everything
degrades cleanly back to keyword search.

## Design in one breath

CLI is the product · twelve commands · SQLite + WAL, safe for concurrent agents ·
write fat, disclose thin (facets: `pin → card → body → neighborhood`) · tags classify,
refs cite, links connect · time is implicit on write, a filter on read · no daemon,
no Cypher, no MCP required.

## License

Apache License 2.0 — see [LICENSE](LICENSE).

Copyright © 2026 Martin Bukowski ([@54rt1n](https://github.com/54rt1n))
