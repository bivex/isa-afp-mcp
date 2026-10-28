package search

import (
	"strings"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
)

// Match holds a matched entry along with relevance score and snippet.
type Match struct {
	Entry   entry.Entry `json:"entry"`
	Score   float64     `json:"score"`
	Snippet string      `json:"snippet"`
}

// Result contains paginated search matches and execution metrics.
type Result struct {
	TotalHits       int     `json:"total_hits"`
	Offset          int     `json:"offset"`
	Limit           int     `json:"limit"`
	Matches         []Match `json:"matches"`
	TookMicrosecond int64   `json:"took_us"`
}

// CompactSummary returns a token-efficient string representation designed specifically
// for LLM context windows (vital info only: id, title, year, topics, snippet).
func (r Result) CompactSummary() string {
	if len(r.Matches) == 0 {
		return "No matching AFP entries found."
	}
	var sb strings.Builder
	for i, m := range r.Matches {
		e := m.Entry
		sb.WriteString(e.Shortname)
		sb.WriteString(" (")
		sb.WriteString(e.Year)
		sb.WriteString("): ")
		sb.WriteString(e.Title)
		sb.WriteString("\n  Topics: ")
		sb.WriteString(strings.Join(e.Topics, ", "))
		if m.Snippet != "" {
			sb.WriteString("\n  Abstract: ")
			sb.WriteString(m.Snippet)
		}
		if i < len(r.Matches)-1 {
			sb.WriteString("\n---\n")
		}
	}
	return sb.String()
}
