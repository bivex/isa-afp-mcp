package port

import (
	"context"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/entry"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/theory"
)

// SearchUseCase defines the contract for searching and filtering the AFP catalog.
type SearchUseCase interface {
	Search(ctx context.Context, q search.Query) (*search.Result, error)
}

// EntryUseCase defines operations on individual AFP entries.
type EntryUseCase interface {
	GetEntry(ctx context.Context, id string) (*entry.Entry, error)
	ListEntries(ctx context.Context, limit, offset int) ([]entry.Entry, int, error)
	GetDependencies(ctx context.Context, id string) (*theory.RootFile, error)
}

// TopicUseCase defines queries on the topic classification tree.
type TopicUseCase interface {
	ListTopics(ctx context.Context) ([]entry.Topic, error)
	GetEntriesByTopic(ctx context.Context, topicName string, limit, offset int) ([]entry.Entry, int, error)
}

// KeywordUseCase defines math/CS keyword discovery and autocomplete.
type KeywordUseCase interface {
	SuggestKeywords(ctx context.Context, prefix string, limit int) ([]string, error)
}

// TheoryUseCase defines inspection of Isabelle theories (.thy) and ROOT specifications.
type TheoryUseCase interface {
	ListTheories(ctx context.Context, entryID string) ([]theory.TheoryFileInfo, error)
	GetTheoryOutline(ctx context.Context, entryID, theoryName string) (*theory.TheoryOutline, error)
	GetTheoryContent(ctx context.Context, entryID, theoryPath string, startLine, maxLines int) (*theory.TheoryContent, error)
	GetRootFile(ctx context.Context, entryID string) (*theory.RootFile, error)
}

// CatalogSyncUseCase defines synchronization of upstream AFP catalog data.
type CatalogSyncUseCase interface {
	Sync(ctx context.Context) error
	GetStatus(ctx context.Context) CatalogStatus
}

// CatalogStatus reports catalog availability and freshness.
type CatalogStatus struct {
	TotalEntries  int    `json:"total_entries"`
	TotalTopics   int    `json:"total_topics"`
	TotalAuthors  int    `json:"total_authors"`
	TotalKeywords int    `json:"total_keywords"`
	LastSyncedAt  string `json:"last_synced_at,omitempty"`
	Source        string `json:"source"`
	Ready         bool   `json:"ready"`
}
