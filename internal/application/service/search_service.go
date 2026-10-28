package service

import (
	"context"
	"fmt"

	"github.com/isa-afp/isa-afp-mcp/internal/domain/port"
	"github.com/isa-afp/isa-afp-mcp/internal/domain/search"
)

// SearchService implements port.SearchUseCase.
type SearchService struct {
	index port.SearchIndex
}

var _ port.SearchUseCase = (*SearchService)(nil)

// NewSearchService creates a new SearchService.
func NewSearchService(index port.SearchIndex) *SearchService {
	return &SearchService{
		index: index,
	}
}

// Search executes an in-memory BM25 query with filters and returns ranked results.
func (s *SearchService) Search(ctx context.Context, q search.Query) (*search.Result, error) {
	if s.index == nil {
		return nil, fmt.Errorf("search index is not initialized")
	}
	return s.index.Search(q)
}
