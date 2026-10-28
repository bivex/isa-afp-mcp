# isa-afp-mcp

High-performance Model Context Protocol (MCP) server for the **Archive of Formal Proofs (AFP)** ([isa-afp.org](https://isa-afp.org)), designed specifically for AI Agents and LLMs doing formal verification, algorithm extraction, and mathematical reasoning in Isabelle/HOL.

Built in **Go** following **Domain-Driven Design (DDD)** and **Hexagonal Architecture (Ports and Adapters)**.

---

## ⚡ Agent-Centric Optimizations

- **Sub-Millisecond In-Memory BM25 Search**: Indexes all 1,045+ verified entries across titles, shortnames, topics, authors, and abstracts. Average query latency is **9–30 microseconds**.
- **Token-Efficient Context Windows**:
  - `get_theory_outline`: Extracts imports, datatypes, type synonyms, definitions, locales, lemmas, and theorems **without bulky proof tactic scripts**, reducing token size by **80–95%** while retaining 100% of the mathematical interface.
  - `search_entries` with default `compact` format generates concise, 1–2 sentence abstract snippets with high keyword density.
  - `get_theory_content` supports line range pagination (`start_line`, `max_lines`) to avoid context blowup.
- **On-Demand Package Streaming & Caching**: Streams `.tar.gz` packages directly from `isa-afp.org` without unpacking 70MB archives to disk. Caches downloaded archives and catalog metadata locally under `~/.cache/isa-afp-mcp`.
- **Zero-Network Instant Startup**: Loads cached catalog metadata and indexes on boot in < 15ms. Operates in full offline mode via `--offline`.

---

## 🏛️ Architecture (DDD + Hexagonal / Ports & Adapters)

```
cmd/
  isa-afp-mcp/                     # CLI entrypoint & server bootstrap
internal/
  domain/                          # Core Domain (Entities, Value Objects & Ports)
    entry/                         # Aggregate root: Entry, Topic, Author, Keyword
    theory/                        # Aggregate root: RootFile, TheoryOutline, TheoryItem
    search/                        # Value objects: Query, Match, Result
    port/
      inbound.go                   # Primary / Driving Ports (Use Cases)
      outbound.go                  # Secondary / Driven Ports (SPI)
  application/
    service/                       # Application Services orchestrating use cases
      catalog_service.go           # Catalog synchronization, topics & autocomplete
      search_service.go            # BM25 search execution
      theory_service.go            # Package downloading, ROOT & theory outline extraction
  adapter/
    driven/                        # Secondary Adapters (Infrastructure)
      afpclient/                   # HTTP client for isa-afp.org REST API & tarballs
      cache/                       # Atomic persistent disk cache (~/.cache/isa-afp-mcp)
      repository/                  # In-memory concurrent catalog repository
      searchindex/                 # High-speed BM25 inverted index & tokenizer
      theoryarchive/               # Streaming tar.gz reader & ROOT/.thy extractor
    driving/                       # Primary Adapters (Interfaces)
      mcp/                         # MCP stdio/SSE server, tool handlers & token formatters
```

---

## 🛠️ MCP Tools

| Tool | Description | Key Parameters |
|---|---|---|
| `search_entries` | Full-text BM25 search across 1045+ entries with ranking and snippet generation | `query` (req), `topic`, `author`, `year_from`, `year_to`, `limit`, `format` (`compact`\|`detailed`\|`json`) |
| `get_entry` | Get metadata, full abstract, authors, release date, and citations | `shortname` (e.g. `Formal_SSA`, `Finite_Fields`) |
| `list_theories` | List all `.thy` files and `ROOT` specifications in an entry archive | `shortname` |
| `get_theory_outline` | **LLM-optimized**: Extracts mathematical interface (types, defs, lemmas, theorems) without proof tactics | `shortname`, `theory` (e.g. `Construct_SSA`) |
| `get_theory_content` | Read raw `.thy` or `ROOT` file with line range pagination | `shortname`, `path`, `start_line`, `max_lines` |
| `get_root` | Inspect Isabelle `ROOT` dependencies, parent session, and exported theories | `shortname` |
| `list_topics` | List hierarchical AFP classifications (Computer science, Mathematics, Logic) | `filter` (optional) |
| `suggest_keywords` | Autocomplete concepts from AFP's 8,869 mathematical terms dictionary | `prefix`, `limit` |
| `get_catalog_status` | Check index health, total entries count, and cache freshness | None |
| `sync_catalog` | Force re-sync of entries, topics, and keywords from `isa-afp.org` | None |

---

## 🚀 Quick Start

### Build Binary
```bash
go build -o bin/isa-afp-mcp ./cmd/isa-afp-mcp
```

### Run Tests & Benchmarks
```bash
go test -v -race -bench=. ./...
```

### Configure in Claude Desktop / Cursor / Windsurf

Add to your MCP configuration (`claude_desktop_config.json` or `.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "isa-afp": {
      "command": "/absolute/path/to/isa-afp-mcp/bin/isa-afp-mcp",
      "args": ["--log-level=info"]
    }
  }
}
```

### CLI Options

```
Usage of bin/isa-afp-mcp:
  --cache-dir string
        Local cache directory (default: ~/.cache/isa-afp-mcp or $ISA_AFP_CACHE_DIR)
  --force-sync
        Force fresh sync from upstream isa-afp.org on startup
  --http string
        Serve HTTP SSE on address (e.g. ':8080') instead of stdio
  --log-level string
        Log level (debug, info, warn, error) (default "info")
  --offline
        Offline mode: do not contact upstream network
  --sync-interval duration
        Background catalog sync interval (default 24h0m0s, 0 to disable)
```

---

## 📄 License
MIT
