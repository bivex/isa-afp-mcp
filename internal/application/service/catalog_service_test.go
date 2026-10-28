package service

import (
	"context"
	"io"
	"testing"

	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/repository"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/searchindex"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
)

type mockAFPClient struct {
	entries  []entry.Entry
	topics   []entry.Topic
	authors  []entry.Author
	keywords []entry.Keyword
}

var _ port.AFPClient = (*mockAFPClient)(nil)

func (m *mockAFPClient) FetchEntries(ctx context.Context) ([]entry.Entry, error) {
	return m.entries, nil
}
func (m *mockAFPClient) FetchTopics(ctx context.Context) ([]entry.Topic, error) {
	return m.topics, nil
}
func (m *mockAFPClient) FetchAuthors(ctx context.Context) ([]entry.Author, error) {
	return m.authors, nil
}
func (m *mockAFPClient) FetchKeywords(ctx context.Context) ([]entry.Keyword, error) {
	return m.keywords, nil
}
func (m *mockAFPClient) DownloadPackageArchive(ctx context.Context, entryShortname string) (io.ReadCloser, error) {
	return nil, nil
}

func TestCatalogService(t *testing.T) {
	mockClient := &mockAFPClient{
		entries: []entry.Entry{
			{
				Shortname: "Formal_SSA",
				Title:     "Verified Construction of SSA Form",
				Authors:   []string{"Denis Lohner"},
				Date:      "2016-02-05",
				Year:      "2016",
				Topics:    []string{"Computer science/Semantics"},
				Abstract:  "We formalize SSA construction.",
			},
			{
				Shortname: "Finite_Fields",
				Title:     "Finite Fields",
				Authors:   []string{"Jose Divasón"},
				Date:      "2018-06-05",
				Year:      "2018",
				Topics:    []string{"Mathematics/Algebra"},
				Abstract:  "Formalization of finite fields.",
			},
		},
		topics: []entry.Topic{
			{Name: "Computer science/Semantics", Link: "../topics/computer-science/semantics/"},
			{Name: "Mathematics/Algebra", Link: "../topics/mathematics/algebra/"},
		},
		authors: []entry.Author{
			{ID: "lohner", Name: "Denis Lohner"},
		},
		keywords: []entry.Keyword{
			{Keyword: "galois field"},
			{Keyword: "graph reachability"},
			{Keyword: "group isomorphism"},
		},
	}

	repo := repository.NewMemoryCatalogRepository()
	idx := searchindex.NewBM25Index()
	svc := NewCatalogService(repo, mockClient, idx, nil, nil, nil)

	ctx := context.Background()

	// 1. Init
	if err := svc.Init(ctx, true); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	status := svc.GetStatus(ctx)
	if !status.Ready || status.TotalEntries != 2 {
		t.Errorf("unexpected status: %+v", status)
	}

	// 2. GetEntry
	e, err := svc.GetEntry(ctx, "Formal_SSA")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if e.Title != "Verified Construction of SSA Form" {
		t.Errorf("unexpected title: %s", e.Title)
	}

	// Case-insensitive test
	e2, err := svc.GetEntry(ctx, "formal_ssa")
	if err != nil || e2.Shortname != "Formal_SSA" {
		t.Errorf("case-insensitive lookup failed: %v", err)
	}

	// 3. ListEntries
	entries, total, err := svc.ListEntries(ctx, 10, 0)
	if err != nil || total != 2 || len(entries) != 2 {
		t.Errorf("ListEntries failed: total=%d, len=%d, err=%v", total, len(entries), err)
	}

	// 4. GetEntriesByTopic
	byTopic, totalT, err := svc.GetEntriesByTopic(ctx, "Algebra", 10, 0)
	if err != nil || totalT != 1 || len(byTopic) != 1 || byTopic[0].Shortname != "Finite_Fields" {
		t.Errorf("GetEntriesByTopic failed: %+v", byTopic)
	}

	// 5. SuggestKeywords
	kws, err := svc.SuggestKeywords(ctx, "gr", 10)
	if err != nil {
		t.Fatalf("SuggestKeywords failed: %v", err)
	}
	if len(kws) != 2 { // "graph reachability" and "group isomorphism"
		t.Errorf("expected 2 keywords, got %d (%v)", len(kws), kws)
	}
}
