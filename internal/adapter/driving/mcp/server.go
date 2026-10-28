package mcp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ServerName    = "isa-afp-mcp"
	ServerVersion = "1.0.0"

	serverInstructions = `Archive of Formal Proofs (AFP) MCP Server.
Provides lightning-fast search and inspection across 1045+ verified Isabelle/HOL proof packages.
Best practices for agents:
1. Use 'search_entries' to find formal specifications of algorithms or concepts (e.g. Dijkstra, SSA, Red-Black Trees, Finite Fields, RSA).
2. 'search_entries' with default 'compact' format returns token-dense results with concise abstract snippets.
3. Use 'get_entry' for comprehensive details and release archive URLs.
4. Use 'get_root' to inspect session dependencies and exported theory lists.
5. Use 'get_theory_outline' to examine datatypes, definitions, lemmas, and theorems WITHOUT proof bodies (saving ~90% tokens).
6. Use 'get_theory_content' with 'start_line' and 'max_lines' only if you need the actual proof tactics or raw source.`
)

// Server encapsulates the MCP server instance and its domain dependencies.
type Server struct {
	mcpServer   *server.MCPServer
	toolManager *ToolManager
	entryUseCase port.EntryUseCase
	topicUseCase port.TopicUseCase
	syncUseCase  port.CatalogSyncUseCase
	logger      *slog.Logger
}

// NewServer builds an MCP server with all tools and resources wired.
func NewServer(
	searchUseCase port.SearchUseCase,
	entryUseCase port.EntryUseCase,
	topicUseCase port.TopicUseCase,
	keywordUseCase port.KeywordUseCase,
	theoryUseCase port.TheoryUseCase,
	syncUseCase port.CatalogSyncUseCase,
	logger *slog.Logger,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	mcpSrv := server.NewMCPServer(
		ServerName,
		ServerVersion,
		server.WithInstructions(serverInstructions),
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(false, false),
	)

	toolMgr := NewToolManager(
		searchUseCase,
		entryUseCase,
		topicUseCase,
		keywordUseCase,
		theoryUseCase,
		syncUseCase,
	)
	toolMgr.RegisterTools(mcpSrv)

	s := &Server{
		mcpServer:   mcpSrv,
		toolManager: toolMgr,
		entryUseCase: entryUseCase,
		topicUseCase: topicUseCase,
		syncUseCase:  syncUseCase,
		logger:      logger,
	}

	s.registerResources()
	return s
}

func (s *Server) registerResources() {
	// Resource: afp://catalog/status
	s.mcpServer.AddResource(mcp.NewResource(
		"afp://catalog/status",
		"AFP Catalog Status",
		mcp.WithResourceDescription("Current health and metrics of the AFP catalog index"),
		mcp.WithMIMEType("application/json"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		status := s.syncUseCase.GetStatus(ctx)
		bytes, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return nil, err
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "afp://catalog/status",
				MIMEType: "application/json",
				Text:     string(bytes),
			},
		}, nil
	})

	// Resource: afp://topics
	s.mcpServer.AddResource(mcp.NewResource(
		"afp://topics",
		"AFP Topic Hierarchy",
		mcp.WithResourceDescription("Complete list of AFP mathematical and computer science topic classifications"),
		mcp.WithMIMEType("text/plain"),
	), func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		topics, err := s.topicUseCase.ListTopics(ctx)
		if err != nil {
			return nil, err
		}
		var sb strings.Builder
		for _, t := range topics {
			sb.WriteString(t.Name)
			sb.WriteString("\n")
		}
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "afp://topics",
				MIMEType: "text/plain",
				Text:     sb.String(),
			},
		}, nil
	})
}

// ServeStdio starts the MCP server over standard I/O (stdin/stdout).
func (s *Server) ServeStdio() error {
	s.logger.Info("Starting isa-afp-mcp server on stdio...")
	return server.ServeStdio(s.mcpServer)
}

// ServeSSE starts the MCP server as an HTTP SSE server on the specified address.
func (s *Server) ServeSSE(addr string) error {
	s.logger.Info("Starting isa-afp-mcp SSE server", "addr", addr)
	sseServer := server.NewSSEServer(s.mcpServer)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: sseServer,
	}
	return httpServer.ListenAndServe()
}
