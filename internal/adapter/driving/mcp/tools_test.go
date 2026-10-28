package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/repository"
	"github.com/isa-afp/isa-afp-mcp/internal/adapter/driven/searchindex"
	"github.com/isa-afp/isa-afp-mcp/internal/application/service"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type mockTheoryUseCase struct{}

func (m *mockTheoryUseCase) ListTheories(ctx context.Context, entryID string) ([]theory.TheoryFileInfo, error) {
	return []theory.TheoryFileInfo{
		{EntryShortname: entryID, Path: "Formal_SSA/ROOT", Name: "ROOT", SizeBytes: 128, IsRoot: true},
		{EntryShortname: entryID, Path: "Formal_SSA/Construct_SSA.thy", Name: "Construct_SSA", SizeBytes: 1024},
	}, nil
}
func (m *mockTheoryUseCase) GetTheoryOutline(ctx context.Context, entryID, theoryName string) (*theory.TheoryOutline, error) {
	return &theory.TheoryOutline{
		EntryShortname: entryID,
		TheoryName:     theoryName,
		Imports:        []string{"Main"},
		TotalTheorems:  5,
		TotalLemmas:    10,
	}, nil
}
func (m *mockTheoryUseCase) GetTheoryContent(ctx context.Context, entryID, theoryPath string, startLine, maxLines int) (*theory.TheoryContent, error) {
	return &theory.TheoryContent{
		EntryShortname: entryID,
		Path:           theoryPath,
		LineStart:      1,
		LineEnd:        5,
		TotalLines:     5,
		Content:        "theory Foo imports Main begin\nend",
	}, nil
}
func (m *mockTheoryUseCase) GetRootFile(ctx context.Context, entryID string) (*theory.RootFile, error) {
	return &theory.RootFile{
		EntryShortname: entryID,
		Chapter:        "AFP",
		Sessions: []theory.Session{
			{Name: entryID, Parent: "HOL"},
		},
	}, nil
}

func TestMCPToolHandlers(t *testing.T) {
	entries := []entry.Entry{
		{
			Shortname: "Formal_SSA",
			Title:     "Verified Construction of SSA Form",
			Authors:   []string{"Denis Lohner", "Sebastian Ullrich"},
			Date:      "2016-02-05",
			Year:      "2016",
			Topics:    []string{"Computer science/Semantics"},
			Abstract:  "We formalize SSA construction.",
			Permalink: "https://isa-afp.org/entries/Formal_SSA.html",
		},
	}
	topics := []entry.Topic{{Name: "Computer science/Semantics"}}
	keywords := []entry.Keyword{{Keyword: "static single assignment"}}

	repo := repository.NewMemoryCatalogRepository()
	_ = repo.SaveAll(entries, topics, nil, keywords)

	idx := searchindex.NewBM25Index()
	_ = idx.Build(entries)

	mockTheory := &mockTheoryUseCase{}
	searchSvc := service.NewSearchService(idx)
	catSvc := service.NewCatalogService(repo, nil, idx, nil, mockTheory, nil)

	tm := NewToolManager(searchSvc, catSvc, catSvc, catSvc, mockTheory, catSvc)
	s := server.NewMCPServer("test", "1.0.0")
	tm.RegisterTools(s)

	ctx := context.Background()

	// 1. Test search_entries handler
	searchReq := mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name: "search_entries",
			Arguments: map[string]any{
				"query": "SSA form",
			},
		},
	}
	res, err := tm.handleSearchEntries(ctx, searchReq)
	if err != nil {
		t.Fatalf("handleSearchEntries returned error: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected successful search result, got error")
	}

	// 2. Test get_entry handler
	entryReq := mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name: "get_entry",
			Arguments: map[string]any{
				"shortname": "Formal_SSA",
			},
		},
	}
	entryRes, err := tm.handleGetEntry(ctx, entryReq)
	if err != nil || entryRes.IsError {
		t.Fatalf("handleGetEntry failed: %v", err)
	}

	// 3. Test get_theory_outline handler
	outlineReq := mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name: "get_theory_outline",
			Arguments: map[string]any{
				"shortname": "Formal_SSA",
				"theory":    "Construct_SSA",
			},
		},
	}
	outlineRes, err := tm.handleGetTheoryOutline(ctx, outlineReq)
	if err != nil || outlineRes.IsError {
		t.Fatalf("handleGetTheoryOutline failed: %v", err)
	}

	// 4. Test get_root handler
	rootReq := mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name: "get_root",
			Arguments: map[string]any{
				"shortname": "Formal_SSA",
			},
		},
	}
	rootRes, err := tm.handleGetRoot(ctx, rootReq)
	if err != nil || rootRes.IsError {
		t.Fatalf("handleGetRoot failed: %v", err)
	}

	// 5. Test get_catalog_status
	statusReq := mcpsdk.CallToolRequest{}
	statusRes, err := tm.handleGetCatalogStatus(ctx, statusReq)
	if err != nil || statusRes.IsError {
		t.Fatalf("handleGetCatalogStatus failed: %v", err)
	}
	var rawText string
	if len(statusRes.Content) > 0 {
		if textContent, ok := statusRes.Content[0].(mcpsdk.TextContent); ok {
			rawText = textContent.Text
		}
	}
	var statusMap map[string]any
	if err := json.Unmarshal([]byte(rawText), &statusMap); err != nil {
		t.Fatalf("status output is not valid json: %v (%s)", err, rawText)
	}
	if statusMap["total_entries"].(float64) != 1 {
		t.Errorf("expected total_entries=1, got %v", statusMap["total_entries"])
	}
}
