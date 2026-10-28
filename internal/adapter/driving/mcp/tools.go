package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolManager coordinates registering and executing MCP tools.
type ToolManager struct {
	searchUseCase port.SearchUseCase
	entryUseCase  port.EntryUseCase
	topicUseCase  port.TopicUseCase
	keywordUseCase port.KeywordUseCase
	theoryUseCase port.TheoryUseCase
	syncUseCase   port.CatalogSyncUseCase
}

// NewToolManager creates a ToolManager wiring domain use cases to MCP tool handlers.
func NewToolManager(
	searchUseCase port.SearchUseCase,
	entryUseCase port.EntryUseCase,
	topicUseCase port.TopicUseCase,
	keywordUseCase port.KeywordUseCase,
	theoryUseCase port.TheoryUseCase,
	syncUseCase port.CatalogSyncUseCase,
) *ToolManager {
	return &ToolManager{
		searchUseCase: searchUseCase,
		entryUseCase:  entryUseCase,
		topicUseCase:  topicUseCase,
		keywordUseCase: keywordUseCase,
		theoryUseCase: theoryUseCase,
		syncUseCase:   syncUseCase,
	}
}

// RegisterTools registers all AFP tools on the MCP server.
func (tm *ToolManager) RegisterTools(s *server.MCPServer) {
	// 1. search_entries
	s.AddTool(mcp.NewTool("search_entries",
		mcp.WithDescription("Search 1045+ formal mathematical and algorithmic proof entries in Archive of Formal Proofs (AFP). Optimized for speed and token-efficient summaries."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search text, algorithm or concept (e.g. 'Dijkstra', 'Red-black trees', 'SSA', 'finite fields')")),
		mcp.WithString("topic", mcp.Description("Optional topic filter (e.g. 'Computer science/Algorithms', 'Mathematics/Algebra')")),
		mcp.WithString("author", mcp.Description("Optional author filter substring")),
		mcp.WithInteger("year_from", mcp.Description("Filter entries released from this year onward (e.g. 2020)")),
		mcp.WithInteger("year_to", mcp.Description("Filter entries released up to this year")),
		mcp.WithInteger("limit", mcp.Description("Number of entries to return (default 10, max 50)")),
		mcp.WithInteger("offset", mcp.Description("Pagination offset (default 0)")),
		mcp.WithString("format", mcp.Description("Output style: 'compact' (default, token-dense for agents), 'detailed', or 'json'")),
	), tm.handleSearchEntries)

	// 2. get_entry
	s.AddTool(mcp.NewTool("get_entry",
		mcp.WithDescription("Get complete metadata, abstract, authors, release date, download links, and publications for a specific AFP entry."),
		mcp.WithString("shortname", mcp.Required(), mcp.Description("AFP entry shortname (e.g. 'Formal_SSA', 'Finite_Fields')")),
	), tm.handleGetEntry)

	// 3. list_theories
	s.AddTool(mcp.NewTool("list_theories",
		mcp.WithDescription("List all Isabelle theory (.thy) files and ROOT specification in an AFP entry package."),
		mcp.WithString("shortname", mcp.Required(), mcp.Description("AFP entry shortname (e.g. 'Formal_SSA')")),
	), tm.handleListTheories)

	// 4. get_theory_outline
	s.AddTool(mcp.NewTool("get_theory_outline",
		mcp.WithDescription("Extract token-efficient interface of an Isabelle theory (.thy) file: datatypes, definitions, lemmas, and theorems WITHOUT proof scripts."),
		mcp.WithString("shortname", mcp.Required(), mcp.Description("AFP entry shortname")),
		mcp.WithString("theory", mcp.Required(), mcp.Description("Theory name or file name (e.g. 'Construct_SSA' or 'Construct_SSA.thy')")),
	), tm.handleGetTheoryOutline)

	// 5. get_theory_content
	s.AddTool(mcp.NewTool("get_theory_content",
		mcp.WithDescription("Read the raw content of an Isabelle theory (.thy) or ROOT file, with line range pagination to protect context window."),
		mcp.WithString("shortname", mcp.Required(), mcp.Description("AFP entry shortname")),
		mcp.WithString("path", mcp.Required(), mcp.Description("File path inside entry archive (e.g. 'Construct_SSA.thy' or 'ROOT')")),
		mcp.WithInteger("start_line", mcp.Description("1-based start line (default 1)")),
		mcp.WithInteger("max_lines", mcp.Description("Max lines to return (default 200, max 1000)")),
	), tm.handleGetTheoryContent)

	// 6. get_root
	s.AddTool(mcp.NewTool("get_root",
		mcp.WithDescription("Inspect Isabelle ROOT session dependencies, parent sessions, and exported theories for an entry."),
		mcp.WithString("shortname", mcp.Required(), mcp.Description("AFP entry shortname")),
	), tm.handleGetRoot)

	// 7. list_topics
	s.AddTool(mcp.NewTool("list_topics",
		mcp.WithDescription("List hierarchical topic classifications in AFP (e.g. Computer science, Mathematics, Logic)."),
		mcp.WithString("filter", mcp.Description("Optional substring to filter topic names (e.g. 'algebra', 'algorithm')")),
	), tm.handleListTopics)

	// 8. suggest_keywords
	s.AddTool(mcp.NewTool("suggest_keywords",
		mcp.WithDescription("Autocomplete or discover mathematical and computer science terms from AFP's concept dictionary."),
		mcp.WithString("prefix", mcp.Required(), mcp.Description("Prefix or search concept (e.g. 'elliptic', 'graph', 'rsa')")),
		mcp.WithInteger("limit", mcp.Description("Max suggestions (default 15)")),
	), tm.handleSuggestKeywords)

	// 9. get_catalog_status
	s.AddTool(mcp.NewTool("get_catalog_status",
		mcp.WithDescription("Get catalog status: total indexed entries, topics, keywords, freshness, and cache status."),
	), tm.handleGetCatalogStatus)

	// 10. sync_catalog
	s.AddTool(mcp.NewTool("sync_catalog",
		mcp.WithDescription("Force re-synchronization of the AFP catalog from upstream isa-afp.org."),
	), tm.handleSyncCatalog)
}

func (tm *ToolManager) handleSearchEntries(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Query    string `json:"query"`
		Topic    string `json:"topic"`
		Author   string `json:"author"`
		YearFrom int    `json:"year_from"`
		YearTo   int    `json:"year_to"`
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
		Format   string `json:"format"`
	}

	bytes, err := json.Marshal(request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError("failed to decode arguments"), nil
	}
	_ = json.Unmarshal(bytes, &args)

	q := search.Query{
		Text:     args.Query,
		Topic:    args.Topic,
		Author:   args.Author,
		YearFrom: args.YearFrom,
		YearTo:   args.YearTo,
		Limit:    args.Limit,
		Offset:   args.Offset,
		Format:   args.Format,
	}

	res, err := tm.searchUseCase.Search(ctx, q)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	text, err := FormatSearchResult(res, q.Format)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("format result failed: %v", err)), nil
	}

	return mcp.NewToolResultText(text), nil
}

func (tm *ToolManager) handleGetEntry(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Shortname string `json:"shortname"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	if args.Shortname == "" {
		return mcp.NewToolResultError("shortname parameter is required"), nil
	}

	e, err := tm.entryUseCase.GetEntry(ctx, args.Shortname)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("entry not found: %s", args.Shortname)), nil
	}

	return mcp.NewToolResultText(FormatEntry(e)), nil
}

func (tm *ToolManager) handleListTheories(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Shortname string `json:"shortname"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	if args.Shortname == "" {
		return mcp.NewToolResultError("shortname parameter is required"), nil
	}

	files, err := tm.theoryUseCase.ListTheories(ctx, args.Shortname)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list theories failed: %v", err)), nil
	}

	return mcp.NewToolResultText(FormatTheoriesList(args.Shortname, files)), nil
}

func (tm *ToolManager) handleGetTheoryOutline(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Shortname string `json:"shortname"`
		Theory    string `json:"theory"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	if args.Shortname == "" || args.Theory == "" {
		return mcp.NewToolResultError("both 'shortname' and 'theory' parameters are required"), nil
	}

	outline, err := tm.theoryUseCase.GetTheoryOutline(ctx, args.Shortname, args.Theory)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get theory outline failed: %v", err)), nil
	}

	return mcp.NewToolResultText(FormatTheoryOutline(outline)), nil
}

func (tm *ToolManager) handleGetTheoryContent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Shortname string `json:"shortname"`
		Path      string `json:"path"`
		StartLine int    `json:"start_line"`
		MaxLines  int    `json:"max_lines"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	if args.Shortname == "" || args.Path == "" {
		return mcp.NewToolResultError("both 'shortname' and 'path' parameters are required"), nil
	}

	content, err := tm.theoryUseCase.GetTheoryContent(ctx, args.Shortname, args.Path, args.StartLine, args.MaxLines)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get theory content failed: %v", err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s in Entry `%s` (Lines %d–%d of %d)\n\n```isabelle\n",
		content.Path, content.EntryShortname, content.LineStart, content.LineEnd, content.TotalLines))
	sb.WriteString(content.Content)
	sb.WriteString("\n```\n")
	if content.Truncated {
		sb.WriteString(fmt.Sprintf("\n*(Truncated at line %d. Use start_line=%d to read more)*\n",
			content.LineEnd, content.LineEnd+1))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func (tm *ToolManager) handleGetRoot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Shortname string `json:"shortname"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	if args.Shortname == "" {
		return mcp.NewToolResultError("shortname parameter is required"), nil
	}

	root, err := tm.theoryUseCase.GetRootFile(ctx, args.Shortname)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get ROOT failed: %v", err)), nil
	}

	return mcp.NewToolResultText(FormatRoot(root)), nil
}

func (tm *ToolManager) handleListTopics(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Filter string `json:"filter"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	topics, err := tm.topicUseCase.ListTopics(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list topics failed: %v", err)), nil
	}

	filter := strings.ToLower(strings.TrimSpace(args.Filter))
	var sb strings.Builder
	sb.WriteString("# AFP Topic Classifications\n\n")

	count := 0
	for _, t := range topics {
		if filter != "" && !strings.Contains(strings.ToLower(t.Name), filter) {
			continue
		}
		sb.WriteString(fmt.Sprintf("- %s\n", t.Name))
		count++
	}

	if count == 0 {
		return mcp.NewToolResultText("No topics matched filter: " + args.Filter), nil
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func (tm *ToolManager) handleSuggestKeywords(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
	}
	bytes, _ := json.Marshal(request.Params.Arguments)
	_ = json.Unmarshal(bytes, &args)

	if args.Prefix == "" {
		return mcp.NewToolResultError("prefix parameter is required"), nil
	}

	kws, err := tm.keywordUseCase.SuggestKeywords(ctx, args.Prefix, args.Limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("suggest keywords failed: %v", err)), nil
	}

	if len(kws) == 0 {
		return mcp.NewToolResultText("No keywords found matching: " + args.Prefix), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Matched %d keywords:\n\n", len(kws)))
	for _, kw := range kws {
		sb.WriteString(fmt.Sprintf("- %s\n", kw))
	}

	return mcp.NewToolResultText(sb.String()), nil
}

func (tm *ToolManager) handleGetCatalogStatus(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := tm.syncUseCase.GetStatus(ctx)
	bytes, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("format status failed: %v", err)), nil
	}
	return mcp.NewToolResultText(string(bytes)), nil
}

func (tm *ToolManager) handleSyncCatalog(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := tm.syncUseCase.Sync(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("catalog sync failed: %v", err)), nil
	}
	status := tm.syncUseCase.GetStatus(ctx)
	return mcp.NewToolResultText(fmt.Sprintf("Catalog synced successfully. Total entries: %d, Source: %s",
		status.TotalEntries, status.Source)), nil
}
