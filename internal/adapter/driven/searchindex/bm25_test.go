package searchindex

import (
	"fmt"
	"testing"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
)

func sampleEntries() []entry.Entry {
	return []entry.Entry{
		{
			Shortname: "Formal_SSA",
			Title:     "Verified Construction of Static Single Assignment Form",
			Authors:   []string{"Denis Lohner", "Sebastian Ullrich"},
			Date:      "2016-02-05",
			Year:      "2016",
			Topics:    []string{"Computer science/Semantics and reasoning"},
			Abstract:  "We define a simple imperative language with unstructured control flow and verify SSA construction.",
		},
		{
			Shortname: "Dijkstra_Shortest_Path",
			Title:     "Dijkstra's Shortest Path Algorithm",
			Authors:   []string{"Benedikt Nordhoff", "Peter Lammich"},
			Date:      "2012-01-30",
			Year:      "2012",
			Topics:    []string{"Computer science/Algorithms/Graph"},
			Abstract:  "We formalize Dijkstra's algorithm for the single-source shortest path problem on graphs with non-negative edge weights.",
		},
		{
			Shortname: "Finite_Fields",
			Title:     "Finite Fields",
			Authors:   []string{"Jose Divasón"},
			Date:      "2018-06-05",
			Year:      "2018",
			Topics:    []string{"Mathematics/Algebra"},
			Abstract:  "This work contains a formalization of finite fields (Galois fields) in Isabelle/HOL.",
		},
		{
			Shortname: "Red_Black_Trees",
			Title:     "Red-Black Trees",
			Authors:   []string{"Tobias Nipkow"},
			Date:      "2004-04-20",
			Year:      "2004",
			Topics:    []string{"Computer science/Data structures"},
			Abstract:  "A verified functional implementation of red-black search trees.",
		},
	}
}

func TestBM25Index_SearchRanking(t *testing.T) {
	idx := NewBM25Index()
	entries := sampleEntries()

	if err := idx.Build(entries); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	tests := []struct {
		name         string
		query        search.Query
		wantTopEntry string
		minTotalHits int
	}{
		{
			name:         "Direct exact shortname match",
			query:        search.Query{Text: "Formal_SSA"},
			wantTopEntry: "Formal_SSA",
			minTotalHits: 1,
		},
		{
			name:         "Algorithm name query in title",
			query:        search.Query{Text: "dijkstra shortest path"},
			wantTopEntry: "Dijkstra_Shortest_Path",
			minTotalHits: 1,
		},
		{
			name:         "Data structure concept",
			query:        search.Query{Text: "red black trees"},
			wantTopEntry: "Red_Black_Trees",
			minTotalHits: 1,
		},
		{
			name:         "Mathematical query with Galois fields in abstract",
			query:        search.Query{Text: "galois fields"},
			wantTopEntry: "Finite_Fields",
			minTotalHits: 1,
		},
		{
			name:         "Topic filtering",
			query:        search.Query{Topic: "Algebra"},
			wantTopEntry: "Finite_Fields",
			minTotalHits: 1,
		},
		{
			name:         "Year range filtering",
			query:        search.Query{Text: "shortest", YearFrom: 2010, YearTo: 2015},
			wantTopEntry: "Dijkstra_Shortest_Path",
			minTotalHits: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := idx.Search(tt.query)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}
			if res.TotalHits < tt.minTotalHits {
				t.Errorf("Expected at least %d hits, got %d", tt.minTotalHits, res.TotalHits)
			}
			if len(res.Matches) > 0 && res.Matches[0].Entry.Shortname != tt.wantTopEntry {
				t.Errorf("Top entry mismatch: got %s, want %s", res.Matches[0].Entry.Shortname, tt.wantTopEntry)
			}
			if res.TookMicrosecond < 0 {
				t.Errorf("Execution time must be >= 0")
			}
		})
	}
}

func BenchmarkBM25Search(b *testing.B) {
	// Synthesize 1500 entries (larger than real AFP catalog)
	entries := make([]entry.Entry, 1500)
	base := sampleEntries()
	for i := range 1500 {
		tpl := base[i%len(base)]
		tpl.Shortname = fmt.Sprintf("%s_%d", tpl.Shortname, i)
		entries[i] = tpl
	}

	idx := NewBM25Index()
	if err := idx.Build(entries); err != nil {
		b.Fatalf("Build failed: %v", err)
	}

	q := search.Query{
		Text:  "dijkstra shortest path",
		Limit: 10,
	}

	b.ResetTimer()
	for range b.N {
		_, _ = idx.Search(q)
	}
}
